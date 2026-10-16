package structured

import (
	"encoding/json"
	"strings"
	"testing"
)

func linkedFixtures(t *testing.T) []*Envelope {
	t.Helper()
	var refs []*Envelope
	for _, stage := range []string{"step-01", "step-02", "step-02-5", "step-03", "step-04"} {
		r := Parse(fixture(t, stage), Target{Stage: stage, Scope: "stage", References: refs})
		if r.Status != Valid {
			t.Fatalf("%s: %+v", stage, r.Diagnostics)
		}
		refs = append(refs, r.Artifact)
	}
	return refs
}
func TestCompleteLinkedHandoffs(t *testing.T) {
	refs := linkedFixtures(t)
	for _, tt := range []struct{ name, stage, scope, id string }{{"step-04-package", "step-04", "package", "P01"}, {"step-05", "step-05", "package", "P01"}} {
		r := Parse(fixture(t, tt.name), Target{Stage: tt.stage, Scope: tt.scope, PackageID: tt.id, References: refs})
		if r.Status != Valid {
			t.Fatalf("%s: %+v", tt.name, r.Diagnostics)
		}
		for _, d := range r.Diagnostics {
			if d.Code == "reference_unchecked" {
				t.Fatalf("complete fixture left an unresolved join: %+v", d)
			}
		}
	}
}
func TestSchemaAndSemanticFailures(t *testing.T) {
	refs := linkedFixtures(t)
	cases := []struct {
		name, stage, code string
		change            func(map[string]any)
	}{
		{"unknown field", "step-01", "schema", func(b map[string]any) { b["summary"] = "Invented" }},
		{"missing field", "step-01", "schema", func(b map[string]any) { delete(b, "problem") }},
		{"null array", "step-01", "schema", func(b map[string]any) { b["goals"] = nil }},
		{"wrong enum", "step-01", "schema", func(b map[string]any) { b["requirements"].([]any)[0].(map[string]any)["kind"] = "other" }},
		{"duplicate ID", "step-01", "duplicate_id", func(b map[string]any) { a := b["requirements"].([]any); b["requirements"] = append(a, a[0]) }},
		{"dangling evidence", "step-01", "reference", func(b map[string]any) {
			b["requirements"].([]any)[0].(map[string]any)["basis"].(map[string]any)["source_ids"] = []any{"missing"}
		}},
		{"guessed requirement", "step-01", "basis", func(b map[string]any) {
			b["requirements"].([]any)[0].(map[string]any)["basis"].(map[string]any)["confidence"] = "assumption"
		}},
		{"empty linked scope", "step-01", "applicability", func(b map[string]any) {
			b["constraints"].([]any)[0].(map[string]any)["applies_to"].(map[string]any)["scope"] = "linked"
		}},
		{"dangling applicability", "step-01", "reference", func(b map[string]any) {
			b["constraints"].([]any)[0].(map[string]any)["applies_to"].(map[string]any)["requirement_ids"] = []any{"FR-missing"}
		}},
		{"bad weights", "step-01", "weights", func(b map[string]any) {
			b["evaluation_criteria"].([]any)[0].(map[string]any)["weight_percent"] = float64(99)
		}},
		{"empty ready deliverable", "step-01", "schema", func(b map[string]any) { b["requirements"] = []any{} }},
		{"dangling contract", "step-03", "reference", func(b map[string]any) {
			b["contracts"].([]any)[0].(map[string]any)["referenced_contract_ids"] = []any{"missing"}
		}},
		{"wrong owner kind", "step-03", "reference", func(b map[string]any) {
			b["contracts"].([]any)[0].(map[string]any)["owner"] = map[string]any{"kind": "module", "id": "CMP-1"}
		}},
		{"module cycle", "step-03", "dependency", func(b map[string]any) { b["modules"].([]any)[0].(map[string]any)["depends_on"] = []any{"M-1"} }},
		{"coverage missing", "step-03", "schema", func(b map[string]any) { b["requirement_coverage"] = []any{} }},
		{"test ID duplicate", "step-03", "duplicate_id", func(b map[string]any) {
			a := b["test_plan"].([]any)[0].(map[string]any)
			a["tests"] = append(a["tests"].([]any), a["tests"].([]any)[0])
		}},
		{"missing source criterion", "step-04", "acceptance", func(b map[string]any) { b["packages"].([]any)[0].(map[string]any)["source_acceptance_ids"] = []any{} }},
		{"changed criterion", "step-04", "conflict", func(b map[string]any) {
			b["packages"].([]any)[0].(map[string]any)["acceptance"].([]any)[0].(map[string]any)["statement"] = "Weakened"
		}},
		{"unrelated package record", "step-04", "package_scope", func(b map[string]any) {
			b["packages"].([]any)[0].(map[string]any)["tests"].([]any)[0].(map[string]any)["requirement_ids"] = []any{"FR-other"}
		}},
		{"new packaging contract", "step-04", "reference", func(b map[string]any) {
			b["packages"].([]any)[0].(map[string]any)["produced_contract_ids"] = []any{"C-new"}
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			v := fixtureJSON(t, tt.stage)
			tt.change(v["blocks"].(map[string]any))
			r := Parse(encoded(t, v), Target{Stage: tt.stage, Scope: "stage", References: refs})
			if r.Status != Invalid || !hasCode(r, tt.code) {
				t.Fatalf("%s: %+v", r.Status, r.Diagnostics)
			}
		})
	}
}
func TestStage5OnlyFuturePrompt(t *testing.T) {
	refs := linkedFixtures(t)
	target := Target{Stage: "step-05", Scope: "package", PackageID: "P01", References: refs}
	for _, field := range []string{"implemented", "passed", "changed_files", "summary"} {
		v := fixtureJSON(t, "step-05")
		v["blocks"].(map[string]any)[field] = true
		if r := Parse(encoded(t, v), target); r.Status != Invalid || !hasCode(r, "schema") {
			t.Fatalf("accepted implementation report %q", field)
		}
	}
	v := fixtureJSON(t, "step-05")
	v["blocks"].(map[string]any)["coding_prompt"] = nil
	if r := Parse(encoded(t, v), target); r.Status != Invalid {
		t.Fatal("ready prompt may not be null")
	}
	v["state"] = "blocked"
	b := v["blocks"].(map[string]any)
	b["blockers"] = []any{"Resolve the save conflict."}
	b["open_questions"] = []any{map[string]any{"id": "Q-1", "question": "Which save policy?", "why_it_matters": "Avoid data loss", "cheapest_check": "Ask owner", "blocking": true, "recommended_default": nil, "resolve_by": nil, "applies_to": map[string]any{"scope": "global", "requirement_ids": []any{}, "component_ids": []any{}, "module_ids": []any{}, "contract_ids": []any{}}}}
	if r := Parse(encoded(t, v), target); r.Status != Valid {
		t.Fatal(r.Diagnostics)
	}
	if r := Parse(fixture(t, "step-05"), Target{Stage: "step-05", Scope: "package", PackageID: "P02"}); r.Status != Invalid || !hasCode(r, "target") {
		t.Fatal(r)
	}
}
func TestLegacyNeverGuessesLinks(t *testing.T) {
	s := strings.Replace(fixture(t, "step-03"), `"schema_version": 2`, `"schema_version": 1`, 1)
	r := Parse(s, Target{Stage: "step-03", Scope: "stage"})
	if r.Status != UnsupportedVersion || r.Blocks != nil {
		t.Fatal(r)
	}
}

