// Package render turns Context Packs into bundles a target app can ingest.
//
// claude.go owns the Claude Projects renderer: a zip of Markdown files,
// grouped by project, deduplicated via the pack digest, and sharded so no
// single file exceeds the digest's size budget. This is milestone m2
// (v0.2): the contract below is final, the body is a documented stub in
// the v0.1 (m1) build.
package render

import (
	"errors"

	"github.com/SuperMarioYL/chatport/internal/ir"
)

// ErrNotImplemented reports the m2 renderer has not landed yet. Callers
// should surface the README roadmap, not a stack trace.
var ErrNotImplemented = errors.New("claude projects rendering ships in v0.2 (m2): see README roadmap")

// Shard is one output file of a rendered bundle.
type Shard struct {
	// Name is the archive-relative path, e.g. "blog-refactor/part-1.md".
	Name string
	// Body is the Markdown payload, UTF-8.
	Body []byte
}

// Bundle is the content of claude-projects-pack.zip.
type Bundle struct {
	// SourceApp the pack was ingested from.
	SourceApp string
	// Files are the Markdown shards, in deterministic order.
	Files []Shard
}

// Options tunes Claude Projects rendering.
type Options struct {
	// SizeBudget overrides ir.Pack.Digest.SizeBudgetPerFile when > 0.
	SizeBudget int
}

// ClaudeProjects renders p into a deduplicated, project-grouped, size-
// budgeted Markdown bundle ready for Claude Projects knowledge upload.
//
// v0.1 stub: returns ErrNotImplemented until m2.
func ClaudeProjects(p *ir.Pack, opt Options) (*Bundle, error) {
	if p == nil {
		return nil, errors.New("render: nil pack")
	}
	return nil, ErrNotImplemented
}
