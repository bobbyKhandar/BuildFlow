package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"buildflow/internal/appdata"
	"buildflow/internal/structured"
	"buildflow/internal/workspace"
)

// ---------------------------------------------------------------------------
// health and diagnostics
// ---------------------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Deliberately unauthenticated and free of details: this is how a second
	// copy of the application checks whether an instance is already running.
	root := s.svc.Root()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"app":           "BuildFlow",
		"version":       Version,
		"port":          s.app.Port,
		"url":           s.LocalURL(),
		"pid":           os.Getpid(),
		"uptimeSeconds": int(time.Since(s.app.Started).Seconds()),
		"workspaceSet":  root != "",
	})
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	settings := s.app.Store.Settings()
	paths := s.app.Paths()
	root := s.svc.Root()
	info := map[string]any{
		"version":       Version,
		"pid":           os.Getpid(),
		"url":           s.LocalURL(),
		"bind":          "127.0.0.1 only (loopback)",
		"port":          s.app.Port,
		"startedAt":     s.app.Started.UTC(),
		"uptimeSeconds": int(time.Since(s.app.Started).Seconds()),
		"executable":    exePath(),
		"appData":       paths.Home,
		"configFile":    paths.Config,
		"logFile":       paths.Log,
		"tokenFile":     paths.Token,
		"tokenHeader":   TokenHeader,
		"workspace":     root,
		"autostart":     settings.LoginStartupEnabled(),
		"startAtLogin":  settings.LoginStartupEnabled(),
		"autostartCmd":  s.app.AutostartCmd(),
		"openOnStart":   settings.OpenBrowserOnStart,
		"lastProject":   settings.LastProject,
	}
	if root != "" {
		exists, writable, err := workspace.CheckWorkspaceDir(root)
		info["workspaceExists"] = exists
		info["workspaceWritable"] = writable
		if err != nil {
			info["workspaceError"] = err.Error()
		}
	}
	writeJSON(w, http.StatusOK, info)
}

func exePath() string {
	if p, err := os.Executable(); err == nil {
		return p
	}
	return os.Args[0]
}

// ---------------------------------------------------------------------------
// settings
// ---------------------------------------------------------------------------

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings := s.app.Store.Settings()
	writeJSON(w, http.StatusOK, map[string]any{
		"port":               settings.Port,
		"url":                s.LocalURL(),
		"workspacePath":      settings.WorkspacePath,
		"workspaceSet":       settings.WorkspacePath != "",
		"startWithWindows":   settings.LoginStartupEnabled(),
		"startAtLogin":       settings.LoginStartupEnabled(),
		"openBrowserOnStart": settings.OpenBrowserOnStart,
		"backgroundLaunch":   settings.BackgroundLaunch,
		"lastProject":        settings.LastProject,
		"appData":            s.app.Paths().Home,
		"configFile":         s.app.Paths().Config,
		"logFile":            s.app.Paths().Log,
		"tokenFile":          s.app.Paths().Token,
		"version":            Version,
		"portLocked":         true,
		"portHint":           "The port is fixed at " + s.LocalURL() + " for a stable bookmark. Change it by editing config.json, then restart BuildFlow.",
	})
}

type settingsUpdate struct {
	OpenBrowserOnStart *bool `json:"openBrowserOnStart"`
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsUpdate
	if !readBody(w, r, &req) {
		return
	}
	if req.OpenBrowserOnStart != nil {
		if err := s.app.Store.Update(func(st *appdata.Settings) {
			st.OpenBrowserOnStart = *req.OpenBrowserOnStart
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save settings: "+err.Error())
			return
		}
	}
	s.handleGetSettings(w, r)
}

type workspaceRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleSetWorkspace(w http.ResponseWriter, r *http.Request) {
	var req workspaceRequest
	if !readBody(w, r, &req) {
		return
	}
	clean, err := workspace.ValidateWorkspacePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	exists, writable, err := workspace.CheckWorkspaceDir(clean)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created := false
	if !exists {
		if err := os.MkdirAll(clean, 0o755); err != nil {
			writeError(w, http.StatusBadRequest, "cannot create the workspace folder: "+err.Error())
			return
		}
		created = true
		// The folder did not exist, so the writability probe above never ran.
		// Re-check now that it does, otherwise a brand-new workspace folder
		// would be refused as "not writable".
		_, writable, err = workspace.CheckWorkspaceDir(clean)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if !writable {
		writeError(w, http.StatusBadRequest, clean+" is not writable by the current user")
		return
	}
	scaffold, err := s.svc.EnsureWorkspace(clean)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot prepare the workspace: "+err.Error())
		return
	}
	if err := s.app.Store.SetWorkspacePath(clean); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot remember the workspace: "+err.Error())
		return
	}
	s.log.Infof("workspace set to %s (created=%v, new entries=%v)", clean, created, len(scaffold.Created))
	writeJSON(w, http.StatusOK, map[string]any{
		"workspacePath": clean,
		"created":       created,
		"scaffolding":   scaffold,
	})
}

