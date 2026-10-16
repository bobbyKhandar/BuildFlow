package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buildflow/internal/workspace"
)

func TestSavingAnswersLeavesInputsUntouched(t *testing.T) {
	s, _ := startupTestServer(t)
	s.svc = workspace.NewService()
	if _, err := s.svc.EnsureWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	project, err := s.svc.CreateProject("No answer copies", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ kind, stage, pkg, noteKind, noteStage string }{
		{workspace.KindOutput, "step-01", "", workspace.KindInput, "step-02"},
		{workspace.KindPackage, "step-04", "P01", workspace.KindPackageInput, "step-05"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			before, err := s.svc.WriteDoc(workspace.WriteRequest{Kind: tc.noteKind, Stage: tc.noteStage, Package: tc.pkg, Project: project.ID, Content: "USER NOTES\n", Force: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, answer := range []string{"FIRST WHOLE ANSWER", "UPDATED WHOLE ANSWER"} {
				body, err := json.Marshal(putDocRequest{Kind: tc.kind, Stage: tc.stage, Package: tc.pkg, Content: answer, Force: true})
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPut, "/api/projects/"+project.ID+"/doc", bytes.NewReader(body))
				req.SetPathValue("id", project.ID)
				recorder := httptest.NewRecorder()
				s.handlePutDoc(recorder, req)
				if recorder.Code != http.StatusOK {
					t.Fatalf("save: %d %s", recorder.Code, recorder.Body.String())
				}
				after, err := s.svc.ReadDoc(tc.noteKind, tc.noteStage, tc.pkg, project.ID)
				if err != nil {
					t.Fatal(err)
				}
				if after.Content != before.Content || after.Hash != before.Hash {
					t.Fatal("saving an answer modified the next input")
				}
			}
		})
	}
}