func TestCrossStageIncompleteJoins(t *testing.T) {
	refs := linkedFixtures(t)
	for _, tt := range []struct {
		name, stage, code string
		change            func(map[string]any)
	}{
		{"missing module batch", "step-03", "schema", func(b map[string]any) { b["build_batches"] = []any{} }},
		{"criterion wrong test requirement", "step-03", "acceptance", func(b map[string]any) {
			b["test_plan"].([]any)[0].(map[string]any)["tests"].([]any)[0].(map[string]any)["requirement_ids"] = []any{}
		}},
		{"conflicting copied test", "step-04", "conflict", func(b map[string]any) {
			b["packages"].([]any)[0].(map[string]any)["tests"].([]any)[0].(map[string]any)["assertions"] = []any{"Data may be lost."}
		}},
		{"missing package test", "step-04", "acceptance", func(b map[string]any) { b["packages"].([]any)[0].(map[string]any)["tests"] = []any{} }},
		{"missing required module contract", "step-04", "coverage", func(b map[string]any) {
			p := b["packages"].([]any)[0].(map[string]any)
			p["produced_contract_ids"] = []any{}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := fixtureJSON(t, tt.stage)
			tt.change(object(v["blocks"]))
			r := Parse(encoded(t, v), Target{Stage: tt.stage, Scope: "stage", References: refs})
			if r.Status != Invalid || !hasCode(r, tt.code) {
				t.Fatalf("%s: %+v", r.Status, r.Diagnostics)
			}
		})
	}
}
func TestPackageScopeAllowsKnownExternalPrerequisites(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-04-package")
	p := object(object(v["blocks"])["packages"].([]any)[0])
	p["depends_on"] = []any{"P02"}
	p["prerequisite_reasons"] = []any{map[string]any{"package_id": "P02", "reason": "Shared store prerequisite"}}
	// A supplied combined mapping must establish P02. Without it, the link is
	// explicitly unchecked, preserving external references in package documents.
	without := Parse(encoded(t, v), Target{Stage: "step-04", Scope: "package", PackageID: "P01", References: refs[:4]})
	if without.Status != Valid || !hasCode(without, "reference_unchecked") {
		t.Fatal(without.Diagnostics)
	}
	combined := fixtureJSON(t, "step-04")
	p2 := object(object(combined["blocks"])["packages"].([]any)[0])
	p2["id"] = "P02"
	object(combined["blocks"])["build_batches"] = []any{[]any{"P02"}}
	r := Parse(encoded(t, combined), Target{Stage: "step-04", Scope: "stage", References: refs[:4]})
	if r.Status != Valid {
		t.Fatal(r.Diagnostics)
	}
	refs = append(refs[:4], r.Artifact)
	r = Parse(encoded(t, v), Target{Stage: "step-04", Scope: "package", PackageID: "P01", References: refs})
	if r.Status != Valid || hasCode(r, "reference_unchecked") {
		t.Fatal(r.Diagnostics)
	}
}