func (s *Server) handleValidateWorkspace(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("path")
	clean, err := workspace.ValidateWorkspacePath(raw)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid": false, "input": raw, "error": err.Error(),
		})
		return
	}
	out := map[string]any{"valid": true, "input": raw, "path": clean}
	exists, writable, err := workspace.CheckWorkspaceDir(clean)
	out["exists"] = exists
	out["writable"] = writable
	if err != nil {
		out["warning"] = err.Error()
	}
	if !exists {
		out["willCreate"] = true
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	if s.app.PickFolder == nil {
		writeError(w, http.StatusNotImplemented, "the native folder picker is not available in this build; type the path instead")
		return
	}
	// The dialog is modal, so allow plenty of time.
	selected, err := s.app.PickFolder("Choose the BuildFlow workspace folder")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "the folder picker failed: "+err.Error())
		return
	}
	if strings.TrimSpace(selected) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": false, "path": selected})
}

type autostartRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleSetAutostart(w http.ResponseWriter, r *http.Request) {
	var req autostartRequest
	if !readBody(w, r, &req) {
		return
	}
	if s.app.Autostart == nil {
		writeError(w, http.StatusNotImplemented, "login startup is not available in this build")
		return
	}
	if err := s.app.Autostart(req.Enabled); err != nil {
		s.log.Errorf("could not change login startup: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.app.Store.Update(func(st *appdata.Settings) {
		st.StartWithWindows = req.Enabled
		st.StartAtLogin = &req.Enabled
	}); err != nil {
		s.log.Warnf("login startup changed but could not be saved to config.json: %v", err)
	}
	s.log.Infof("start at login = %v", req.Enabled)
	writeJSON(w, http.StatusOK, map[string]any{
		"startWithWindows": s.app.AutostartNow(),
		"startAtLogin":     s.app.AutostartNow(),
		"command":          s.app.AutostartCmd(),
		"location":         s.app.AutostartLocation(),
	})
}

// ---------------------------------------------------------------------------
// stages and templates
// ---------------------------------------------------------------------------

func (s *Server) handleStages(w http.ResponseWriter, r *http.Request) {
	ids := workspace.StageIDs()
	readable := workspace.StageIDs()
	type stageView struct {
		ID         string `json:"id"`
		Num        string `json:"num"`
		Title      string `json:"title"`
		Question   string `json:"question"`
		PerPackage bool   `json:"perPackage"`
	}
	out := make([]stageView, 0, len(ids))
	for _, st := range workspace.Stages {
		out = append(out, stageView{
			ID: st.ID, Num: st.Num, Title: st.Title, Question: st.Question,
			PerPackage: st.ID == "step-05",
		})
	}
	_ = readable
	writeJSON(w, http.StatusOK, map[string]any{
		"stages":    out,
		"readable":  ids,
		"tokens":    workspace.KnownTokens,
		"tokenHelp": workspace.TokenHelp,
		"starter":   workspace.StarterTemplateIDs(),
	})
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	type tmplView struct {
		Stage        string `json:"stage"`
		GlobalExists bool   `json:"globalExists"`
		LocalExists  bool   `json:"localExists"`
		ActiveScope  string `json:"activeScope"`
		Modified     bool   `json:"modifiedFromGlobal"`
		IsStarter    bool   `json:"isStarter"`
		Title        string `json:"title"`
	}
	out := []tmplView{}
	for _, stage := range workspace.Stages {
		global, err1 := s.svc.GetTemplate(stage.ID, "global", "")
		local, err2 := s.svc.GetTemplate(stage.ID, "local", "")
		if err1 != nil || err2 != nil {
			continue
		}
		v := tmplView{
			Stage: stage.ID, GlobalExists: global.Exists, LocalExists: local.Exists,
			ActiveScope: local.ActiveScope,
			Modified:    local.Modified, IsStarter: local.IsStarter, Title: local.Title,
		}
		if v.Title == "" {
			v.Title = stage.Title
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}

// ---------------------------------------------------------------------------
// projects
// ---------------------------------------------------------------------------

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	projects, err := s.svc.ListProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []workspace.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projects":  projects,
		"workspace": s.svc.Root(),
	})
}

