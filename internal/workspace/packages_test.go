package workspace

import (
	"buildflow/internal/structured"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packageTestService(t *testing.T) (*Service, string) {
	t.Helper()
	s := NewService()
	if _, err := s.EnsureWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	project, err := s.CreateProject("Package storage", "")
	if err != nil {
		t.Fatal(err)
	}
	return s, project.ID
}

func packageTestWrite(t *testing.T, s *Service, project, kind, stage, pkg, content string) Doc {
	t.Helper()
	doc, err := s.WriteDoc(WriteRequest{Project: project, Kind: kind, Stage: stage, Package: pkg, Content: content, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func packageCombined(t *testing.T) string {
	t.Helper()
	docs := projectionTestDocuments(t)
	return strings.Replace(docs[projectionDocKey(KindOutput, "step-04", "")].Content, "# NARRATIVE_SENTINEL", "Introduction stays combined.\n\n## P01 - Save documents\n\nPackage body.\n\n## Delivery Overview\n\nStage overview.\n\n## P99 - Overview example", 1)
}

func TestPackageSplitSeparatesFooterAndOverview(t *testing.T) {
	s, project := packageTestService(t)
	docs := projectionTestDocuments(t)
	design := docs[projectionDocKey(KindOutput, "step-03", "")]
	packageTestWrite(t, s, project, design.Kind, design.Stage, "", design.Content)
	source := packageCombined(t)
	combined := packageTestWrite(t, s, project, KindOutput, "step-04", "", source)
	preview, err := s.DetectPackages(project, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Packages) != 1 || preview.Structured.Status != structured.Valid {
		t.Fatalf("preview: %+v", preview)
	}
	if !strings.Contains(preview.Overview, "P99") || !strings.Contains(preview.Unassigned, "Introduction") || !strings.Contains(preview.Footer, "buildflow:view:start") {
		t.Fatalf("missing stage content: %+v", preview)
	}
	p := preview.Packages[0]
	if strings.Contains(p.Content, "Delivery Overview") || strings.Contains(p.Content, "P99") || strings.Contains(p.Content, `"scope": "stage"`) {
		t.Fatal(p.Content)
	}
	if p.Structured.Status != structured.Valid || p.Structured.Artifact.Scope != "package" || *p.Structured.Artifact.PackageID != "P01" {
		t.Fatalf("package projection: %+v", p.Structured)
	}
	if p.Projection == nil || p.Projection.Hash != combined.Hash || len(p.Projection.RecordHashes) == 0 {
		t.Fatal("missing provenance")
	}
	result, err := s.ImportPackages(project, ImportRequest{Packages: preview.Packages, Accept: []string{"P01"}})
	if err != nil || len(result.Created) != 1 {
		t.Fatalf("import: %+v %v", result, err)
	}
	doc, _ := s.ReadDoc(KindPackage, "", "P01", project)
	if parsed := s.InspectDocument(project, doc); parsed.Status != structured.Valid {
		t.Fatalf("saved document: %+v", parsed)
	}
	// Editing a slot never changes the saved combined answer.
	packageTestWrite(t, s, project, KindPackage, "", "P01", strings.Replace(doc.Content, "Package body.", "Slot edit.", 1))
	after, _ := s.ReadDoc(KindOutput, "step-04", "", project)
	if after.Hash != combined.Hash || after.Content != source {
		t.Fatal("slot rewrote combined source")
	}
	slots, err := s.ListStep4Slots(project)
	if err != nil || slots[0].OutputHash == "" || slots[0].OutputHash == doc.Hash || slots[0].Structured.Status != structured.Valid {
		t.Fatalf("slots: %+v %v", slots, err)
	}
}

func TestPackageImportUsesPreviewHashes(t *testing.T) {
	for _, change := range []string{"modified", "created", "deleted", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			s, project := packageTestService(t)
			if change != "created" {
				packageTestWrite(t, s, project, KindPackage, "", "P01", "original")
			}
			preview, err := s.DetectPackages(project, "## P01 - Save documents\nReplacement\n")
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "created", "modified":
				packageTestWrite(t, s, project, KindPackage, "", "P01", "newer edits")
			case "deleted":
				doc, _ := s.ReadDoc(KindPackage, "", "P01", project)
				if err := os.Remove(doc.AbsPath); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := s.ReadDoc(KindPackage, "", "P01", project)
			result, err := s.ImportPackages(project, ImportRequest{Packages: preview.Packages, Accept: []string{"P01"}, Overwrite: []string{"P01"}})
			if err != nil {
				t.Fatal(err)
			}
			after, _ := s.ReadDoc(KindPackage, "", "P01", project)
			if change == "unchanged" {
				if len(result.Overwritten) != 1 || after.Hash == before.Hash {
					t.Fatalf("unchanged preview refused: %+v", result)
				}
			} else if len(result.Refused) != 1 || after.Hash != before.Hash {
				t.Fatalf("stale preview replaced live file: %+v", result)
			}
		})
	}
}

func TestPackageFallbackAndHeadingMatching(t *testing.T) {
	s, project := packageTestService(t)
	for _, heading := range []string{"## P01 - Save documents", "**Work Package P01** - Save documents", "P01: Save documents"} {
		result, err := s.DetectPackages(project, heading+"\nBody\n## Delivery Overview\nOverview\n## BuildFlow Structured View\n<!-- buildflow:view:start -->\n```json\n{broken}\n```\n<!-- buildflow:view:end -->\n")
		if err != nil || len(result.Packages) != 1 || len(result.Warnings) == 0 {
			t.Fatalf("fallback: %+v %v", result, err)
		}
		if strings.Contains(result.Packages[0].Content, "Overview") || strings.Contains(result.Packages[0].Content, "buildflow:") {
			t.Fatal("stage content leaked")
		}
	}
	result, err := s.DetectPackages(project, strings.Replace(packageCombined(t), "P01 - Save documents", "P01 - Wrong title", 1))
	if err != nil || len(result.Packages) != 0 || !strings.Contains(result.Unassigned, "Wrong title") {
		t.Fatalf("mismatch: %+v %v", result, err)
	}
	// Nested fences cannot turn examples into package boundaries.
	hits := findPackageHeadings("````markdown\n```go\n## P99 - Example\n```\n````\n## P01 - Actual\n")
	if len(hits) != 1 || hits[0].id != "P01" {
		t.Fatal(hits)
	}
}

func TestPackageReadValidationAndArtifactStatuses(t *testing.T) {
	s, project := packageTestService(t)
	docs := projectionTestDocuments(t)
	for _, doc := range docs {
		packageTestWrite(t, s, project, doc.Kind, doc.Stage, doc.Package, doc.Content)
	}
	status := func(want string) {
		t.Helper()
		list, err := s.ListPackages(project)
		if err != nil || len(list) != 1 || list[0].Status != want {
			t.Fatalf("want %s, got %+v %v", want, list, err)
		}
	}
	status("ready")
	packageTestWrite(t, s, project, KindPackageSnapshot, "step-05", "P01", "Generator prompt")
	status("prompt-ready")
	packageTestWrite(t, s, project, KindPackageOutput, "step-05", "P01", "Implementation complete. Everything is done.")
	status("output-saved")
	raw, err := os.ReadFile(filepath.Join("..", "structured", "testdata", "step-05.md"))
	if err != nil {
		t.Fatal(err)
	}
	packageTestWrite(t, s, project, KindPackageOutput, "step-05", "P01", string(raw))
	status("prompt-generated")
	// A ready claim with invalid references/coverage is never prompt-generated.
	packageTestWrite(t, s, project, KindPackageOutput, "step-05", "P01", strings.Replace(string(raw), "FR-1", "FR-unknown", 1))
	status("output-saved")
	for _, mutate := range []func(map[string]any){
		func(p map[string]any) { p["id"] = "P02" },
		func(p map[string]any) { p["module_ids"] = []string{"M-unknown"} },
		func(p map[string]any) { p["acceptance"] = []any{}; p["source_acceptance_ids"] = []any{} },
	} {
		envelope := projectionFixture(t, "step-04-package")
		mutate(projectionObject(projectionArray(projectionObject(envelope["blocks"])["packages"])[0]))
		encoded, _ := json.Marshal(envelope)
		doc := Doc{Kind: KindPackage, Package: "P01", Content: "## BuildFlow Structured View\n<!-- buildflow:view:start -->\n```json\n" + string(encoded) + "\n```\n<!-- buildflow:view:end -->\n"}
		parsed := s.InspectDocument(project, doc)
		if parsed.Status != structured.Invalid || len(parsed.Diagnostics) == 0 {
			t.Fatalf("invalid package accepted: %+v", parsed)
		}
	}
}

func TestPackageFooterFiltersApplicableCommonAndIntegrationRecords(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	design := structured.Parse(docs[projectionDocKey(KindOutput, "step-03", "")].Content, structured.Target{Stage: "step-03", Scope: "stage"})
	source := structured.Parse(docs[projectionDocKey(KindOutput, "step-04", "")].Content, structured.Target{Stage: "step-04", Scope: "stage", References: []*structured.Envelope{design.Artifact}})
	if source.Status != structured.Valid {
		t.Fatal(source.Diagnostics)
	}
	b := source.Blocks.(*structured.WorkPackageBlocks)
	b.Assumptions = []structured.Assumption{
		{ID: "AS-global", Statement: "Global", ConsequenceIfWrong: "Review", Validation: "Check", AppliesTo: structured.Applicability{Scope: "global"}},
		{ID: "AS-component", Statement: "Component", ConsequenceIfWrong: "Review", Validation: "Check", AppliesTo: structured.Applicability{Scope: "linked", ComponentIDs: []string{"CMP-1"}}},
		{ID: "AS-contract", Statement: "Shared declaration", ConsequenceIfWrong: "Review", Validation: "Check", AppliesTo: structured.Applicability{Scope: "linked", ContractIDs: []string{"C-3"}}},
		{ID: "AS-unrelated", Statement: "Unrelated", ConsequenceIfWrong: "Review", Validation: "Check", AppliesTo: structured.Applicability{Scope: "linked", ModuleIDs: []string{"M-3"}}},
	}
	for i := range b.Assumptions {
		app := &b.Assumptions[i].AppliesTo
		if app.RequirementIDs == nil {
			app.RequirementIDs = []string{}
		}
		if app.ComponentIDs == nil {
			app.ComponentIDs = []string{}
		}
		if app.ModuleIDs == nil {
			app.ModuleIDs = []string{}
		}
		if app.ContractIDs == nil {
			app.ContractIDs = []string{}
		}
	}
	footer, provenance, err := projectPackageFooter(source.Artifact, "P01", "Save documents", "outputs/step-04.md", "source-hash", design.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{"AS-global", "AS-component", "AS-contract", "Check the shared boundary"} {
		if !strings.Contains(footer, wanted) {
			t.Fatalf("missing %s", wanted)
		}
	}
	for _, excluded := range []string{"AS-unrelated", "UNRELATED_INTEGRATION", "UNRELATED_PACKAGE_GOAL", "SRC-1"} {
		if strings.Contains(footer, excluded) {
			t.Fatalf("unrelated record %s", excluded)
		}
	}
	if len(provenance.SelectedPointers) != 5 {
		t.Fatalf("unexpected selections: %+v", provenance)
	}
	// External prerequisite/integration package IDs remain intact and resolvable
	// when the combined catalogue is supplied, without copying other packages.
	projected := structured.Parse(footer, structured.Target{Stage: "step-04", Scope: "package", PackageID: "P01", References: []*structured.Envelope{design.Artifact, source.Artifact}})
	if projected.Status != structured.Valid {
		t.Fatal(projected.Diagnostics)
	}
}

func TestPackageSlotsAllocateAfterNotesAndReturnTheirHashes(t *testing.T) {
	s, project := packageTestService(t)
	first := packageTestWrite(t, s, project, KindStep4Input, "step-04", "P01", "Notes one")
	third := packageTestWrite(t, s, project, KindStep4Input, "step-04", "P03", "Notes three")
	slots, err := s.ListStep4Slots(project)
	if err != nil || len(slots) != 3 {
		t.Fatalf("slots: %+v %v", slots, err)
	}
	if slots[0].InputHash != first.Hash || slots[1].InputHash != third.Hash || slots[2].ID != "P04" || slots[2].InputHash != "" {
		t.Fatal(slots)
	}
}
