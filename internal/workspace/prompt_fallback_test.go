package workspace

import (
	"strings"
	"testing"

	"buildflow/internal/structured"
)

func fallbackTestBuilder(content string, rows []ProjectionField) *projectionBuilder {
	b := &projectionBuilder{target: ProjectionTarget{Stage: "step-02"}, out: PromptProjection{Sources: []AssemblySource{{Label: "step-01", Path: "source.md"}}}}
	d := &projectionDocument{doc: Doc{Path: "source.md", Content: content, Exists: true}, parsed: structured.Parse(content, structured.Target{Stage: "step-01", Scope: "stage"})}
	b.legacyFallback(d, rows)
	b.legacyFallback(d, rows)
	return b
}

func TestLegacySectionRegistryFencesBoundariesAndDedup(t *testing.T) {
	content := "# Intro\r\nnot selected\r\n````markdown\r\n## Goals and non-goals\r\nFAKE\r\n```\r\n````\r\n## 2. GOALS AND NON-GOALS\r\nVERBATIM 雪 {{literal}}\r\n### Detail\r\n~~~\r\n## example heading\r\n~~~\r\n## Other\r\nNOT_SELECTED\r\n"
	rows := []ProjectionField{researchProjection()[1], researchProjection()[2]}
	b := fallbackTestBuilder(content, rows)
	if len(b.out.Legacy) != 1 || b.out.Sources[0].Mode != "markdown_section" {
		t.Fatalf("unexpected fallback: %+v", b.out)
	}
	rendered := b.out.Legacy[0].Rendered
	if strings.Count(rendered, "VERBATIM") != 1 || strings.Contains(rendered, "FAKE") || strings.Contains(rendered, "NOT_SELECTED") || !strings.Contains(rendered, "field selection unavailable") || !strings.Contains(rendered, "{{literal}}\r\n") {
		t.Fatal(rendered)
	}
	bounds := b.out.Sources[0].FallbackBoundaries
	if len(bounds) != 1 || !strings.Contains(rendered, content[bounds[0].Start:bounds[0].End]) {
		t.Fatal("lost byte boundary provenance")
	}
}

func TestLegacyWholeDocumentOnceAndMetadataPreservation(t *testing.T) {
	for _, footer := range []string{"", "\n## BuildFlow Structured View\n\n<!-- buildflow:view:start -->\n```json\nBROKEN\n```\n<!-- buildflow:view:end -->\n", "\n<!-- buildflow:view:start -->\nDAMAGED"} {
		content := "---\nstage: step-01\n---\n## Goals and non-goals\nSENTINEL\n" + footer
		b := fallbackTestBuilder(content, researchProjection())
		if len(b.out.Legacy) != 1 || b.out.Sources[0].Mode != "markdown_document" {
			t.Fatal("not deduplicated document fallback")
		}
		rendered := b.out.Legacy[0].Rendered
		if strings.Count(rendered, "SENTINEL") != 1 || strings.Contains(rendered, "stage: step-01") {
			t.Fatal(rendered)
		}
		if strings.Contains(footer, "end -->") && strings.Contains(rendered, "BROKEN") {
			t.Fatal("bounded metadata retained")
		}
		if strings.Contains(footer, "DAMAGED") && !strings.Contains(rendered, "DAMAGED") {
			t.Fatal("damaged metadata lost")
		}
	}
}

func TestLegacyProjectionAndRecovery(t *testing.T) {
	docs := projectionTestDocuments(t)
	key := projectionDocKey(KindOutput, "step-03", "")
	d := docs[key]
	d.Content = "## Module breakdown\nLEGACY MODULE\n## Design changes\nAMENDED COMMAND\n"
	d.Hash = hashBytes([]byte(d.Content))
	docs[key] = d
	p := runProjection(t, docs, ProjectionTarget{Stage: "step-04"})
	requireProjectionReady(t, p)
	rendered, _ := RenderPromptProjection(p)
	if strings.Count(rendered, "LEGACY MODULE") != 1 || !strings.Contains(rendered, "AMENDED COMMAND") || !strings.Contains(rendered, "legacy context recovery") {
		t.Fatal(rendered)
	}
	if len(p.Sources) != 2 || p.Sources[0].Mode != "markdown_document" || p.Sources[1].Mode != "json_projection" {
		t.Fatalf("bad recovery provenance: %+v", p.Sources)
	}
	// An explicit effective section prevents baseline recovery.
	d.Content += "## Effective implementation context\nEFFECTIVE\n"
	docs[key] = d
	p = runProjection(t, docs, ProjectionTarget{Stage: "step-04"})
	if len(p.Sources) != 1 {
		t.Fatal("recovered conflicting baseline")
	}
	// An explicit valid null never recovers from Markdown or Stage 2.5.
	docs = projectionTestDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		projectionObject(e["blocks"])["implementation_context"] = nil
		e["state"] = "blocked"
		projectionObject(e["blocks"])["open_questions"] = []any{map[string]any{"id": "Q-context", "question": "Which context?", "blocking": true, "why_it_matters": "Essential", "cheapest_check": "Review", "recommended_default": nil, "resolve_by": nil, "applies_to": map[string]any{"scope": "global", "requirement_ids": []any{}, "component_ids": []any{}, "module_ids": []any{}, "contract_ids": []any{}}}}
	})
	p = runProjection(t, docs, ProjectionTarget{Stage: "step-04"})
	if p.State != "blocked" || len(p.Legacy) != 0 || len(p.Sources) != 1 {
		t.Fatal("explicit null triggered legacy recovery")
	}
}

func TestWrongPackageMetadataUsesOnlySelectedSavedDocument(t *testing.T) {
	docs := projectionTestDocuments(t)
	// Mismatched footer belongs to P02, but is saved in the selected P01 slot.
	key := projectionDocKey(KindPackage, "step-04", "P01")
	e := projectionFixture(t, "step-04-package")
	e["package_id"] = "P02"
	setProjectionDoc(t, docs, KindPackage, "step-04", "P01", e)
	d := docs[key]
	d.Content = "SELECTED_SLOT\n" + d.Content
	docs[key] = d
	other := d
	other.Path = "packages/P02.md"
	other.Content = "OTHER_PACKAGE_MUST_NOT_LEAK"
	docs[projectionDocKey(KindPackage, "step-04", "P02")] = other
	p := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	if p.State != "blocked" {
		t.Fatal("wrong package became ready")
	}
	rendered, _ := RenderPromptProjection(p)
	if !strings.Contains(rendered, "SELECTED_SLOT") || strings.Contains(rendered, "OTHER_PACKAGE_MUST_NOT_LEAK") || strings.Contains(rendered, "NARRATIVE_SENTINEL\n\n## BuildFlow Structured View") {
		t.Fatal(rendered)
	}
	if len(p.Legacy) != 1 || !strings.Contains(p.Legacy[0].Rendered, "unfiltered legacy context") {
		t.Fatal("unlabelled package fallback")
	}
}