type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var req createProjectRequest
	if !readBody(w, r, &req) {
		return
	}
	p, err := s.svc.CreateProject(req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Infof("created project %q at %s", p.ID, p.Path)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	p, err := s.svc.GetProject(r.PathValue("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, workspace.ErrNoWorkspace) {
			status = http.StatusPreconditionRequired
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type updateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	id := r.PathValue("id")
	var req updateProjectRequest
	if !readBody(w, r, &req) {
		return
	}
	current, err := s.svc.GetProject(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Description != nil {
		dir, _ := s.svc.ProjectPath(id)
		if err := s.svc.SetProjectDescription(id, *req.Description); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = dir
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != current.Name {
		updated, err := s.svc.RenameProject(id, *req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if settings := s.app.Store.Settings(); settings.LastProject == id {
			_ = s.app.Store.SetLastProject(updated.ID)
		}
		s.log.Infof("renamed project %q to %q (folder %q)", id, updated.Name, updated.ID)
		writeJSON(w, http.StatusOK, updated)
		return
	}
	p, err := s.svc.GetProject(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type deleteProjectRequest struct {
	Confirm string `json:"confirm"`
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var req deleteProjectRequest
	if !readBody(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if err := s.svc.DeleteProject(id, req.Confirm); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Warnf("deleted project %q", id)
	if s.app.Store.Settings().LastProject == id {
		_ = s.app.Store.SetLastProject("")
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// ---------------------------------------------------------------------------
// documents
// ---------------------------------------------------------------------------

type docResponse struct {
	workspace.Doc
	ReadOnly bool   `json:"readOnly"`
	Reason   string `json:"reason,omitempty"`
}

func (s *Server) handleGetDoc(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	id := r.PathValue("id")
	kind := r.URL.Query().Get("kind")
	stage := r.URL.Query().Get("stage")
	pkg := r.URL.Query().Get("package")
	doc, err := s.svc.ReadDoc(kind, stage, pkg, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		workspace.Doc
		Structured *structured.Result `json:"structured,omitempty"`
	}{doc, s.svc.InspectDocument(id, doc)})
}

type putDocRequest struct {
	Kind     string `json:"kind"`
	Stage    string `json:"stage"`
	Package  string `json:"package"`
	Content  string `json:"content"`
	BaseHash string `json:"baseHash"`
	Force    bool   `json:"force"`
}

func (s *Server) handlePutDoc(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	id := r.PathValue("id")
	var req putDocRequest
	if !readBody(w, r, &req) {
		return
	}
	// A work package written from Step 4 is the package's content, so make sure
	// it carries the front matter Step 5 expects.
	content := req.Content
	if req.Kind == workspace.KindPackage {
		content = workspace.EnsurePackageFrontMatter(req.Package, content)
	}
	doc, err := s.svc.WriteDoc(workspace.WriteRequest{
		Kind: req.Kind, Stage: req.Stage, Package: req.Package, Project: id,
		Content: content, BaseHash: req.BaseHash, Force: req.Force,
	})
	if err != nil {
		var ce *workspace.ConflictError
		if errors.As(err, &ce) {
			s.log.Warnf("refused to overwrite %s: %s", ce.Path, ce.Reason)
			writeErrorDetails(w, http.StatusConflict, ce.Error(), ce)
			return
		}
		s.log.Errorf("write failed for project %s: %v", id, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		workspace.Doc
		Structured *structured.Result `json:"structured,omitempty"`
	}{doc, s.svc.InspectDocument(id, doc)})
}

func (s *Server) handleDeleteDoc(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	id := r.PathValue("id")
	kind := r.URL.Query().Get("kind")
	stage := r.URL.Query().Get("stage")
	pkg := r.URL.Query().Get("package")
	if err := s.svc.DeleteDoc(kind, stage, pkg, id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ---------------------------------------------------------------------------
// templates
// ---------------------------------------------------------------------------

// noProjectID is the placeholder the interface sends when no project is
// selected yet. A local template needs a real project, so the request is
// answered with the global template instead of failing.
const noProjectID = "_"

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	stage := r.URL.Query().Get("stage")
	scope := r.URL.Query().Get("scope")
	id := r.PathValue("id")
	if id == noProjectID {
		scope = "global"
		id = ""
	}
	tmpl, err := s.svc.GetTemplate(stage, scope, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

type saveTemplateRequest struct {
	Stage    string `json:"stage"`
	Scope    string `json:"scope"`
	Content  string `json:"content"`
	BaseHash string `json:"baseHash"`
	Force    bool   `json:"force"`
}

func (s *Server) handleSaveTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var req saveTemplateRequest
	if !readBody(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if id == noProjectID {
		id = ""
	}
	if req.Scope == "local" && id == "" {
		writeError(w, http.StatusBadRequest, "select a project before saving a local prompt override")
		return
	}
	tmpl, err := s.svc.SaveTemplate(req.Stage, req.Scope, id, req.Content, req.BaseHash, req.Force)
	if err != nil {
		var ce *workspace.ConflictError
		if errors.As(err, &ce) {
			writeErrorDetails(w, http.StatusConflict, ce.Error(), ce)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

func (s *Server) handleRevertTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	stage := r.URL.Query().Get("stage")
	expected := r.URL.Query().Get("expectedHash")
	tmpl, err := s.svc.RevertTemplate(stage, r.PathValue("id"), expected)
	if err != nil {
		var ce *workspace.ConflictError
		if errors.As(err, &ce) {
			writeErrorDetails(w, http.StatusConflict, ce.Error(), ce)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

// ---------------------------------------------------------------------------
// assembly and snapshots
// ---------------------------------------------------------------------------

func (s *Server) handleAssemble(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	stage := r.URL.Query().Get("stage")
	pkg := r.URL.Query().Get("package")
	out, err := s.svc.AssemblePrompt(r.PathValue("id"), stage, pkg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type snapshotRequest struct {
	Stage   string `json:"stage"`
	Package string `json:"package"`
	Content string `json:"content"`
}

func (s *Server) handleSaveSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var req snapshotRequest
	if !readBody(w, r, &req) {
		return
	}
	out, err := s.svc.SaveSnapshot(r.PathValue("id"), req.Stage, req.Package, req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Infof("saved prompt snapshot for %s%s", req.Stage, packageSuffix(req.Package))
	writeJSON(w, http.StatusOK, out)
}

func packageSuffix(pkg string) string {
	if pkg == "" {
		return ""
	}
	return "/" + pkg
}

// ---------------------------------------------------------------------------
// packages
// ---------------------------------------------------------------------------

type detectRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleDetectPackages(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var req detectRequest
	if !readBody(w, r, &req) {
		return
	}
	content := req.Content
	if strings.TrimSpace(content) == "" {
		// Fall back to the saved Step 4 output.
		doc, err := s.svc.ReadDoc(workspace.KindOutput, "step-04", "", r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !doc.Exists {
			writeError(w, http.StatusBadRequest, "there is no Step 4 output to split yet. Save Step 4's output first, or paste text into the detector.")
			return
		}
		content = doc.Content
	}
	res, err := s.svc.DetectPackages(r.PathValue("id"), content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type importRequest struct {
	Packages  []workspace.DetectedPackage `json:"packages"`
	Accept    []string                    `json:"accept"`
	Overwrite []string                    `json:"overwrite"`
}

func (s *Server) handleImportPackages(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var req importRequest
	if !readBody(w, r, &req) {
		return
	}
	if len(req.Packages) == 0 {
		writeError(w, http.StatusBadRequest, "no packages were sent to import")
		return
	}
	res, err := s.svc.ImportPackages(r.PathValue("id"), workspace.ImportRequest{
		Packages: req.Packages, Accept: req.Accept, Overwrite: req.Overwrite,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Infof("package import for %q: created=%v overwritten=%v refused=%v",
		r.PathValue("id"), res.Created, res.Overwritten, res.Refused)
	writeJSON(w, http.StatusOK, res)
}

type createPackageRequest struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (s *Server) handleCreatePackage(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	var req createPackageRequest
	if !readBody(w, r, &req) {
		return
	}
	pkg, err := s.svc.CreatePackage(r.PathValue("id"), req.ID, req.Title)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Infof("created work package %s in %q", pkg.ID, r.PathValue("id"))
	writeJSON(w, http.StatusCreated, pkg)
}

// handleStep4Slots returns the work-package slots for Step 4. The list always
// ends with at least one empty slot, so the interface can always offer a place
// to add the next package.
func (s *Server) handleStep4Slots(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspace(w) {
		return
	}
	slots, err := s.svc.ListStep4Slots(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if slots == nil {
		slots = []workspace.Step4Slot{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

// ---------------------------------------------------------------------------
// app controls
// ---------------------------------------------------------------------------

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	if s.app.OpenBrowser == nil {
		writeError(w, http.StatusNotImplemented, "opening a browser is not available in this build")
		return
	}
	if err := s.app.OpenBrowser(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"opened": s.LocalURL()})
}

type copyRequest struct {
	Text string `json:"text"`
}

func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	if s.app.CopyToClip == nil {
		writeError(w, http.StatusNotImplemented, "clipboard access is not available in this build")
		return
	}
	var req copyRequest
	if !readBody(w, r, &req) {
		return
	}
	if err := s.app.CopyToClip(req.Text); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"copied": len([]rune(req.Text))})
}

func (s *Server) handleQuit(w http.ResponseWriter, r *http.Request) {
	if s.app.Quit == nil {
		writeError(w, http.StatusNotImplemented, "quitting is not available in this build")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"quitting": true})
	if err := s.app.Quit(); err != nil {
		s.log.Errorf("quit failed: %v", err)
		return
	}
}

// workspaceRelative renders a path for display, always with forward slashes.
func workspaceRelative(root, abs string) string {
	if root == "" {
		return filepath.ToSlash(abs)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

var _ = workspaceRelative
var _ = time.Now
