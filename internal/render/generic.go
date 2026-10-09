// generic.go owns the generic knowledge-base renderer: the same Context
// Pack rendered as a flat, app-agnostic Markdown knowledge upload that any
// assistant with a file-upload entry can consume (Q&A upload panels,
// project knowledge, prompt libraries). This is milestone m3 (v0.3) and is
// a documented stub in the v0.1 (m1) build.
package render

import (
	"errors"

	"github.com/SuperMarioYL/chatport/internal/ir"
)

// ErrGenericNotImplemented reports the m3 renderer has not landed yet.
var ErrGenericNotImplemented = errors.New("generic knowledge-base rendering ships in v0.3 (m3): see README roadmap")

// Generic renders p into a flat Markdown knowledge bundle with one file
// per project group plus an index, deduplicated via the pack digest.
//
// v0.1 stub: returns ErrGenericNotImplemented until m3.
func Generic(p *ir.Pack) (*Bundle, error) {
	if p == nil {
		return nil, errors.New("render: nil pack")
	}
	return nil, ErrGenericNotImplemented
}
