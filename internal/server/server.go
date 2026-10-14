// Package server exposes BuildFlow's local REST API and its embedded web
// interface. The listener is bound to 127.0.0.1 only and every request is
// checked for an approved origin and a valid local API token, so a web page
// loaded from the internet cannot read or change anything.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"buildflow/internal/appdata"
	"buildflow/internal/llm"
	"buildflow/internal/logging"
	"buildflow/internal/workspace"
)

// Version is reported by the health endpoint and the diagnostics panel.
const Version = "v0.5"

// TokenHeader is the header a local script must send to use the REST API.
const TokenHeader = "X-BuildFlow-Token"

// maxBodyBytes caps request bodies. Prompt documents are Markdown, so 8 MiB is
// far more than a real document needs.
const maxBodyBytes = 8 << 20

// App carries the process-level controls the API needs.
type App struct {
	Store             *appdata.Store
	LLM               *llm.Manager
	Log               *logging.Logger
	Port              int
	BaseURL           string
	Started           time.Time
	OpenBrowser       func() error
	OpenURL           func(string) error
	OpenFolder        func(string) error
	PickFolder        func(title string) (string, error)
	Autostart         func(enabled bool) error
	AutostartNow      func() bool
	AutostartCmd      func() string
	AutostartLocation func() string
	Quit              func() error
	Paths             func() DiagnosticsPaths
	CopyToClip        func(string) error
	Version           string
}

// DiagnosticsPaths describes files the user is told about.
type DiagnosticsPaths struct {
	Config string `json:"config"`
	Log    string `json:"log"`
	Token  string `json:"token"`
	Home   string `json:"home"`
}

// Server owns the HTTP handler and listener.
type Server struct {
	app     *App
	svc     *workspace.Service
	token   string
	assets  fs.FS
	handler http.Handler
	log     *logging.Logger
	// restart is set by main so the process can exit with a specific code.
	restart func()
}

// New builds the handler. assets must be the embedded web file system.
func New(app *App, svc *workspace.Service, token string, assets fs.FS, restart func()) *Server {
	s := &Server{app: app, svc: svc, token: token, assets: assets, log: app.Log, restart: restart}
	s.handler = s.buildMux()
	return s
}

// Handler exposes the fully wrapped handler, used by tests.
func (s *Server) Handler() http.Handler { return s.handler }

// LocalURL is the address the interface is served on.
func (s *Server) LocalURL() string { return "http://127.0.0.1:" + strconv.Itoa(s.app.Port) }

// LoopbackOnly is the address the listener binds to. It is never 0.0.0.0.
const LoopbackOnly = "127.0.0.1"

// Listen binds the loopback listener. A failure here is reported to the caller
// so it can be shown and logged.
func (s *Server) Listen() (net.Listener, error) {
	addr := net.JoinHostPort(LoopbackOnly, strconv.Itoa(s.app.Port))
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		return nil, fmt.Errorf("cannot listen on %s: %w", addr, err)
	}
	return ln, nil
}

// ListenRetry behaves like Listen but keeps trying for a short while before
// giving up. An instance that has just exited can still be releasing the port,
// and a user who double-clicks the shortcut twice in a row should not be told
// to go and change the port number. The delay is only spent when the first
// attempt genuinely failed.
func (s *Server) ListenRetry(within time.Duration) (net.Listener, error) {
	ln, err := s.Listen()
	if err == nil {
		return ln, nil
	}
	deadline := time.Now().Add(within)
	for attempt := 1; time.Now().Before(deadline); attempt++ {
		time.Sleep(150 * time.Millisecond)
		ln, err = s.Listen()
		if err == nil {
			return ln, nil
		}
		_ = attempt
	}
	return nil, err
}

