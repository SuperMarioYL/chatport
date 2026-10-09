// Package ingest turns vendor export archives into Context Packs.
//
// chatgpt.go implements the ChatGPT official export (conversations.json
// inside the data-export zip). It is the only wired parser in v0.1 (m1):
// the CN adapter lands in v0.3 behind the Gate-0 audit (see cn_export.go).
package ingest

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SuperMarioYL/chatport/internal/ir"
)

// ErrUnknownFormat means the archive matched no known export layout.
var ErrUnknownFormat = errors.New("unrecognized export archive")

// Parser is the single seam every export adapter implements. Adapters are
// cheap: one file, one Parse function, one shared IR.
type Parser interface {
	SourceApp() string
	Parse(zr *zip.Reader) (*ir.Pack, error)
}

// Detect sniffs the archive layout and returns the matching parser.
// It reads entry names only; the returned parser streams the payload.
func Detect(zr *zip.Reader) (Parser, error) {
	names := make([]string, 0, len(zr.File))
	hasChatGPT := false
	for _, f := range zr.File {
		names = append(names, f.Name)
		if strings.HasSuffix(f.Name, "conversations.json") {
			hasChatGPT = true
		}
	}
	if hasChatGPT {
		return ChatGPT{}, nil
	}
	// v0.1 only knows ChatGPT exports. detectCN is the reserved dispatch
	// seam for the Gate-0-selected pair; with no adapter wired it always
	// reports not-matched, so anything else is an unknown archive.
	if err := detectCN(names); err != nil && !errors.Is(err, ErrCNNotMatched) {
		return nil, err
	}
	return nil, fmt.Errorf("%w: no conversations.json entry (supported: ChatGPT data export; %v)",
		ErrUnknownFormat, ErrCNAdapterNotWired)
}

// ChatGPT parses the ChatGPT official export zip.
type ChatGPT struct{}

// SourceApp implements Parser.
func (ChatGPT) SourceApp() string { return "chatgpt" }

// Parse implements Parser. It walks each conversation's mapping tree from
// the current_node backwards, which is the branch the user actually sees,
// then reverses it into chronological order.
func (ChatGPT) Parse(zr *zip.Reader) (*ir.Pack, error) {
	entry := findEntry(zr, "conversations.json")
	if entry == nil {
		return nil, fmt.Errorf("ingest chatgpt: %w", ErrUnknownFormat)
	}
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("ingest chatgpt: open conversations.json: %w", err)
	}
	defer rc.Close()

	pack := &ir.Pack{Meta: ir.Meta{SourceApp: "chatgpt"}}
	dec := json.NewDecoder(rc)
	if tok, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("ingest chatgpt: %w", err)
	} else if d, ok := tok.(json.Delim); !ok || d != '[' {
		return nil, fmt.Errorf("ingest chatgpt: conversations.json is not a JSON array")
	}
	index := 0
	for dec.More() {
		var dto conversationDTO
		if err := dec.Decode(&dto); err != nil {
			return nil, fmt.Errorf("ingest chatgpt: conversation #%d: %w", index, err)
		}
		pack.Threads = append(pack.Threads, dto.conversation(index))
		index++
	}
	if _, err := dec.Token(); err != nil { // closing ']'
		return nil, fmt.Errorf("ingest chatgpt: %w", err)
	}
	if len(pack.Threads) == 0 {
		return nil, fmt.Errorf("ingest chatgpt: export contains no conversations")
	}
	pack.Finalize(ir.DefaultSizeBudgetPerFile)
	return pack, nil
}

func findEntry(zr *zip.Reader, base string) *zip.File {
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, base) {
			return f
		}
	}
	return nil
}

type conversationDTO struct {
	ID             string             `json:"id"`
	ConversationID string             `json:"conversation_id"`
	Title          string             `json:"title"`
	CreateTime     *float64           `json:"create_time"`
	UpdateTime     *float64           `json:"update_time"`
	CurrentNode    string             `json:"current_node"`
	Mapping        map[string]nodeDTO `json:"mapping"`
}

type nodeDTO struct {
	ID       string      `json:"id"`
	Message  *messageDTO `json:"message"`
	Parent   *string     `json:"parent"`
	Children []string    `json:"children"`
}

type messageDTO struct {
	Author struct {
		Role string `json:"role"`
	} `json:"author"`
	CreateTime *float64 `json:"create_time"`
	Content    struct {
		ContentType string            `json:"content_type"`
		Parts       []json.RawMessage `json:"parts"`
	} `json:"content"`
	Metadata struct {
		IsVisuallyHidden bool `json:"is_visually_hidden_from_conversation"`
	} `json:"metadata"`
	Status string `json:"status"`
}

func (c *conversationDTO) conversation(index int) ir.Conversation {
	conv := ir.Conversation{
		ID:        c.ConversationID,
		Title:     strings.TrimSpace(c.Title),
		CreatedAt: unix(c.CreateTime),
		UpdatedAt: unix(c.UpdateTime),
	}
	if conv.ID == "" {
		conv.ID = c.ID
	}
	if conv.ID == "" {
		conv.ID = fmt.Sprintf("conv-%04d", index)
	}
	conv.Messages = c.linearize()
	return conv
}

// linearize walks the mapping tree from current_node back to the root and
// returns the visible messages in chronological order. Hidden nodes
// (is_visually_hidden_from_conversation) and empty parts are skipped.
func (c *conversationDTO) linearize() []ir.Message {
	if len(c.Mapping) == 0 {
		return nil
	}
	nodeID := c.CurrentNode
	if nodeID == "" {
		nodeID = anyLeaf(c.Mapping)
	}
	var chain []nodeDTO
	seen := map[string]bool{}
	for id := nodeID; id != ""; {
		if seen[id] { // defensive: never loop on malformed parents
			break
		}
		seen[id] = true
		node, ok := c.Mapping[id]
		if !ok {
			break
		}
		chain = append(chain, node)
		if node.Parent == nil {
			break
		}
		id = *node.Parent
	}
	msgs := make([]ir.Message, 0, len(chain))
	for i := len(chain) - 1; i >= 0; i-- { // reversed walk = chronological
		m := chain[i].Message
		if m == nil || m.Metadata.IsVisuallyHidden {
			continue
		}
		text := messageText(m)
		if text == "" {
			continue
		}
		role := ir.Role(m.Author.Role)
		if role == "" {
			role = ir.RoleAssistant
		}
		msgs = append(msgs, ir.Message{
			Role: role,
			At:   unix(m.CreateTime),
			Text: text,
		})
	}
	return msgs
}

// messageText joins the string parts of a content payload. Non-string parts
// (multimedia payloads) and unknown content types are ignored, not guessed.
func messageText(m *messageDTO) string {
	if m.Content.ContentType != "text" && m.Content.ContentType != "code" {
		return ""
	}
	var b strings.Builder
	for _, raw := range m.Content.Parts {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s)
	}
	return strings.TrimSpace(b.String())
}

func anyLeaf(m map[string]nodeDTO) string {
	for _, node := range m {
		if len(node.Children) == 0 {
			return node.ID
		}
	}
	return ""
}

func unix(p *float64) time.Time {
	if p == nil || *p <= 0 {
		return time.Time{}
	}
	sec := int64(*p)
	return time.Unix(sec, int64((*p-float64(sec))*1e9)).UTC()
}
