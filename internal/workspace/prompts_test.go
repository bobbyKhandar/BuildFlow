package workspace

import (
	"reflect"
	"strings"
	"testing"
)

func TestStarterPlaceholdersAndNoteWiring(t *testing.T) {
	s := NewService()
	if _, err := s.EnsureWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	project, err := s.CreateProject("Template wiring", "")
	if err != nil {
		t.Fatal(err)
	}
	write := func(kind, stage, pkg, content string) {
		t.Helper()
		if _, err := s.WriteDoc(WriteRequest{Kind: kind, Stage: stage, Package: pkg, Project: project.ID, Content: content, Force: true}); err != nil {
			t.Fatal(err)
		}
	}
	// Save upstream stages before the package so the freshness check does not
	// depend on Go map iteration order.
	documents := projectionTestDocuments(t)
	for _, stage := range StageIDs() {
		if doc, ok := documents[projectionDocKey(KindOutput, stage, "")]; ok {
			write(doc.Kind, doc.Stage, doc.Package, doc.Content)
		}
	}
	packageDoc := documents[projectionDocKey(KindPackage, "step-04", "P01")]
	write(packageDoc.Kind, packageDoc.Stage, packageDoc.Package, packageDoc.Content)
	for _, stage := range StageIDs() {
		write(KindInput, stage, "", "CURRENT_NOTES_"+stage+"\n<!-- buildflow:auto:step-01 -->\nLEGACY_COPIED_ANSWER\n<!-- /buildflow:auto:step-01 -->")
	}
	write(KindStep4Input, "step-04", "P01", "SLOT_ONE\n<!-- buildflow:auto:step-03 -->\nLEGACY_SLOT_COPY")
	write(KindStep4Input, "step-04", "P02", "SLOT_TWO")
	write(KindPackageInput, "step-05", "P01", "SESSION_ONE {{user.project_name}}\n<!-- buildflow:auto:step-04 -->\nLEGACY_PACKAGE_COPY")
	write(KindPackageInput, "step-05", "P02", "SESSION_TWO")
	expected := map[string][]string{
		"step-01":   words("user.project_name user.use_case contract.schema"),
		"step-02":   words("user.project_name handoff user.notes contract.schema"),
		"step-02-5": words("user.project_name handoff user.notes contract.schema"),
		"step-03":   words("user.project_name handoff user.notes contract.schema"),
		"step-04":   words("user.project_name user.target handoff user.notes user.package_notes contract.schema"),
		"step-05":   words("user.project_name user.target handoff user.package_notes user.notes contract.schema"),
	}
	for _, stage := range StageIDs() {
		t.Run(stage, func(t *testing.T) {
			meta, body := SplitFrontMatter(StarterTemplate(stage))
			declared := strings.Trim(frontMatterValue(meta, "assembles"), "[]")
			var tokens []string
			for _, match := range tokenRe.FindAllStringSubmatch(body, -1) {
				tokens = append(tokens, match[1])
			}
			if !reflect.DeepEqual(tokens, expected[stage]) || declared != strings.Join(tokens, ", ") {
				t.Fatalf("assembles or usage mismatch: %s / %v", declared, tokens)
			}
			packages := []string{""}
			if stage == "step-04" {
				packages = append(packages, "P01")
			}
			if stage == "step-05" {
				packages = []string{"P01"}
			}
			for _, pkg := range packages {
				out, err := s.AssemblePrompt(project.ID, stage, pkg)
				if err != nil {
					t.Fatal(err)
				}
				if stage != "step-01" {
					requireProjectionReady(t, *out.Projection)
				}
				for _, token := range tokens {
					if strings.Contains(out.Content, "{{"+token+"}}") && !(stage == "step-05" && token == "user.project_name") {
						t.Fatalf("unresolved token %s", token)
					}
				}
				for _, warning := range out.Warnings {
					if warning.Label == warning.Token || strings.Contains(warning.Reason, "bypasses") {
						t.Fatalf("unexpected token warning: %+v", warning)
					}
				}
				for _, excluded := range []string{"LEGACY_COPIED_ANSWER", "LEGACY_SLOT_COPY", "LEGACY_PACKAGE_COPY", "NARRATIVE_SENTINEL", "SESSION_TWO"} {
					if strings.Contains(out.Content, excluded) {
						t.Fatalf("leaked %s", excluded)
					}
				}
				if !strings.Contains(out.Content, project.Name) || !strings.Contains(out.Content, `"$schema"`) {
					t.Fatal("missing project/schema")
				}
				if stage == "step-04" || stage == "step-05" {
					if !strings.Contains(out.Content, "SLOT_ONE") {
						t.Fatal("missing selected Stage 4 notes")
					}
					if strings.Contains(out.Content, "SLOT_TWO") != (stage == "step-04" && pkg == "") {
						t.Fatal("wrong package note scope")
					}
				}
				if stage == "step-05" {
					if !strings.Contains(out.Content, "SESSION_ONE {{user.project_name}}") || strings.Contains(out.Content, "CURRENT_NOTES_step-05") {
						t.Fatal("session notes scope or recursive substitution")
					}
					if !strings.Contains(out.Content, "Do not implement the package") {
						t.Fatal("Stage 5 lost prompt-only rule")
					}
				} else if !strings.Contains(out.Content, "CURRENT_NOTES_"+stage) {
					t.Fatal("missing stage notes")
				}
				if stage == "step-04" && pkg != "" && !strings.Contains(out.Content, `"module_ids":["M-1"]`) {
					t.Fatal("missing established module mapping")
				}
			}
		})
	}
}

func TestCustomWholeOutputTokensWithHandoff(t *testing.T) {
	for _, template := range []string{"{{output_step_01}}\n{{previous_output}}", "{{handoff}}\n{{output_step_01}}\n{{previous_output}}"} {
		t.Run(template, func(t *testing.T) {
			s := NewService()
			if _, err := s.EnsureWorkspace(t.TempDir()); err != nil {
				t.Fatal(err)
			}
			project, err := s.CreateProject("Custom template", "")
			if err != nil {
				t.Fatal(err)
			}
			content := "WHOLE_ANSWER " + strings.Repeat("x", 130000)
			if _, err := s.WriteDoc(WriteRequest{Kind: KindOutput, Stage: "step-01", Project: project.ID, Content: content, Force: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.SaveTemplate("step-02", "local", project.ID, template, "", true); err != nil {
				t.Fatal(err)
			}
			out, err := s.AssemblePrompt(project.ID, "step-02", "")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(out.Content, content+"\n"+content+"\n") {
				t.Fatal("custom whole outputs lost or trimmed")
			}
			bypass := map[string]bool{}
			for _, warning := range out.Warnings {
				if strings.Contains(warning.Reason, "bypasses") {
					bypass[warning.Token] = true
				}
			}
			if !bypass["output_step_01"] || !bypass["previous_output"] || sizeWarning(out.Size) == nil {
				t.Fatal("missing compatibility/size warnings")
			}
		})
	}
}
