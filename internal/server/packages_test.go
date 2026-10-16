package server

import (
	"buildflow/internal/structured"
	"buildflow/internal/workspace"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageSlotAPIReturnsBackendHashesAndBlocks(t *testing.T) {
	s, _ := startupTestServer(t)
	s.svc = workspace.NewService()
	if _, err := s.svc.EnsureWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	project, err := s.svc.CreateProject("Slot API", "")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join("..", "structured", "testdata", "step-04-package.md"))
	if err != nil {
		t.Fatal(err)
	}
	put := func(content, hash string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(putDocRequest{Kind: workspace.KindPackage, Package: "P01", Content: content, BaseHash: hash})
		req := httptest.NewRequest(http.MethodPut, "/doc", bytes.NewReader(body))
		req.SetPathValue("id", project.ID)
		out := httptest.NewRecorder()
		s.handlePutDoc(out, req)
		return out
	}
	first := put(string(content), "")
	if first.Code != http.StatusOK {
		t.Fatal(first.Body.String())
	}
	var saved struct {
		workspace.Doc
		Structured structured.Result `json:"structured"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	disk, _ := s.svc.ReadDoc(workspace.KindPackage, "", "P01", project.ID)
	if len(saved.Hash) != 64 || saved.Hash != disk.Hash || saved.Structured.Status != structured.Valid {
		t.Fatalf("save response: %+v", saved)
	}
	req := httptest.NewRequest(http.MethodGet, "/step4", nil)
	req.SetPathValue("id", project.ID)
	out := httptest.NewRecorder()
	s.handleStep4Slots(out, req)
	var slots struct {
		Slots []workspace.Step4Slot `json:"slots"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &slots); err != nil {
		t.Fatal(err)
	}
	if len(slots.Slots) < 1 || slots.Slots[0].OutputHash != saved.Hash || slots.Slots[0].Structured.Status != structured.Valid {
		t.Fatalf("slot response: %+v", slots)
	}
	second := put(saved.Content+"\n", slots.Slots[0].OutputHash)
	if second.Code != http.StatusOK {
		t.Fatal(second.Body.String())
	}
	if stale := put("stale editor", saved.Hash); stale.Code != http.StatusConflict {
		t.Fatal(stale.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/doc?kind=package&package=P01", nil)
	req.SetPathValue("id", project.ID)
	out = httptest.NewRecorder()
	s.handleGetDoc(out, req)
	if err := json.Unmarshal(out.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Structured.Status != structured.Valid || saved.Structured.Blocks == nil {
		t.Fatal("read API omitted package blocks")
	}
}
