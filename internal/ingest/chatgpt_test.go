package ingest

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// goldenChatGPTExport is a minimal but structurally faithful ChatGPT data
// export: two conversations, one with an edit branch (current_node picks
// the live branch), one with a hidden system node and a duplicate message
// that digest dedup must count.
const goldenChatGPTExport = `[
  {
    "id": "conv-aaa",
    "conversation_id": "conv-aaa",
    "title": "【博客】重构计划",
    "create_time": 1720000000.0,
    "update_time": 1720001000.0,
    "current_node": "n4",
    "mapping": {
      "root": {"id": "root", "message": null, "parent": null, "children": ["n1"]},
      "n1": {"id": "n1", "message": null, "parent": "root", "children": ["n2"]},
      "n2": {
        "id": "n2",
        "message": {
          "id": "m1", "author": {"role": "user"}, "create_time": 1720000010.0,
          "content": {"content_type": "text", "parts": ["帮我规划博客重构"]},
          "metadata": {}, "status": "finished_successfully"
        },
        "parent": "n1", "children": ["n3a", "n3b"]
      },
      "n3a": {
        "id": "n3a",
        "message": {
          "id": "m2-old", "author": {"role": "assistant"}, "create_time": 1720000020.0,
          "content": {"content_type": "text", "parts": ["旧草稿回复（已被编辑）"]},
          "metadata": {}, "status": "finished_successfully"
        },
        "parent": "n2", "children": []
      },
      "n3b": {
        "id": "n3b",
        "message": {
          "id": "m2", "author": {"role": "assistant"}, "create_time": 1720000025.0,
          "content": {"content_type": "text", "parts": ["第一步：定信息架构", "第二步：迁移旧文"]},
          "metadata": {}, "status": "finished_successfully"
        },
        "parent": "n2", "children": ["n4"]
      },
      "n4": {
        "id": "n4",
        "message": {
          "id": "m3", "author": {"role": "user"}, "create_time": 1720000030.0,
          "content": {"content_type": "text", "parts": ["第一步展开讲讲"]},
          "metadata": {}, "status": "finished_successfully"
        },
        "parent": "n3b", "children": []
      }
    }
  },
  {
    "id": "conv-bbb",
    "conversation_id": "conv-bbb",
    "title": "help me with go",
    "create_time": 1720010000.0,
    "update_time": 1720010500.0,
    "current_node": "k4",
    "mapping": {
      "k1": {
        "id": "k1",
        "message": {
          "id": "s1", "author": {"role": "system"}, "create_time": 1720010000.0,
          "content": {"content_type": "text", "parts": ["You are ChatGPT"]},
          "metadata": {"is_visually_hidden_from_conversation": true}, "status": "finished_successfully"
        },
        "parent": null, "children": ["k2"]
      },
      "k2": {
        "id": "k2",
        "message": {
          "id": "s2", "author": {"role": "user"}, "create_time": 1720010010.0,
          "content": {"content_type": "text", "parts": ["帮我规划博客重构"]},
          "metadata": {}, "status": "finished_successfully"
        },
        "parent": "k1", "children": ["k3"]
      },
      "k3": {
        "id": "k3",
        "message": {
          "id": "s3", "author": {"role": "assistant"}, "create_time": 1720010020.0,
          "content": {"content_type": "code", "parts": ["fmt.Println(\"hi\")"]},
          "metadata": {}, "status": "finished_successfully"
        },
        "parent": "k2", "children": ["k4"]
      },
      "k4": {
        "id": "k4",
        "message": {
          "id": "s4", "author": {"role": "tool"}, "create_time": 1720010021.0,
          "content": {"content_type": "text", "parts": ["   hi\n "]},
          "metadata": {}, "status": "finished_successfully"
        },
        "parent": "k3", "children": []
      }
    }
  }
]`

func zipFrom(t *testing.T, name, content string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zr
}

