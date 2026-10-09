// cn_export.go reserves the ingest seam for the domestic-app (国产 App)
// export adapters. It is a documented stub in v0.1 (m1).
//
// Status: per the v0.1 plan, exactly one CN migration pair ships — in
// v0.3 (m3) and only after the Gate-0 audit confirms a parseable
// structured export plus an official upload / project-knowledge entry on
// the target app. Until that audit passes, nothing here parses any CN
// format, and no CN format layout is guessed in advance.
package ingest

import (
	"errors"
	"fmt"
)

// ErrCNAdapterNotWired is returned until the Gate-0-selected pair lands.
var ErrCNAdapterNotWired = errors.New("cn export adapter ships in v0.3 after the Gate-0 audit (see README roadmap)")

// ErrCNNotMatched means the archive does not look like a known CN export.
// With zero wired adapters it matches everything, so Detect only consults
// it once a real adapter exists.
var ErrCNNotMatched = errors.New("archive does not match any known cn export layout")

// detectCN is the dispatch hook Detect calls after the ChatGPT layout
// misses. v0.1 has no wired CN adapter, so it always reports not-matched;
// the v0.3 adapter will replace the body with real layout sniffing.
func detectCN(entryNames []string) error {
	return fmt.Errorf("%w (%w)", ErrCNNotMatched, ErrCNAdapterNotWired)
}
