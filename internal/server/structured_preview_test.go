package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"buildflow/internal/structured"
	"buildflow/internal/workspace"
)

func TestStructuredPreview(t *testing.T) {
	s, _ := startupTestServer(t)
	s.svc = workspace.NewService()
	root := t.TempDir()
	if _, err := s.svc.EnsureWorkspace(root); err != nil {
		t.Fatal(err)
	}
	project, err := s.svc.CreateProject("Draft preview", "")
	if err != nil {
		t.Fatal(err)
	}
	s.token = "preview-test-token"
	s.assets = fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("BuildFlow")}}
	handler := s.buildMux()
	fixture := func(name string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join("..", "structured", "testdata", name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	stageDraft := fixture("step-01")
	packageDraft := fixture("step-04-package")
	// Existing content must also survive previews of unsaved edits.
	if _, err := s.svc.WriteDoc(workspace.WriteRequest{Project: project.ID, Kind: workspace.KindOutput, Stage: "step-01", Content: "Saved answer"}); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]string {
		t.Helper()
		files := map[string]string{}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			files[path] = "directory"
			if !entry.IsDir() {
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[path] = string(content)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	for _, tc := range []struct {
		name, kind, stage, pkg, content, raw string
		missingProject, unauthorized         bool
		code                                 int
		status                               structured.Status
	}{
		{name: "valid stage draft", kind: workspace.KindOutput, stage: "step-01", content: stageDraft, code: 200, status: structured.Valid},
		{name: "valid unsaved package", kind: workspace.KindPackage, pkg: "P01", content: packageDraft, code: 200, status: structured.Valid},
		{name: "invalid JSON", raw: `{`, code: 400},
		{name: "unknown field", raw: `{"extra":true}`, code: 400},
		{name: "wrong kind", kind: "unknown", stage: "step-01", code: 400},
		{name: "unsupported kind", kind: workspace.KindInput, stage: "step-01", code: 400},
		{name: "wrong stage", kind: workspace.KindOutput, stage: "unknown", code: 400},
		{name: "wrong package", kind: workspace.KindPackage, pkg: "../P01", code: 400},
		{name: "missing package", kind: workspace.KindPackage, code: 400},
		{name: "footer target mismatch", kind: workspace.KindOutput, stage: "step-02", content: stageDraft, code: 200, status: structured.Invalid},
		{name: "missing project", kind: workspace.KindOutput, stage: "step-01", missingProject: true, code: 400},
		{name: "authentication required", unauthorized: true, code: 401},
		{name: "body limit", kind: workspace.KindOutput, stage: "step-01", content: strings.Repeat("x", maxBodyBytes), code: 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(tc.raw)
			if tc.raw == "" {
				body, err = json.Marshal(structuredPreviewRequest{Kind: tc.kind, Stage: tc.stage, PackageID: tc.pkg, Content: tc.content})
				if err != nil {
					t.Fatal(err)
				}
			}
			id := project.ID
			if tc.missingProject {
				id = "missing-project"
			}
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/projects/"+id+"/structured/preview", bytes.NewReader(body))
			if !tc.unauthorized {
				req.Header.Set(TokenHeader, s.token)
			}
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.code {
				t.Fatalf("status = %d, want %d: %s", out.Code, tc.code, out.Body.String())
			}
			if tc.status != "" {
				var result structured.Result
				if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Status != tc.status {
					t.Fatalf("result = %+v", result)
				}
				expected := s.svc.InspectDocument(id, workspace.Doc{Kind: tc.kind, Stage: tc.stage, Package: tc.pkg, Content: tc.content})
				want, err := json.Marshal(expected)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(bytes.TrimSpace(out.Body.Bytes()), want) {
					t.Fatal("preview differs from saved-document inspection")
				}
			}
			if !reflect.DeepEqual(before, snapshot()) {
				t.Fatal("preview modified workspace files")
			}
		})
	}
}