func TestEffectiveDesignCannotLoseEarlierRequirements(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-03")
	// Replace every local FR-1 link with FR-2; local joins remain consistent,
	// but the earlier Stage 2 requirement has disappeared without disposition.
	raw, _ := json.Marshal(v)
	_ = json.Unmarshal([]byte(strings.ReplaceAll(string(raw), "FR-1", "FR-2")), &v)
	r := Parse(encoded(t, v), Target{Stage: "step-03", Scope: "stage", References: refs})
	if !hasCode(r, "coverage") {
		t.Fatal(r.Diagnostics)
	}
}
func TestEssentialUnknownCommandBlocksReadyPackage(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-03")
	object(object(object(v["blocks"])["implementation_context"])["technology"])["commands"].(map[string]any)["build"] = nil
	s3 := Parse(encoded(t, v), Target{Stage: "step-03", Scope: "stage", References: refs[:3]})
	if s3.Status != Valid {
		t.Fatal(s3.Diagnostics)
	}
	refs[3] = s3.Artifact
	r := Parse(fixture(t, "step-04"), Target{Stage: "step-04", Scope: "stage", References: refs})
	if r.Status != Invalid || !hasCode(r, "commands") {
		t.Fatal(r.Diagnostics)
	}
}
func TestCriterionCanBeNarrowedForSharedRequirement(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-03")
	b := object(v["blocks"])
	// The canonical criterion covers two modules of the same requirement.
	m1 := object(b["modules"].([]any)[0])
	var m2 map[string]any
	data, _ := json.Marshal(m1)
	_ = json.Unmarshal(data, &m2)
	m2["id"] = "M-2"
	b["modules"] = append(b["modules"].([]any), m2)
	b["build_batches"] = []any{[]any{"M-1", "M-2"}}
	ac := object(b["acceptance"].([]any)[0])
	ac["module_ids"] = []any{"M-1", "M-2"}
	object(b["requirement_coverage"].([]any)[0])["module_ids"] = []any{"M-1", "M-2"}
	s3 := Parse(encoded(t, v), Target{Stage: "step-03", Scope: "stage", References: refs[:3]})
	if s3.Status != Valid {
		t.Fatal(s3.Diagnostics)
	}
	refs[3] = s3.Artifact
	r := Parse(fixture(t, "step-04-package"), Target{Stage: "step-04", Scope: "package", PackageID: "P01", References: refs[:4]})
	if r.Status != Valid {
		t.Fatal(r.Diagnostics)
	}
}