// Serve runs the HTTP server on the given listener until the context is
// cancelled.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    32 << 10,
		ErrorLog:          log.New(discardWriter{}, "", 0),
	}
	errCh := make(chan error, 1)
	go func() {
		s.log.Infof("HTTP server listening on %s (loopback only)", s.LocalURL())
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return <-errCh
	case err := <-errCh:
		return err
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// ---------------------------------------------------------------------------
// middleware
// ---------------------------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// allowedOrigins lists the origins that may talk to this server. Anything else
// is refused, which is what stops a page on the internet from making a
// cross-site request to 127.0.0.1.
func (s *Server) allowedOrigins() map[string]bool {
	out := map[string]bool{
		"null": false, // explicitly not allowed
	}
	for _, host := range []string{"127.0.0.1", "localhost", "[::1]"} {
		out["http://"+host+":"+strconv.Itoa(s.app.Port)] = true
		out["https://"+host+":"+strconv.Itoa(s.app.Port)] = true
	}
	return out
}

func (s *Server) buildMux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/diagnostics", s.handleDiagnostics)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", s.handleUpdateSettings)
	mux.HandleFunc("POST /api/settings/workspace", s.handleSetWorkspace)
	mux.HandleFunc("GET /api/settings/workspace/validate", s.handleValidateWorkspace)
	mux.HandleFunc("POST /api/settings/workspace/pick", s.handlePickFolder)
	mux.HandleFunc("POST /api/settings/autostart", s.handleSetAutostart)
	mux.HandleFunc("GET /api/llm", s.handleGetLLMConfig)
	mux.HandleFunc("PUT /api/llm", s.handleSetLLMConfig)
	mux.HandleFunc("PUT /api/llm/key", s.handleSetLLMKey)
	mux.HandleFunc("DELETE /api/llm/key", s.handleRemoveLLMKey)
	mux.HandleFunc("GET /api/llm/models", s.handleLLMModels)
	mux.HandleFunc("POST /api/llm/chat", s.handleLLMChat)
	mux.HandleFunc("GET /api/stages", s.handleStages)
	mux.HandleFunc("GET /api/templates", s.handleListTemplates)

	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	mux.HandleFunc("PATCH /api/projects/{id}", s.handleUpdateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)

	mux.HandleFunc("GET /api/projects/{id}/doc", s.handleGetDoc)
	mux.HandleFunc("PUT /api/projects/{id}/doc", s.handlePutDoc)
	mux.HandleFunc("DELETE /api/projects/{id}/doc", s.handleDeleteDoc)
	mux.HandleFunc("POST /api/projects/{id}/detect-packages", s.handleDetectPackages)
	mux.HandleFunc("POST /api/projects/{id}/import-packages", s.handleImportPackages)
	mux.HandleFunc("POST /api/projects/{id}/packages", s.handleCreatePackage)
	mux.HandleFunc("GET /api/projects/{id}/step4", s.handleStep4Slots)
	mux.HandleFunc("GET /api/projects/{id}/assemble", s.handleAssemble)
	mux.HandleFunc("GET /api/projects/{id}/template", s.handleGetTemplate)
	mux.HandleFunc("PUT /api/projects/{id}/template", s.handleSaveTemplate)
	mux.HandleFunc("DELETE /api/projects/{id}/template", s.handleRevertTemplate)
	mux.HandleFunc("POST /api/projects/{id}/snapshot", s.handleSaveSnapshot)

	mux.HandleFunc("POST /api/app/open", s.handleOpen)
	mux.HandleFunc("POST /api/app/quit", s.handleQuit)
	mux.HandleFunc("POST /api/app/copy", s.handleCopy)

	// The embedded interface.
	assets := http.FileServer(http.FS(s.assets))
	index, err := fs.ReadFile(s.assets, "index.html")
	if err != nil {
		panic("server: embedded index.html is missing: " + err.Error())
	}
	page := strings.ReplaceAll(string(index), "__BUILDFLOW_TOKEN__", s.token)
	page = strings.ReplaceAll(page, "__BUILDFLOW_VERSION__", Version)
	page = strings.ReplaceAll(page, "__BUILDFLOW_PORT__", strconv.Itoa(s.app.Port))
	page = strings.ReplaceAll(page, "__BUILDFLOW_URL__", s.LocalURL())

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "no such API endpoint")
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			serveIndex(w, r, page)
			return
		}
		raw, err := fs.ReadFile(s.assets, p)
		if err != nil {
			// Unknown path: hand back the workbench rather than a bare 404.
			if r.Method == http.MethodGet {
				serveIndex(w, r, page)
				return
			}
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		serveAsset(w, r, p, raw)
		_ = assets
	})

	return s.securityChain(mux)
}

