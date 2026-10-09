// Package web serves the localhost preview UI embedded in the single
// chatport binary. stdlib net/http only; the UI ships as embedded static
// assets so the binary stays self-contained and fully offline.
//
// Status: the interactive preview (drag zip -> parse progress -> select
// conversations -> download the Context Pack) is milestone m2 (v0.2).
// v0.1 (m1) ships the server plumbing and an honest placeholder page that
// points users at `chatport parse` — it does not fake the m2 flow.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

//go:embed static
var staticFS embed.FS

// Server serves the preview UI and the small JSON API behind it.
type Server struct {
	version string
	srv     *http.Server
}

// New builds a Server advertising the given binary version.
func New(version string) *Server {
	return &Server{version: version}
}

// Handler assembles the routes: /api/version plus the embedded static page.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":    "chatport",
			"version": s.version,
		})
	})
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("web: embedded static missing: " + err.Error())
	}
	mux.Handle("/", http.FileServer(http.FS(static)))
	return mux
}

// Listen starts serving on addr (host:port). Use an empty port to get a
// free one; the chosen address is returned.
func (s *Server) Listen(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("web: listen %s: %w", addr, err)
	}
	s.srv = &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		_ = s.srv.Serve(ln) // returns on Shutdown
	}()
	return ln.Addr().String(), nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// OpenBrowser opens the default browser at url, best effort. It never
// fails the command when no opener exists (headless boxes, CI).
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return errors.New("web: no browser opener for " + runtime.GOOS)
	}
	return cmd.Start()
}
