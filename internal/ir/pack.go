// Package ir defines the Context Pack: an app-agnostic intermediate
// representation (IR) for conversation history. Every ingest parser writes
// into a Pack; every target renderer reads from one. v0.1 is fully
// deterministic: no model calls, no network.
package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DefaultSizeBudgetPerFile is the default per-file size budget used when
// sharding a rendered bundle. It is a rendering knob, not a vendor claim.
const DefaultSizeBudgetPerFile = 100 << 10 // 100 KiB

// Role is who said a message. Values follow the source exports
// (user / assistant / system / tool) so ingest stays lossless.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message is a single turn in a conversation.
type Message struct {
	Role Role      `json:"role"`
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// Conversation is one thread of history. ProjectHint is inferred from the
// title by deterministic rules (see InferProjectHint); empty means ungrouped.
type Conversation struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	ProjectHint string    `json:"project_hint,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Messages    []Message `json:"messages"`
}

// Meta describes where a Pack came from.
type Meta struct {
	SourceApp      string    `json:"source_app"`  // "chatgpt" for now; CN adapters land in v0.3
	ExportedAt     time.Time `json:"exported_at"` // latest update_time across threads (proxy for export time)
	NConversations int       `json:"n_conversations"`
	NMessages      int       `json:"n_messages"`
}

// Digest carries the deterministic compaction knowledge a renderer needs:
// which messages are exact duplicates, how threads group by project, and the
// per-file size budget. v0.1 computes it; applying it happens at render time.
type Digest struct {
	// DedupIndex maps a normalized-text fingerprint to how many messages
	// share it. The first occurrence is canonical; the rest dedupe away.
	DedupIndex map[string]int `json:"dedup_index"`
	// DupMessages counts messages dropped by dedup (sum of counts minus keys).
	DupMessages int `json:"dup_messages"`
	// SizeBudgetPerFile is the per-file byte budget for sharded output.
	SizeBudgetPerFile int `json:"size_budget_per_file"`
	// PerProjectGroups maps project hint -> thread IDs. The empty key is the
	// ungrouped bucket.
	PerProjectGroups map[string][]string `json:"per_project_groups"`
}

// Pack is the Context Pack itself: the single type every adapter speaks.
type Pack struct {
	Meta    Meta           `json:"meta"`
	Threads []Conversation `json:"threads"`
	Digest  Digest         `json:"digest"`
}

// ConvStat is one row of the per-conversation breakdown printed by
// `chatport parse --stats`.
type ConvStat struct {
	Title       string
	Messages    int
	Duplicates  int
	FirstAt     time.Time
	LastAt      time.Time
	ProjectHint string
}

// Stats summarizes a Pack for the terminal.
type Stats struct {
	SourceApp     string
	Conversations int
	Messages      int
	Duplicates    int
	FirstAt       time.Time
	LastAt        time.Time
	Projects      map[string]int // project hint -> conversation count ("" = ungrouped)
	PerConv       []ConvStat
}

// InferProjectHint applies the deterministic title rules used in v0.1:
// a leading 【…】, […], or Project:/项目： prefix names the project.
// Everything else is ungrouped. No model calls, no guessing beyond prefixes.
func InferProjectHint(title string) string {
	t := strings.TrimSpace(title)
	for _, pair := range []struct{ open, close string }{
		{"【", "】"},
		{"[", "]"},
	} {
		if strings.HasPrefix(t, pair.open) {
			if end := strings.Index(t, pair.close); end > len(pair.open) {
				return strings.TrimSpace(t[len(pair.open):end])
			}
		}
	}
	for _, prefix := range []string{"Project:", "project:", "项目：", "项目:"} {
		if strings.HasPrefix(t, prefix) {
			if rest := strings.TrimSpace(t[len(prefix):]); rest != "" {
				return rest
			}
		}
	}
	return ""
}

// NormalizeText collapses whitespace so that messages differing only in
// spacing fingerprint identically. This is the entire dedup rule in v0.1:
// exact (normalized) equality, nothing smarter.
func NormalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Fingerprint is the dedup key for a message text.
func Fingerprint(text string) string {
	sum := sha256.Sum256([]byte(NormalizeText(text)))
	return hex.EncodeToString(sum[:8])
}

// Finalize recomputes every derived field: meta counts, exported_at,
// project hints, and the digest. Ingest calls it once after parsing;
// callers may call it again if they mutate threads.
func (p *Pack) Finalize(sizeBudget int) {
	if sizeBudget <= 0 {
		sizeBudget = DefaultSizeBudgetPerFile
	}
	p.Meta.NConversations = len(p.Threads)
	p.Meta.NMessages = 0
	p.Digest = Digest{
		DedupIndex:        map[string]int{},
		SizeBudgetPerFile: sizeBudget,
		PerProjectGroups:  map[string][]string{},
	}
	if p.Meta.SourceApp == "" {
		p.Meta.SourceApp = "unknown"
	}
	for i := range p.Threads {
		conv := &p.Threads[i]
		if conv.ProjectHint == "" {
			conv.ProjectHint = InferProjectHint(conv.Title)
		}
		p.Digest.PerProjectGroups[conv.ProjectHint] = append(
			p.Digest.PerProjectGroups[conv.ProjectHint], conv.ID)
		for j := range conv.Messages {
			m := &conv.Messages[j]
			p.Meta.NMessages++
			if m.Text == "" {
				continue
			}
			key := Fingerprint(m.Text)
			p.Digest.DedupIndex[key]++
			if !m.At.IsZero() && m.At.After(p.Meta.ExportedAt) {
				p.Meta.ExportedAt = m.At
			}
		}
		if conv.UpdatedAt.After(p.Meta.ExportedAt) {
			p.Meta.ExportedAt = conv.UpdatedAt
		}
	}
	for _, n := range p.Digest.DedupIndex {
		if n > 1 {
			p.Digest.DupMessages += n - 1
		}
	}
}

// Stats computes the terminal summary. It never mutates the pack.
func (p *Pack) Stats() Stats {
	s := Stats{
		SourceApp:     p.Meta.SourceApp,
		Conversations: p.Meta.NConversations,
		Messages:      p.Meta.NMessages,
		Duplicates:    p.Digest.DupMessages,
		Projects:      map[string]int{},
	}
	convByID := make(map[string]*Conversation, len(p.Threads))
	for i := range p.Threads {
		c := &p.Threads[i]
		convByID[c.ID] = c
		s.Projects[c.ProjectHint]++
		if !c.CreatedAt.IsZero() && (s.FirstAt.IsZero() || c.CreatedAt.Before(s.FirstAt)) {
			s.FirstAt = c.CreatedAt
		}
		if c.UpdatedAt.After(s.LastAt) {
			s.LastAt = c.UpdatedAt
		}
		dups := 0
		for _, m := range c.Messages {
			if m.Text != "" && p.Digest.DedupIndex[Fingerprint(m.Text)] > 1 {
				dups++
			}
		}
		s.PerConv = append(s.PerConv, ConvStat{
			Title:       c.Title,
			Messages:    len(c.Messages),
			Duplicates:  dups,
			FirstAt:     firstMessageAt(c),
			LastAt:      c.UpdatedAt,
			ProjectHint: c.ProjectHint,
		})
	}
	sort.Slice(s.PerConv, func(i, j int) bool {
		return s.PerConv[i].FirstAt.Before(s.PerConv[j].FirstAt)
	})
	return s
}

func firstMessageAt(c *Conversation) time.Time {
	for _, m := range c.Messages {
		if !m.At.IsZero() {
			return m.At
		}
	}
	return c.CreatedAt
}

// ProjectLabel renders the ungrouped bucket in zh-facing output.
func ProjectLabel(hint string) string {
	if hint == "" {
		return "未分组"
	}
	return hint
}

// SummaryLine is a compact one-line description used in logs and errors.
func (p *Pack) SummaryLine() string {
	return fmt.Sprintf("%s: %d conversations, %d messages",
		p.Meta.SourceApp, p.Meta.NConversations, p.Meta.NMessages)
}