func TestEmptyListItemsAndExactDiagnosticPaths(t *testing.T) {
	v := fixtureJSON(t, "step-02")
	object(object(object(v["blocks"])["design"])["interfaces"].([]any)[0])["owner"] = map[string]any{"kind": "component", "id": "missing"}
	r := Parse(encoded(t, v), Target{Stage: "step-02", Scope: "stage"})
	found := false
	for _, d := range r.Diagnostics {
		if d.Path == "/blocks/design/interfaces/0/owner/id" {
			found = true
		}
	}
	if !found {
		t.Fatal(r.Diagnostics)
	}
	v = fixtureJSON(t, "step-01")
	object(object(v["blocks"])["goals"].([]any)[0])["outcome"] = ""
	object(v["blocks"])["non_goals"] = []any{map[string]any{"item": " ", "reason": "Excluded"}}
	if r := Parse(encoded(t, v), Target{Stage: "step-01", Scope: "stage"}); r.Status != Invalid {
		t.Fatal("empty required string accepted")
	}
	v = fixtureJSON(t, "step-02")
	object(v["blocks"])["scope"].(map[string]any)["never"] = []any{""}
	if r := Parse(encoded(t, v), Target{Stage: "step-02", Scope: "stage"}); r.Status != Invalid {
		t.Fatal("empty substantive string list item accepted")
	}
}

func TestSourceCriteriaResolveAgainstStage3Only(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-04")
	p := object(object(v["blocks"])["packages"].([]any)[0])
	ac := object(p["acceptance"].([]any)[0])
	data, _ := json.Marshal(ac)
	var added map[string]any
	_ = json.Unmarshal(data, &added)
	added["id"] = "AC-invented"
	p["acceptance"] = append(p["acceptance"].([]any), added)
	p["source_acceptance_ids"] = append(p["source_acceptance_ids"].([]any), "AC-invented")
	r := Parse(encoded(t, v), Target{Stage: "step-04", Scope: "stage", References: refs})
	if r.Status != Invalid || !hasCode(r, "reference") {
		t.Fatal(r.Diagnostics)
	}
}
func TestCriteriaUniqueWithinEachPackage(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-04")
	p := object(object(v["blocks"])["packages"].([]any)[0])
	p["acceptance"] = append(p["acceptance"].([]any), p["acceptance"].([]any)[0])
	r := Parse(encoded(t, v), Target{Stage: "step-04", Scope: "stage", References: refs})
	if r.Status != Invalid || !hasCode(r, "duplicate_id") {
		t.Fatal(r.Diagnostics)
	}
}

func TestStage4SharedPackageRecords(t *testing.T) {
	refs := linkedFixtures(t)[:4]
	design := fixtureJSON(t, "step-03")
	b := object(design["blocks"])
	clone := func(value any) map[string]any {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var copy map[string]any
		if err := json.Unmarshal(data, &copy); err != nil {
			t.Fatal(err)
		}
		return copy
	}
	m2 := clone(b["modules"].([]any)[0])
	m2["id"] = "M-2"
	b["modules"] = append(b["modules"].([]any), m2)
	b["build_batches"] = []any{[]any{"M-1", "M-2"}}
	object(b["acceptance"].([]any)[0])["module_ids"] = []any{"M-1", "M-2"}
	object(b["requirement_coverage"].([]any)[0])["module_ids"] = []any{"M-1", "M-2"}
	s3 := Parse(encoded(t, design), Target{Stage: "step-03", Scope: "stage", References: refs[:3]})
	if s3.Status != Valid {
		t.Fatal(s3.Diagnostics)
	}
	refs[3] = s3.Artifact

	for _, tt := range []struct {
		name, code, path string
		change           func(map[string]any, map[string]any)
	}{
		{"identical shared tests", "", "", func(_, _ map[string]any) {}},
		{"conflicting test", "conflict", "/blocks/packages/1/tests/0", func(_, p2 map[string]any) {
			object(p2["tests"].([]any)[0])["assertions"] = []any{"Data may be lost."}
		}},
		{"conflicting criterion links", "conflict", "/blocks/packages/1/acceptance/0", func(_, p2 map[string]any) {
			object(p2["acceptance"].([]any)[0])["test_ids"] = []any{"T-other"}
		}},
		{"duplicate test in first package", "duplicate_id", "/blocks/packages/0/tests/1/id", func(p1, _ map[string]any) {
			p1["tests"] = append(p1["tests"].([]any), p1["tests"].([]any)[0])
		}},
		{"duplicate test in second package", "duplicate_id", "/blocks/packages/1/tests/1/id", func(_, p2 map[string]any) {
			p2["tests"] = append(p2["tests"].([]any), p2["tests"].([]any)[0])
		}},
		{"duplicate criterion in second package", "duplicate_id", "/blocks/packages/1/acceptance/1/id", func(_, p2 map[string]any) {
			p2["acceptance"] = append(p2["acceptance"].([]any), p2["acceptance"].([]any)[0])
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := fixtureJSON(t, "step-04")
			blocks := object(v["blocks"])
			p1 := object(blocks["packages"].([]any)[0])
			p2 := clone(p1)
			p2["id"] = "P02"
			p2["module_ids"] = []any{"M-2"}
			object(p2["acceptance"].([]any)[0])["module_ids"] = []any{"M-2"}
			blocks["packages"] = append(blocks["packages"].([]any), p2)
			blocks["build_batches"] = []any{[]any{"P01", "P02"}}
			tt.change(p1, p2)
			r := Parse(encoded(t, v), Target{Stage: "step-04", Scope: "stage", References: refs})
			if tt.code == "" {
				if r.Status != Valid || hasCode(r, "duplicate_id") {
					t.Fatal(r.Diagnostics)
				}
				return
			}
			for _, d := range r.Diagnostics {
				if r.Status == Invalid && d.Code == tt.code && d.Path == tt.path {
					return
				}
			}
			t.Fatalf("expected %s at %s: %+v", tt.code, tt.path, r.Diagnostics)
		})
	}
}

