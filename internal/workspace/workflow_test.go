package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildflow/internal/structured"
)

// Exercise the shipped answer fixtures through real workspace saves, template
// updates, import and assembly, rather than only the in-memory projection API.
func TestStructuredWorkflowFixtures(t *testing.T) {
	s, project := packageTestService(t)
	readFixture := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join("..", "structured", "testdata", name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	read := func(kind, stage, pkg string) Doc {
		t.Helper()
		d, err := s.ReadDoc(kind, stage, pkg, project)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	var combined Doc
	for _, stage := range StageIDs() {
		// Existing workspaces adopt templates by ordinary editing and saving.
		active, err := s.GetTemplate(stage, "global", project)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveTemplate(stage, "global", project, StarterTemplate(stage)+"\nWorkflow review instruction.\n", active.Hash, false); err != nil {
			t.Fatal(err)
		}
		kind, pkg := KindOutput, ""
		if stage == "step-05" {
			kind, pkg = KindPackageOutput, "P01"
		}
		assembled, err := s.AssemblePrompt(project, stage, pkg)
		if err != nil {
			t.Fatal(err)
		}
		if stage != "step-01" {
			requireProjectionReady(t, *assembled.Projection)
		}
		if !strings.Contains(assembled.Content, "Workflow review instruction.") {
			t.Fatal("saved active template was not used")
		}
		exact := assembled.Content + "\r\nReviewer note: café 🛠 {{literal}}\r\n"
		if _, err := s.SaveSnapshot(project, stage, pkg, exact); err != nil {
			t.Fatal(err)
		}
		snapshotKind := KindSnapshot
		if pkg != "" {
			snapshotKind = KindPackageSnapshot
		}
		if _, got, found := strings.Cut(read(snapshotKind, stage, pkg).Content, "\n---\n\n"); !found || got != exact {
			t.Fatalf("%s snapshot changed exact prompt bytes", stage)
		}
		content := readFixture(stage)
		saved := packageTestWrite(t, s, project, kind, stage, pkg, content)
		if parsed := s.InspectDocument(project, saved); parsed.Status != structured.Valid {
			t.Fatalf("%s: %+v", stage, parsed.Diagnostics)
		}
		if got := read(kind, stage, pkg); got.Content != content || got.Hash != saved.Hash {
			t.Fatalf("%s source changed", stage)
		}
		if stage != "step-04" {
			continue
		}
		combined = saved
		preview, err := s.DetectPackages(project, read(KindOutput, "step-04", "").Content)
		if err != nil || len(preview.Packages) != 1 {
			t.Fatalf("preview: %+v %v", preview, err)
		}
		result, err := s.ImportPackages(project, ImportRequest{Packages: preview.Packages, Accept: []string{"P01"}})
		if err != nil || len(result.Created) != 1 {
			t.Fatalf("import: %+v %v", result, err)
		}
		individual, err := s.AssemblePrompt(project, stage, "P01")
		if err != nil {
			t.Fatal(err)
		}
		requireProjectionReady(t, *individual.Projection)
		// A slot refinement must not rewrite the original combined answer.
		slot := read(KindPackage, "", "P01")
		if _, err := s.WriteDoc(WriteRequest{Project: project, Kind: KindPackage, Package: "P01", Content: slot.Content + "\n", BaseHash: slot.Hash}); err != nil {
			t.Fatal(err)
		}
	}
	if after := read(KindOutput, "step-04", ""); after.Hash != combined.Hash || after.Content != combined.Content {
		t.Fatal("package processing changed combined source")
	}
	packages, err := s.ListPackages(project)
	if err != nil || len(packages) != 1 || packages[0].Status != "prompt-generated" {
		t.Fatalf("artifact status: %+v %v", packages, err)
	}
	// An external edit wins over a stale ordinary save, without losing either
	// the existing combined source or the bytes written by the external editor.
	slot := read(KindPackage, "", "P01")
	external := slot.Content + "\nExternal editor: café.\n"
	if err := os.WriteFile(slot.AbsPath, []byte(external), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = s.WriteDoc(WriteRequest{Project: project, Kind: KindPackage, Package: "P01", Content: "stale replacement", BaseHash: slot.Hash})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || read(KindPackage, "", "P01").Content != external {
		t.Fatalf("external edit not protected: %v", err)
	}
	// Legacy notes and answers still assemble, without copying new answers or
	// exposing the historical auto-copy tail as user-authored notes.
	legacy, err := s.CreateProject("Legacy workflow", "")
	if err != nil {
		t.Fatal(err)
	}
	notes := "My notes\n<!-- buildflow:auto:step-01 -->\nOLD_COPY"
	noteDoc := packageTestWrite(t, s, legacy.ID, KindInput, "step-02", "", notes)
	packageTestWrite(t, s, legacy.ID, KindOutput, "step-01", "", readFixture("legacy"))
	assembly, err := s.AssemblePrompt(legacy.ID, "step-02", "")
	if err != nil || strings.Contains(assembly.Content, "OLD_COPY") || !strings.Contains(assembly.Content, "My notes") || strings.Count(assembly.Content, "Preserve it exactly.") != 1 {
		t.Fatalf("legacy handoff: %+v %v", assembly, err)
	}
	if after, err := s.ReadDoc(KindInput, "step-02", "", legacy.ID); err != nil || after.Hash != noteDoc.Hash {
		t.Fatal("answer save or assembly changed legacy notes")
	}
}

func TestImportedWorkflowPackageIsolation(t *testing.T) {
	s, project := packageTestService(t)
	docs := isolatedProjectionDocuments(t)
	combinedKey := projectionDocKey(KindOutput, "step-04", "")
	combined := docs[combinedKey]
	combined.Content = strings.Replace(combined.Content, "# NARRATIVE_SENTINEL", "# Combined work packages\n\n## P01 - Save documents\nSelected work.\n\n## P02 - UNRELATED_PACKAGE_TITLE\nUnrelated work.\n\n## P03 - UNRELATED_PACKAGE_TITLE\nOther unrelated work.", 1)
	docs[combinedKey] = combined
	for _, stage := range words("step-01 step-02 step-02-5 step-03 step-04") {
		d := docs[projectionDocKey(KindOutput, stage, "")]
		packageTestWrite(t, s, project, d.Kind, stage, "", d.Content)
	}
	preview, err := s.DetectPackages(project, docs[projectionDocKey(KindOutput, "step-04", "")].Content)
	if err != nil || len(preview.Packages) != 3 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	result, err := s.ImportPackages(project, ImportRequest{Packages: preview.Packages, Accept: []string{"P01", "P02", "P03"}})
	if err != nil || len(result.Created) != 3 {
		t.Fatalf("import: %+v %v", result, err)
	}
	for _, pkg := range words("P01 P02") {
		packageTestWrite(t, s, project, KindPackageInput, "step-05", pkg, "SESSION_"+pkg)
	}
	assembly, err := s.AssemblePrompt(project, "step-05", "P01")
	if err != nil {
		t.Fatal(err)
	}
	requireProjectionReady(t, *assembly.Projection)
	for _, excluded := range words("SESSION_P02 UNRELATED_PACKAGE_GOAL UNRELATED_PACKAGE_NOTES UNRELATED_INTEGRATION") {
		if strings.Contains(assembly.Content, excluded) {
			t.Fatalf("selected package leaked %s", excluded)
		}
	}
	if !strings.Contains(assembly.Content, "SESSION_P01") || !strings.Contains(assembly.Content, "type SharedType struct") {
		t.Fatal("selected session notes or shared contract closure missing")
	}
}
