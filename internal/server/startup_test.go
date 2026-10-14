package server

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"buildflow/internal/appdata"
	"buildflow/internal/logging"
)

func startupTestServer(t *testing.T) (*Server, *appdata.Store) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "isolated")
	t.Setenv(appdata.EnvOverride, root)
	store, err := appdata.Open()
	if err != nil {
		t.Fatal(err)
	}
	logger, err := logging.New(store.LogDir(), logging.LevelInfo, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	app := &App{
		Store: store, Log: logger,
		Paths: func() DiagnosticsPaths {
			return DiagnosticsPaths{Config: store.ConfigPath(), Log: filepath.Join(store.LogDir(), "buildflow.log"), Token: store.TokenPath(), Home: store.RootPath()}
		},
		AutostartNow:      func() bool { return store.LoginStartupEnabled() },
		AutostartCmd:      func() string { return `"/tmp/Build Flow" --minimized` },
		AutostartLocation: func() string { return filepath.Join(root, "autostart", "buildflow.desktop") },
	}
	return &Server{app: app, log: logger}, store
}

func TestSettingsAPIStartupPrecedenceAndAlias(t *testing.T) {
	s, store := startupTestServer(t)
	if err := store.Update(func(st *appdata.Settings) {
		canonical := false
		st.StartAtLogin = &canonical
		st.StartWithWindows = true
	}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	s.handleGetSettings(r, httptest.NewRequest("GET", "/api/settings", nil))
	var out map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["startAtLogin"] != false || out["startWithWindows"] != false {
		t.Fatalf("canonical false did not win: %#v", out)
	}

	if err := store.Update(func(st *appdata.Settings) { st.StartAtLogin = nil; st.StartWithWindows = true }); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRecorder()
	s.handleGetSettings(r, httptest.NewRequest("GET", "/api/settings", nil))
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["startAtLogin"] != true || out["startWithWindows"] != true {
		t.Fatalf("legacy fallback/alias failed: %#v", out)
	}
}

func TestAutostartRequestShapeAndCanonicalPersistence(t *testing.T) {
	s, store := startupTestServer(t)
	var applied bool
	s.app.Autostart = func(enabled bool) error { applied = enabled; return nil }
	r := httptest.NewRecorder()
	s.handleSetAutostart(r, httptest.NewRequest("POST", "/api/settings/autostart", strings.NewReader(`{"enabled":false}`)))
	if r.Code != 200 {
		t.Fatalf("status = %d, body=%s", r.Code, r.Body.String())
	}
	if applied {
		t.Fatal("enabled=false request changed to true")
	}
	settings := store.Settings()
	if settings.StartAtLogin == nil || *settings.StartAtLogin || settings.StartWithWindows {
		t.Fatalf("startup settings were not synchronized: %#v", settings)
	}
	var out map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["startAtLogin"] != false || out["startWithWindows"] != false {
		t.Fatalf("response aliases differ: %#v", out)
	}
}
