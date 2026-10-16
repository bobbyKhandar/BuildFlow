package workspace

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompactProjectionLosslessAndDeterministic(t *testing.T) {
	docs := projectionTestDocuments(t)
	exact := "line one\r\n\t\"quoted\" \\ slash 雪🙂 <tag> &\n```go\n{{input}}\n```\n"
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		blocks := projectionObject(e["blocks"])
		projectionObject(projectionArray(blocks["contracts"])[0])["declaration"] = exact
		ctx := projectionObject(blocks["implementation_context"])
		projectionObject(projectionObject(ctx["technology"])["commands"])["build"] = exact
		system := projectionObject(ctx["system_context"])
		projectionObject(projectionArray(system["invariants"])[0])["rule"] = exact
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-04"})
	requireProjectionReady(t, out)
	// Stage 4 intentionally drops pattern examples; Stage 5's profile carries them.
	payload, err := RenderPromptProjection(out)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		again, _ := RenderPromptProjection(out)
		if again != payload {
			t.Fatal("nondeterministic rendering")
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(renderedProjectionJSON(t, payload)), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, out.Data) {
		t.Fatal("rendering changed decoded data")
	}
	ctx := projectionObject(decoded["implementation_context"])
	if projectionObject(projectionArray(decoded["contracts"])[0])["declaration"] != exact || projectionObject(projectionObject(ctx["technology"])["commands"])["build"] != exact || projectionObject(projectionArray(projectionObject(ctx["system_context"])["invariants"])[0])["rule"] != exact {
		t.Fatal("verbatim value changed")
	}
	if strings.Contains(renderedProjectionJSON(t, payload), "\n") || !strings.Contains(payload, "雪🙂 <tag> &") {
		t.Fatal("JSON is not compact/readable")
	}
	if substitute("{{handoff}}", map[string]string{"handoff": payload, "input": "REPLACED"}, nil) != payload {
		t.Fatal("recursive substitution")
	}
	// Pattern examples obey the same decoded preservation, independently of the
	// consumer's intentional field exclusions.
	pattern := map[string]any{"example": exact}
	rendered, _ := compactJSON(pattern)
	var got map[string]any
	_ = json.Unmarshal([]byte(rendered), &got)
	if !reflect.DeepEqual(pattern, got) {
		t.Fatal("example changed")
	}
}

func TestProjectedCompactFixturesSmallerThanV2Baseline(t *testing.T) {
	for _, stage := range words("step-02 step-02-5 step-03 step-04 step-05") {
		t.Run(stage, func(t *testing.T) {
			docs := projectionTestDocuments(t)
			target := ProjectionTarget{Stage: stage}
			if stage == "step-05" {
				target.PackageID = "P01"
			}
			p := runProjection(t, docs, target)
			requireProjectionReady(t, p)
			compact, _ := RenderPromptProjection(p)
			// V2 supplies complete blocks in indented JSON, with no field projection.
			baseline := map[string]any{}
			for _, source := range p.Sources {
				if !source.Active {
					continue
				}
				for _, doc := range docs {
					if doc.Path == source.Path {
						name := doc.Stage
						if doc.Kind == KindPackage {
							name = "step-04-package"
						}
						baseline[source.Path] = projectionFixture(t, name)["blocks"]
					}
				}
			}
			pretty, _ := json.MarshalIndent(baseline, "", "  ")
			if utf8.RuneCountInString(compact) >= utf8.RuneCountInString(string(pretty)) {
				t.Fatalf("compact=%d baseline=%d", utf8.RuneCountInString(compact), utf8.RuneCountInString(string(pretty)))
			}
			t.Logf("serialized characters: projected/minified %d; v2 whole-block/pretty %d", utf8.RuneCountInString(compact), utf8.RuneCountInString(string(pretty)))
		})
	}
}

func TestPromptSizeThresholdsAndManualEdits(t *testing.T) {
	for _, stage := range words("step-03 step-04 step-05") {
		limit := 32000
		if stage == "step-04" {
			limit = 48000
		}
		for _, n := range []int{limit*4 - 4, limit * 4, limit*4 + 4} {
			text := strings.Repeat("雪", n)
			size := EstimatePromptSize(text, stage)
			if size.Chars != n || size.EstimatedTokens != n/4 || size.WarningThreshold != limit || (sizeWarning(size) != nil) != (n >= limit*4) {
				t.Fatalf("bad estimate: %+v", size)
			}
		}
	}
}