func serveIndex(w http.ResponseWriter, r *http.Request, page string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, page)
}

func serveAsset(w http.ResponseWriter, r *http.Request, name string, raw []byte) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	// No caching: the interface is served from inside the executable and must
	// never go stale after an upgrade.
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

// securityChain applies the protections in a fixed order.
func (s *Server) securityChain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		defer func() {
			if rec.status >= 500 {
				s.log.Errorf("%s %s from %s -> %d", r.Method, r.URL.Path, clientLabel(r), rec.status)
			}
		}()

		// 1. Baseline response headers on everything, including errors.
		h := rec.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; "+
				"connect-src 'self'; font-src 'self'; form-action 'none'; frame-ancestors 'none'; base-uri 'none'")
		// No CORS headers are ever sent. That is deliberate: see README.

		// 2. DNS rebinding protection: only the loopback names we serve are
		//    accepted in Host.
		if !s.hostAllowed(rec, r) {
			s.deny(rec, http.StatusForbidden, "Host header is not a loopback address for this BuildFlow instance")
			return
		}

		// 3. Origin checks for everything. A request with no Origin is a local
		//    client (curl, a script) and is handled by the token check below.
		if origin := r.Header.Get("Origin"); origin != "" {
			if !s.allowedOrigins()[strings.ToLower(strings.TrimSpace(origin))] {
				s.log.Warnf("rejected request from unapproved origin %q (%s %s)", origin, r.Method, r.URL.Path)
				s.deny(rec, http.StatusForbidden, "origin is not allowed to call the BuildFlow API")
				return
			}
		}
		if site := strings.ToLower(r.Header.Get("Sec-Fetch-Site")); site == "cross-site" {
			s.log.Warnf("rejected cross-site request (Sec-Fetch-Site: %s) %s %s", site, r.Method, r.URL.Path)
			s.deny(rec, http.StatusForbidden, "cross-site requests are refused")
			return
		}

		// 4. Token check for every API call except health, which carries no
		//    information and is used to detect an already-running instance.
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/health" {
			if !s.tokenOK(r) {
				s.deny(rec, http.StatusUnauthorized,
					"missing or invalid "+TokenHeader+". See the README section 'Using the REST API from a script'.")
				return
			}
		}

		next.ServeHTTP(rec, r)
	})
}

func (s *Server) hostAllowed(w http.ResponseWriter, r *http.Request) bool {
	host := r.Host
	if host == "" {
		return false
	}
	if hostOnly, _, err := net.SplitHostPort(host); err == nil {
		host = hostOnly
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	s.log.Warnf("rejected request with unexpected Host header %q", r.Host)
	return false
}

func (s *Server) tokenOK(r *http.Request) bool {
	supplied := r.Header.Get(TokenHeader)
	if supplied == "" {
		// Allow the token in a query parameter for convenience with tooling
		// that cannot set headers, but only for read-only requests.
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			supplied = r.URL.Query().Get("token")
		}
	}
	if supplied == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(s.token)) == 1
}

func (s *Server) deny(w http.ResponseWriter, code int, message string) {
	writeError(w, code, message)
}

func clientLabel(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type errorBody struct {
	Error   string `json:"error"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, errorBody{Error: message})
}

func writeErrorDetails(w http.ResponseWriter, code int, message string, details any) {
	writeJSON(w, code, errorBody{Error: message, Details: details})
}

// readBody decodes a JSON request body with a hard size limit.
func readBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
			return false
		}
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "request body is empty")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func (s *Server) requireWorkspace(w http.ResponseWriter) bool {
	if s.svc.Root() == "" {
		writeError(w, http.StatusPreconditionRequired,
			"no workspace folder selected yet. Open Settings and choose the BuildFlow workspace folder.")
		return false
	}
	return true
}

func methodNotAllowed(w http.ResponseWriter, allowed []string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// fileInfoString renders a byte count for human-readable messages.
func fileInfoString(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	}
}

var _ = fileInfoString
var _ = os.Getenv