func TestDetectRoutesChatGPTExport(t *testing.T) {
	zr := zipFrom(t, "conversations.json", goldenChatGPTExport)
	p, err := Detect(zr)
	if err != nil {
		t.Fatal(err)
	}
	if p.SourceApp() != "chatgpt" {
		t.Errorf("SourceApp = %q, want chatgpt", p.SourceApp())
	}
}

func TestDetectRejectsUnknownArchive(t *testing.T) {
	zr := zipFrom(t, "random.json", "{}")
	if _, err := Detect(zr); !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("want ErrUnknownFormat, got %v", err)
	}
}

func TestChatGPTParseGolden(t *testing.T) {
	zr := zipFrom(t, "conversations.json", goldenChatGPTExport)
	p, err := ChatGPT{}.Parse(zr)
	if err != nil {
		t.Fatal(err)
	}

	if p.Meta.SourceApp != "chatgpt" || p.Meta.NConversations != 2 {
		t.Fatalf("meta wrong: %+v", p.Meta)
	}

	// Conversation 1: current_node branch must win over the stale sibling.
	c1 := p.Threads[0]
	if c1.ID != "conv-aaa" || c1.Title != "【博客】重构计划" {
		t.Fatalf("c1 header wrong: %+v", c1)
	}
	if c1.ProjectHint != "博客" {
		t.Errorf("c1 hint = %q, want 博客", c1.ProjectHint)
	}
	var gotTexts []string
	for _, m := range c1.Messages {
		gotTexts = append(gotTexts, string(m.Role)+": "+m.Text)
	}
	want := []string{
		"user: 帮我规划博客重构",
		"assistant: 第一步：定信息架构\n第二步：迁移旧文",
		"user: 第一步展开讲讲",
	}
	if len(gotTexts) != len(want) {
		t.Fatalf("linearized messages = %v, want %v", gotTexts, want)
	}
	for i := range want {
		if gotTexts[i] != want[i] {
			t.Errorf("msg[%d] = %q, want %q", i, gotTexts[i], want[i])
		}
	}
	if !c1.CreatedAt.Equal(time.Unix(1720000000, 0).UTC()) {
		t.Errorf("c1.CreatedAt = %v", c1.CreatedAt)
	}

	// Conversation 2: hidden system node skipped; code part kept; roles kept.
	c2 := p.Threads[1]
	if len(c2.Messages) != 3 {
		t.Fatalf("c2 messages = %d, want 3 (hidden system skipped)", len(c2.Messages))
	}
	if c2.Messages[0].Role != "user" || c2.Messages[2].Role != "tool" {
		t.Errorf("c2 roles wrong: %+v", c2.Messages)
	}
	if c2.Messages[1].Text != "fmt.Println(\"hi\")" {
		t.Errorf("code content lost: %q", c2.Messages[1].Text)
	}
	if c2.Messages[2].Text != "hi" { // whitespace-normalizable text kept verbatim here
		t.Errorf("tool text = %q, want \"hi\"", c2.Messages[2].Text)
	}

	// Digest: the duplicate user prompt across conversations is counted.
	if p.Digest.DupMessages != 1 {
		t.Errorf("DupMessages = %d, want 1", p.Digest.DupMessages)
	}
	if len(p.Digest.PerProjectGroups["博客"]) != 1 || len(p.Digest.PerProjectGroups[""]) != 1 {
		t.Errorf("groups wrong: %v", p.Digest.PerProjectGroups)
	}
	if p.Meta.ExportedAt.Unix() != 1720010500 {
		t.Errorf("ExportedAt = %v, want 1720010500", p.Meta.ExportedAt)
	}
}

func TestChatGPTParseRealExportFile(t *testing.T) {
	// Integration check against the shipped sample, when present.
	path := filepath.Join("..", "..", "examples", "sample-chatgpt-export.zip")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("shipped sample zip not built yet")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ChatGPT{}.Parse(zr)
	if err != nil {
		t.Fatal(err)
	}
	if p.Meta.NConversations == 0 || p.Meta.NMessages == 0 {
		t.Errorf("sample parsed empty: %+v", p.Meta)
	}
}