func TestStage5RejectsMissingKnownPackageAndContracts(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-05")
	v["package_id"] = "P02"
	object(object(v["blocks"])["target"])["id"] = "P02"
	r := Parse(encoded(t, v), Target{Stage: "step-05", Scope: "package", PackageID: "P02", References: refs})
	if r.Status != Invalid || !hasCode(r, "reference") {
		t.Fatal(r.Diagnostics)
	}
	v = fixtureJSON(t, "step-05")
	object(v["blocks"])["interfaces_consumed"] = []any{}
	object(v["blocks"])["interfaces_produced"] = []any{}
	r = Parse(encoded(t, v), Target{Stage: "step-05", Scope: "package", PackageID: "P01", References: refs})
	if r.Status != Invalid || !hasCode(r, "reference") {
		t.Fatal(r.Diagnostics)
	}
}

func TestConflictingUpstreamCataloguesAreNotLastWriterWins(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-03")
	object(object(v["blocks"])["contracts"].([]any)[0])["declaration"] = "func Save() error"
	other := Parse(encoded(t, v), Target{Stage: "step-03", Scope: "stage", References: refs[:3]})
	if other.Status != Valid {
		t.Fatal(other.Diagnostics)
	}
	refs = append(refs, other.Artifact)
	r := Parse(fixture(t, "step-04"), Target{Stage: "step-04", Scope: "stage", References: refs})
	if r.Status != Invalid || !hasCode(r, "conflict") {
		t.Fatal(r.Diagnostics)
	}
}

func TestEarlierStableIDsCannotBeSilentlyRenamed(t *testing.T) {
	refs := linkedFixtures(t)
	for _, tt := range []struct {
		stage, old, new string
		refs            []*Envelope
	}{{"step-02", "FR-1", "FR-renamed", refs[:1]}, {"step-02-5", "CMP-1", "CMP-renamed", refs[:2]}} {
		r := Parse(strings.ReplaceAll(fixture(t, tt.stage), tt.old, tt.new), Target{Stage: tt.stage, Scope: "stage", References: tt.refs})
		if r.Status != Invalid || !hasCode(r, "identity") {
			t.Fatalf("%s silently lost IDs: %+v", tt.stage, r.Diagnostics)
		}
	}
}

func TestGlobalApplicabilityMayRetainExplicitLinks(t *testing.T) {
	v := fixtureJSON(t, "step-01")
	object(object(object(v["blocks"])["constraints"].([]any)[0])["applies_to"])["requirement_ids"] = []any{"FR-1"}
	r := Parse(encoded(t, v), Target{Stage: "step-01", Scope: "stage"})
	if r.Status != Valid {
		t.Fatal(r.Diagnostics)
	}
}
func TestExplicitAuthoritativeBuildCheckCanReplaceCommandKey(t *testing.T) {
	refs := linkedFixtures(t)
	v := fixtureJSON(t, "step-04")
	p := object(object(v["blocks"])["packages"].([]any)[0])
	p["command_keys"] = []any{"test"}
	source := object(p["tests"].([]any)[0])
	data, _ := json.Marshal(source)
	var build map[string]any
	_ = json.Unmarshal(data, &build)
	build["id"] = "T-build"
	build["name"] = "Build package"
	build["command"] = "go build ./..."
	build["required_for_acceptance"] = false
	p["tests"] = append(p["tests"].([]any), build)
	r := Parse(encoded(t, v), Target{Stage: "step-04", Scope: "stage", References: refs})
	if r.Status != Valid {
		t.Fatal(r.Diagnostics)
	}
}