func TestAssemblySizeContributions(t *testing.T) {
	p := PromptProjection{State: "ready", Data: map[string]any{"declaration": strings.Repeat("x", 140000)}, Legacy: []LegacyPromptSource{{Path: "legacy", Rendered: "LEGACY 雪"}}}
	payload, _ := RenderPromptProjection(p)
	body := "fixed {{handoff}}\n{{input}} {{handoff}}"
	values := map[string]string{"handoff": payload, "input": "my notes"}
	text := substitute(body, values, nil) + "\n"
	size := assemblySize(text, "step-03", body, values, &p)
	total := 0
	for _, c := range size.Contributions {
		if c.Chars < 0 {
			t.Fatal("negative contribution")
		}
		total += c.Chars
		if c.EstimatedTokens != estimatedTokens(c.Chars) {
			t.Fatal("bad contribution estimate")
		}
	}
	if total != size.Chars || sizeWarning(size) == nil || !strings.Contains(text, strings.Repeat("x", 140000)) {
		t.Fatal("lost required content or size accounting")
	}
	if size.Contributions[1].Chars == 0 || size.Contributions[2].Chars != len("my notes") || size.Contributions[3].Chars != 2*utf8.RuneCountInString("\n\nLEGACY 雪") {
		t.Fatalf("wrong breakdown: %+v", size)
	}
}

func TestAssemblyLargeDeclarationWarnsWithoutTruncation(t *testing.T) {
	s := NewService()
	if _, err := s.EnsureWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	project, err := s.CreateProject("Size reporting", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveTemplate("step-04", "local", project.ID, "{{handoff}}\n{{input}}", "", true); err != nil {
		t.Fatal(err)
	}
	docs := projectionTestDocuments(t)
	exact := strings.Repeat("雪 <tag> {{literal}}\n", 12000) + "END_REQUIRED_DECLARATION"
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		projectionObject(projectionArray(projectionObject(e["blocks"])["contracts"])[0])["declaration"] = exact
	})
	d := docs[projectionDocKey(KindOutput, "step-03", "")]
	if _, err := s.WriteDoc(WriteRequest{Kind: KindOutput, Stage: "step-03", Project: project.ID, Content: d.Content, Force: true}); err != nil {
		t.Fatal(err)
	}
	assembled, err := s.AssemblePrompt(project.ID, "step-04", "")
	if err != nil {
		t.Fatal(err)
	}
	if assembled.Size.WarningThreshold != 48000 || assembled.EstimatedTokens != estimatedTokens(assembled.CharCount) || assembled.CharCount != utf8.RuneCountInString(assembled.Content) {
		t.Fatalf("bad assembly estimate: %+v", assembled.Size)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(renderedProjectionJSON(t, assembled.Content)), &data); err != nil {
		t.Fatal(err)
	}
	if projectionObject(projectionArray(data["contracts"])[0])["declaration"] != exact {
		t.Fatal("large declaration was changed or truncated")
	}
	warning := false
	for _, w := range assembled.Warnings {
		if w.Token == "prompt_size" {
			warning = true
		}
	}
	if !warning {
		t.Fatal("assembly omitted size warning")
	}
	for _, source := range assembled.Projection.Sources {
		if source.Chars == 0 || source.EstimatedTokens == 0 || source.Hash == "" || source.ProjectionVersion == "" {
			t.Fatalf("missing size/provenance: %+v", source)
		}
	}
}

// The rendered handoff contains a single JSON line inside an adaptive fence.
func renderedProjectionJSON(t *testing.T, rendered string) string {
	t.Helper()
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, "json") && strings.HasPrefix(line, "```") && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	t.Fatal("missing labelled JSON handoff")
	return ""
}

func TestProjectionFenceOutgrowsEmbeddedExamples(t *testing.T) {
	p := PromptProjection{Stage: "step-05", PackageID: "P01", State: "ready", Data: map[string]any{"example": "\n" + strings.Repeat("`", 12) + "\n"}}
	rendered, err := RenderPromptProjection(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rendered, "## Selected handoff — step-05 / P01\n"+strings.Repeat("`", 13)+"json\n") {
		t.Fatal("unsafe or unlabelled JSON boundary")
	}
}
