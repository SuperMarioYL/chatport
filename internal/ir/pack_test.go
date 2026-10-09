package ir

import (
	"testing"
	"time"
)

func TestInferProjectHint(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"【项目】博客重构", "项目"},
		{"[SaaS] pricing page", "SaaS"},
		{"Project: Outbound Pricing", "Outbound Pricing"},
		{"项目：出海定价", "出海定价"},
		{"项目:出海定价", "出海定价"},
		{"just a chat about go", ""},
		{"", ""},
		{"【unclosed", ""},
	}
	for _, c := range cases {
		if got := InferProjectHint(c.title); got != c.want {
			t.Errorf("InferProjectHint(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

func TestFingerprintNormalizesWhitespace(t *testing.T) {
	a := Fingerprint("hello   world\n\tgo")
	b := Fingerprint("hello world go")
	if a != b {
		t.Fatalf("whitespace normalization failed: %q != %q", a, b)
	}
	if a == Fingerprint("hello world") {
		t.Fatal("distinct texts must not share a fingerprint")
	}
}

func testConv(id, title string, msgs ...string) Conversation {
	c := Conversation{ID: id, Title: title, CreatedAt: time.Unix(1700000000, 0).UTC()}
	for i, m := range msgs {
		c.Messages = append(c.Messages, Message{
			Role: RoleUser,
			At:   time.Unix(int64(1700000000+i*60), 0).UTC(),
			Text: m,
		})
	}
	return c
}

func TestFinalizeCountsGroupsAndDedup(t *testing.T) {
	p := &Pack{
		Meta: Meta{SourceApp: "chatgpt"},
		Threads: []Conversation{
			testConv("c1", "【博客】重构计划", "设计信息架构", "设计信息架构", "写第一篇"),
			testConv("c2", "[博客] 选型", "选 Hugo 还是 Astro", "设计信息架构"),
			testConv("c3", "随便聊聊", "你好"),
		},
	}
	p.Finalize(0) // 0 falls back to DefaultSizeBudgetPerFile

	if got := p.Meta.NConversations; got != 3 {
		t.Errorf("NConversations = %d, want 3", got)
	}
	// c1 has 3 messages, c2 has 2, c3 has 1; two extra copies of
	// "设计信息架构" dedupe away.
	if got := p.Meta.NMessages; got != 6 {
		t.Errorf("NMessages = %d, want 6", got)
	}
	if got := p.Digest.DupMessages; got != 2 {
		t.Errorf("DupMessages = %d, want 2", got)
	}
	if got := p.Digest.SizeBudgetPerFile; got != DefaultSizeBudgetPerFile {
		t.Errorf("SizeBudgetPerFile = %d, want default %d", got, DefaultSizeBudgetPerFile)
	}
	groups := p.Digest.PerProjectGroups
	if len(groups["博客"]) != 2 {
		t.Errorf("博客 group = %v, want 2 threads (c1, c2)", groups["博客"])
	}
	if len(groups[""]) != 1 {
		t.Errorf("ungrouped = %v, want 1 thread (c3)", groups[""])
	}
	if p.Threads[0].ProjectHint != "博客" || p.Threads[2].ProjectHint != "" {
		t.Errorf("hints not applied in place: %+v %+v", p.Threads[0], p.Threads[2])
	}
}

func TestStatsAcrossProjects(t *testing.T) {
	p := &Pack{
		Meta: Meta{SourceApp: "chatgpt"},
		Threads: []Conversation{
			{ID: "a", Title: "Project: alpha", CreatedAt: time.Unix(1700000100, 0).UTC(),
				Messages: []Message{{Role: RoleUser, At: time.Unix(1700000100, 0).UTC(), Text: "hi"}}},
			{ID: "b", Title: "no group", CreatedAt: time.Unix(1700000200, 0).UTC(),
				Messages: []Message{{Role: RoleAssistant, At: time.Unix(1700000200, 0).UTC(), Text: "hello"}}},
		},
	}
	p.Finalize(0)
	s := p.Stats()
	if s.Conversations != 2 || s.Messages != 2 || s.Duplicates != 0 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if s.Projects["alpha"] != 1 || s.Projects[""] != 1 {
		t.Errorf("project counts wrong: %v", s.Projects)
	}
	if s.FirstAt.IsZero() || !s.FirstAt.Equal(time.Unix(1700000100, 0).UTC()) {
		t.Errorf("FirstAt = %v", s.FirstAt)
	}
	if len(s.PerConv) != 2 || s.PerConv[0].Messages != 1 {
		t.Errorf("PerConv wrong: %+v", s.PerConv)
	}
}

func TestFinalizeExportedAtFollowsLatestUpdate(t *testing.T) {
	at := time.Unix(1759000000, 0).UTC()
	p := &Pack{
		Meta:    Meta{SourceApp: "chatgpt"},
		Threads: []Conversation{{ID: "x", Title: "t", UpdatedAt: at, Messages: []Message{{Role: RoleUser, Text: "m"}}}},
	}
	p.Finalize(0)
	if !p.Meta.ExportedAt.Equal(at) {
		t.Errorf("ExportedAt = %v, want %v", p.Meta.ExportedAt, at)
	}
}
