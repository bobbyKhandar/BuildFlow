package workspace

import (
	"buildflow/internal/structured"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func projectionFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "structured", "testdata", name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	target := structured.Target{Stage: name, Scope: "stage"}
	if name == "step-04-package" {
		target = structured.Target{Stage: "step-04", Scope: "package", PackageID: "P01"}
	}
	parsed := structured.Parse(string(content), target)
	if parsed.Status != structured.Valid {
		t.Fatalf("fixture %s: %+v", name, parsed.Diagnostics)
	}
	return cloneProjectionMap(t, parsed.Artifact)
}

func cloneProjectionMap(t *testing.T, v any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func projectionTestDocuments(t *testing.T) map[string]Doc {
	t.Helper()
	docs := map[string]Doc{}
	for _, stage := range words("step-01 step-02 step-02-5 step-03 step-04") {
		setProjectionDoc(t, docs, KindOutput, stage, "", projectionFixture(t, stage))
	}
	setProjectionDoc(t, docs, KindPackage, "step-04", "P01", projectionFixture(t, "step-04-package"))
	return docs
}

func projectionDocKey(kind, stage, pkg string) string { return kind + "/" + stage + "/" + pkg }

func setProjectionDoc(t *testing.T, docs map[string]Doc, kind, stage, pkg string, envelope map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	content := "# NARRATIVE_SENTINEL\n\n## BuildFlow Structured View\n\n<!-- buildflow:view:start -->\n```json\n" + string(encoded) + "\n```\n<!-- buildflow:view:end -->\n"
	rel, err := Resolve(kind, stage, pkg, "example")
	if err != nil {
		t.Fatal(err)
	}
	docs[projectionDocKey(kind, stage, pkg)] = Doc{Kind: kind, Stage: stage, Package: pkg, Path: filepath.ToSlash(rel), Content: content, Hash: hashBytes([]byte(content)), Exists: true}
}

func runProjection(t *testing.T, docs map[string]Doc, target ProjectionTarget) PromptProjection {
	t.Helper()
	reads := map[string]int{}
	out, err := projectPrompt(target, func(kind, stage, pkg string) (Doc, error) {
		key := projectionDocKey(kind, stage, pkg)
		reads[key]++
		if reads[key] > 1 {
			t.Fatalf("source read more than once: %s", key)
		}
		if d, ok := docs[key]; ok {
			return d, nil
		}
		rel, err := Resolve(kind, stage, pkg, "example")
		return Doc{Path: filepath.ToSlash(rel)}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func requireProjectionReady(t *testing.T, result PromptProjection) {
	t.Helper()
	if result.State != "ready" || result.Data == nil {
		t.Fatalf("projection blocked: %+v", result.Diagnostics)
	}
}

func requireProjectionBlocked(t *testing.T, result PromptProjection, code string) {
	t.Helper()
	if result.State != "blocked" || result.Data != nil {
		t.Fatalf("expected blocked result without partial data: state=%s diagnostics=%+v", result.State, result.Diagnostics)
	}
	for _, d := range result.Diagnostics {
		if d.Code == code && d.Severity == "error" {
			return
		}
	}
	t.Fatalf("missing blocker %s: %+v", code, result.Diagnostics)
}

func TestProjectionAllRegisteredConsumers(t *testing.T) {
	for _, stage := range words("step-02 step-02-5 step-03 step-04 step-05") {
		t.Run(stage, func(t *testing.T) {
			docs := projectionTestDocuments(t)
			target := ProjectionTarget{Stage: stage}
			if stage == "step-05" {
				target.PackageID = "P01"
			}
			out := runProjection(t, docs, target)
			requireProjectionReady(t, out)
			encoded, _ := json.Marshal(out.Data)
			for _, forbidden := range words("NARRATIVE_SENTINEL rejection_reason storage_rationale runner_up maintainer_health cost_note monthly deliberately_excluded design_changes") {
				if strings.Contains(string(encoded), forbidden) {
					t.Fatalf("unselected field or narrative leaked: %s", forbidden)
				}
			}
			for _, source := range out.Sources {
				if source.ProjectionVersion != stage+"/v3" || source.Hash == "" || source.Mode != "json_projection" || len(source.Selections) == 0 || len(source.SelectedPointers) == 0 {
					t.Fatalf("incomplete provenance: %+v", source)
				}
			}
		})
	}
}

func TestProjectionRegistryIsFixedAndInspectable(t *testing.T) {
	for _, stage := range words("step-02 step-02-5 step-03 step-04 step-05") {
		spec, ok := PromptProjectionSpec(stage)
		if !ok || spec.ID != stage+"/v3" || len(spec.Fields) == 0 {
			t.Fatal(spec)
		}
		original := spec.Fields[0].Fields[0:len(spec.Fields[0].Fields):len(spec.Fields[0].Fields)]
		spec.Fields[0].Source = "injected"
		spec.Fields[0].LegacyHeadings[0] = "injected"
		if len(original) > 0 {
			spec.Fields[0].Fields[0] = "injected"
		}
		fresh, _ := PromptProjectionSpec(stage)
		if fresh.Fields[0].Source == "injected" || fresh.Fields[0].LegacyHeadings[0] == "injected" || (len(fresh.Fields[0].Fields) > 0 && fresh.Fields[0].Fields[0] == "injected") {
			t.Fatal("registry can be mutated by callers")
		}
		for _, row := range fresh.Fields {
			if row.Source == "" || row.Output == "" || row.Predicate == "" || row.NullHandling == "" || len(row.LegacyHeadings) == 0 {
				t.Fatalf("incomplete row: %+v", row)
			}
			for _, field := range row.Fields {
				if strings.Contains(field, "*") {
					t.Fatalf("wildcard field: %s", field)
				}
			}
		}
	}
}

// A dependency owns shared declarations, plus private work and its own
// requirement/test/acceptance. A third module is wholly unrelated. Stage 5
// must carry only the selected work and exact shared boundary declarations.
func isolatedProjectionDocuments(t *testing.T) map[string]Doc {
	t.Helper()
	docs := projectionTestDocuments(t)
	s3 := projectionFixture(t, "step-03")
	b := projectionObject(s3["blocks"])
	modules := projectionArray(b["modules"])
	module := projectionObject(modules[0])
	module["depends_on"] = []any{"M-2"}
	requirements, contracts := projectionArray(b["requirements"]), projectionArray(b["contracts"])
	contract2 := projectionObject(contracts[1])
	contract2["owner"] = map[string]any{"kind": "module", "id": "M-2"}
	contract2["referenced_contract_ids"] = []any{"C-3"}
	contract2["declaration"] = "type Document struct { Shared SharedType }"
	shared := cloneProjectionMap(t, contract2)
	shared["id"], shared["name"], shared["declaration"], shared["referenced_contract_ids"] = "C-3", "SharedType", "type SharedType struct { Value string }", []any{}
	contracts = append(contracts, shared)
	for _, n := range []int{2, 3} {
		rid, mid, cid, tid, aid := fmt.Sprintf("FR-%d", n), fmt.Sprintf("M-%d", n), fmt.Sprintf("C-%d", n+1), fmt.Sprintf("T-%d", n), fmt.Sprintf("AC-%d", n)
		if n == 2 {
			cid = "C-2"
		} else {
			cid = "C-4"
		}
		r := cloneProjectionMap(t, requirements[0])
		r["id"], r["statement"] = rid, fmt.Sprintf("UNRELATED_REQUIREMENT_%d", n)
		requirements = append(requirements, r)
		m := cloneProjectionMap(t, module)
		m["id"], m["name"], m["responsibility"], m["why_separate"] = mid, fmt.Sprintf("Boundary%d", n), fmt.Sprintf("PRIVATE_MODULE_BODY_%d", n), "PRIVATE_MODULE_RATIONALE"
		m["component_ids"], m["requirement_ids"], m["depends_on"], m["consumed_contract_ids"] = []any{fmt.Sprintf("CMP-%d", n)}, []any{rid}, []any{}, []any{}
		m["produced_contract_ids"], m["public_contract_ids"] = []any{cid}, []any{cid}
		if n == 2 {
			m["produced_contract_ids"] = []any{"C-2", "C-3"}
		}
		modules = append(modules, m)
		if n == 3 {
			c := cloneProjectionMap(t, shared)
			c["id"], c["name"], c["declaration"], c["owner"] = cid, "UnrelatedType", "UNRELATED_CONTRACT_DECLARATION", map[string]any{"kind": "module", "id": mid}
			contracts = append(contracts, c)
		}
		test := cloneProjectionMap(t, projectionArray(projectionObject(projectionArray(b["test_plan"])[0])["tests"])[0])
		test["id"], test["name"], test["requirement_ids"], test["contract_ids"] = tid, "UNRELATED_TEST_"+tid, []any{rid}, []any{cid}
		b["test_plan"] = append(projectionArray(b["test_plan"]), map[string]any{"module_id": mid, "tests": []any{test}})
		ac := cloneProjectionMap(t, projectionArray(b["acceptance"])[0])
		ac["id"], ac["statement"], ac["requirement_ids"], ac["module_ids"], ac["test_ids"] = aid, "UNRELATED_ACCEPTANCE_"+aid, []any{rid}, []any{mid}, []any{tid}
		b["acceptance"] = append(projectionArray(b["acceptance"]), ac)
		b["requirement_coverage"] = append(projectionArray(b["requirement_coverage"]), map[string]any{"requirement_id": rid, "module_ids": []any{mid}, "disposition": "implemented", "reason": "Assigned"})
	}
	b["requirements"], b["modules"], b["contracts"], b["build_batches"] = requirements, modules, contracts, []any{[]any{"M-2", "M-3"}, []any{"M-1"}}
	context := projectionObject(b["implementation_context"])
	system := projectionObject(context["system_context"])
	invariant := cloneProjectionMap(t, projectionArray(system["invariants"])[0])
	invariant["id"], invariant["rule"], invariant["applies_to"] = "INV-2", "UNRELATED_INVARIANT", linkedProjectionApplicability("module_ids", "M-2")
	system["invariants"] = append(projectionArray(system["invariants"]), invariant)
	system["repository_layout"] = []any{
		map[string]any{"path": ".", "purpose": "Root layout", "example_paths": []any{}, "applies_to": linkedProjectionApplicability("module_ids", "M-2")},
		map[string]any{"path": "backend", "purpose": "UNRELATED_LAYOUT", "example_paths": []any{}, "applies_to": linkedProjectionApplicability("module_ids", "M-2")},
	}
	setup := projectionObject(projectionArray(projectionObject(projectionObject(context["technology"])["environment"])["installation"])[0])
	unrelatedSetup := cloneProjectionMap(t, setup)
	unrelatedSetup["id"], unrelatedSetup["text"], unrelatedSetup["applies_to"] = "ENV-unrelated", "UNRELATED_INSTALL", linkedProjectionApplicability("module_ids", "M-2")
	projectionObject(projectionObject(context["technology"])["environment"])["installation"] = []any{setup, unrelatedSetup}
	setProjectionDoc(t, docs, KindOutput, "step-03", "", s3)
	s4 := projectionFixture(t, "step-04")
	pb := projectionObject(s4["blocks"])
	packages := projectionArray(pb["packages"])
	p := projectionObject(packages[0])
	p["depends_on"], p["prerequisite_reasons"] = []any{"P02"}, []any{map[string]any{"package_id": "P02", "reason": "Shared boundary is available"}}
	for _, n := range []int{2, 3} {
		other := cloneProjectionMap(t, p)
		other["id"], other["title"], other["goal"] = fmt.Sprintf("P%02d", n), "UNRELATED_PACKAGE_TITLE", "UNRELATED_PACKAGE_GOAL"
		other["module_ids"], other["requirement_ids"], other["depends_on"], other["prerequisite_reasons"] = []any{fmt.Sprintf("M-%d", n)}, []any{fmt.Sprintf("FR-%d", n)}, []any{}, []any{}
		other["consumed_contract_ids"], other["produced_contract_ids"] = []any{}, []any{"C-4"}
		if n == 2 {
			other["produced_contract_ids"] = []any{"C-2", "C-3"}
		}
		impl := projectionObject(other["implementation"])
		impl["notes"], impl["contract_ids"] = []any{"UNRELATED_PACKAGE_NOTES"}, other["produced_contract_ids"]
		other["tests"] = projectionObject(projectionArray(b["test_plan"])[n-1])["tests"]
		other["acceptance"], other["source_acceptance_ids"] = []any{projectionArray(b["acceptance"])[n-1]}, []any{fmt.Sprintf("AC-%d", n)}
		packages = append(packages, other)
	}
	pb["packages"], pb["build_batches"] = packages, []any{[]any{"P02", "P03"}, []any{"P01"}}
	pb["integration_points"] = []any{
		map[string]any{"package_ids": []any{"P01", "P02"}, "contract_ids": []any{"C-2"}, "verification": "Check the shared boundary"},
		map[string]any{"package_ids": []any{"P02", "P03"}, "contract_ids": []any{"C-4"}, "verification": "UNRELATED_INTEGRATION"},
	}
	setProjectionDoc(t, docs, KindOutput, "step-04", "", s4)
	selected := cloneProjectionMap(t, s4)
	selected["scope"], selected["package_id"] = "package", "P01"
	selectedBlocks := projectionObject(selected["blocks"])
	selectedBlocks["packages"], selectedBlocks["build_batches"] = []any{p}, []any{[]any{"P01"}}
	selectedBlocks["integration_points"] = []any{projectionArray(pb["integration_points"])[0]}
	setProjectionDoc(t, docs, KindPackage, "step-04", "P01", selected)
	return docs
}

func linkedProjectionApplicability(key, id string) map[string]any {
	app := map[string]any{"scope": "linked", "requirement_ids": []any{}, "component_ids": []any{}, "module_ids": []any{}, "contract_ids": []any{}}
	app[key] = []any{id}
	return app
}

func TestStage5ProjectionIsolationAndContractClosure(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	requireProjectionReady(t, out)
	encoded, _ := json.Marshal(out.Data)
	for _, sentinel := range words("NARRATIVE_SENTINEL UNRELATED_ PRIVATE_MODULE_BODY PRIVATE_MODULE_RATIONALE depends_on_module files_path") {
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("out-of-scope content leaked: %s\n%s", sentinel, encoded)
		}
	}
	if len(projectionArray(out.Data["requirements"])) != 1 || len(projectionArray(out.Data["modules"])) != 1 {
		t.Fatal("incorrect work selection")
	}
	boundaries := projectionArray(out.Data["boundary_modules"])
	if len(boundaries) != 1 || !reflect.DeepEqual(boundaries[0], map[string]any{"id": "M-2", "name": "Boundary2"}) {
		t.Fatalf("boundary contains implementation content: %+v", boundaries)
	}
	contracts := projectionArray(out.Data["contracts"])
	if len(contracts) != 3 {
		t.Fatalf("incomplete or duplicate closure: %+v", contracts)
	}
	for _, declaration := range []string{"func Save(", "type Document struct", "type SharedType struct"} {
		if strings.Count(string(encoded), declaration) != 1 {
			t.Fatalf("declaration not supplied exactly once: %s", declaration)
		}
	}
	commands := projectionObject(projectionObject(out.Data["implementation_context"])["technology"])["commands"]
	if !reflect.DeepEqual(commands, map[string]any{"build": "go build ./...", "test": "go test ./...", "lint": "go vet ./..."}) {
		t.Fatal(commands)
	}
	if len(projectionArray(out.Data["sources"])) != 1 {
		t.Fatalf("evidence not deduplicated: %+v", out.Data["sources"])
	}
	for _, source := range out.Sources {
		if source.Label == "step-03" {
			if len(source.RecordHashes) != 4 {
				t.Fatalf("selected record provenance: %+v", source.RecordHashes)
			}
			for _, pointer := range source.SelectedPointers {
				if strings.Contains(pointer, "/modules/1/") && pointer != "/blocks/modules/1/id" && pointer != "/blocks/modules/1/name" {
					t.Fatalf("private boundary field selected: %s", pointer)
				}
			}
		}
	}
}

func TestStage4CombinedBeforePackageGeneration(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	combined := runProjection(t, docs, ProjectionTarget{Stage: "step-04"})
	requireProjectionReady(t, combined)
	if len(projectionArray(combined.Data["modules"])) != 3 || len(projectionArray(combined.Data["requirements"])) != 3 {
		t.Fatal("combined generation omitted assigned work")
	}
	selected := runProjection(t, docs, ProjectionTarget{Stage: "step-04", PackageID: "P01"})
	requireProjectionReady(t, selected)
	if len(projectionArray(selected.Data["modules"])) != 1 || len(projectionArray(selected.Data["contracts"])) != 3 {
		t.Fatal("package generation did not use mapping and closure")
	}
	for _, source := range selected.Sources {
		if !source.Active || len(source.SelectedPointers) == 0 || len(source.Selections) == 0 {
			t.Fatalf("mapping source lacks selection provenance: %+v", source)
		}
	}
	// Initial slot generation can use the established combined mapping directly.
	withoutSlot := isolatedProjectionDocuments(t)
	delete(withoutSlot, projectionDocKey(KindPackage, "step-04", "P01"))
	requireProjectionReady(t, runProjection(t, withoutSlot, ProjectionTarget{Stage: "step-04", PackageID: "P01"}))
	delete(docs, projectionDocKey(KindOutput, "step-04", ""))
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-04", PackageID: "P01"}), "missing_source")
}

func TestStage4PackageMappingPreflight(t *testing.T) {
	for _, test := range []struct {
		name, kind, pkg, code string
		mutate                func(map[string]any)
	}{
		{"combined contract", KindOutput, "", "reference", func(e map[string]any) {
			p := projectionObject(projectionArray(projectionObject(e["blocks"])["packages"])[0])
			p["produced_contract_ids"] = []any{"C-not-in-design"}
		}},
		{"slot contract", KindPackage, "P01", "reference", func(e map[string]any) {
			p := projectionObject(projectionArray(projectionObject(e["blocks"])["packages"])[0])
			p["produced_contract_ids"] = []any{"C-not-in-design"}
		}},
		{"blocked slot", KindPackage, "P01", "blocked_source", func(e map[string]any) {
			e["state"] = "blocked"
			b := projectionObject(e["blocks"])
			q := map[string]any{"id": "Q-review", "question": "Review the mapping?", "blocking": true, "why_it_matters": "Work is not approved", "cheapest_check": "Review design", "recommended_default": nil, "resolve_by": nil,
				"applies_to": linkedProjectionApplicability("module_ids", "M-1")}
			b["open_questions"] = []any{q}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			docs := projectionTestDocuments(t)
			mutateProjectionDocument(t, docs, test.kind, "step-04", test.pkg, test.mutate)
			requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-04", PackageID: "P01"}), test.code)
		})
	}
}

func TestStage5EmptyCommandSelectionDoesNotIncludeProjectCommands(t *testing.T) {
	docs := projectionTestDocuments(t)
	mutateProjectionDocument(t, docs, KindPackage, "step-04", "P01", func(e map[string]any) {
		p := projectionObject(projectionArray(projectionObject(e["blocks"])["packages"])[0])
		p["command_keys"] = []any{}
		// Both required checks are explicit tests. No project command keys are
		// needed, and run/release commands must not leak into this handoff.
		build := cloneProjectionMap(t, projectionArray(p["tests"])[0])
		build["id"], build["name"], build["command"] = "T-build", "Build package", "go build ./..."
		p["tests"] = append(projectionArray(p["tests"]), build)
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	requireProjectionReady(t, out)
	commands := projectionObject(projectionObject(out.Data["implementation_context"])["technology"])["commands"]
	if !reflect.DeepEqual(commands, map[string]any{}) {
		t.Fatalf("unselected project commands entered the handoff: %+v", commands)
	}
}

func mutateProjectionDocument(t *testing.T, docs map[string]Doc, kind, stage, pkg string, mutate func(map[string]any)) {
	t.Helper()
	doc := docs[projectionDocKey(kind, stage, pkg)]
	target := structured.Target{Stage: stage, Scope: "stage", PackageID: pkg}
	if pkg != "" {
		target.Scope = "package"
	}
	parsed := structured.Parse(doc.Content, target)
	if parsed.Status != structured.Valid {
		t.Fatalf("cannot mutate invalid fixture: %+v", parsed.Diagnostics)
	}
	envelope := cloneProjectionMap(t, parsed.Artifact)
	mutate(envelope)
	setProjectionDoc(t, docs, kind, stage, pkg, envelope)
}

func TestStage5ProjectionPreflightRejectsInvalidLinksAndCopies(t *testing.T) {
	for _, test := range []struct {
		name, code string
		mutate     func(map[string]any)
	}{
		{"requirement", "reference", func(p map[string]any) { p["requirement_ids"] = []any{"FR-missing"} }},
		{"module", "reference", func(p map[string]any) { p["module_ids"] = []any{"M-missing"} }},
		{"contract introduced during packaging", "reference", func(p map[string]any) { p["produced_contract_ids"] = []any{"C-new-in-stage4"} }},
		{"decision", "reference", func(p map[string]any) { projectionObject(p["implementation"])["decision_ids"] = []any{"D-missing"} }},
		{"source criterion", "reference", func(p map[string]any) { p["source_acceptance_ids"] = []any{"AC-missing"} }},
		{"criterion copy", "conflict", func(p map[string]any) {
			projectionObject(projectionArray(p["acceptance"])[0])["statement"] = "A contradictory statement"
		}},
		{"test copy", "conflict", func(p map[string]any) {
			projectionObject(projectionArray(p["tests"])[0])["assertions"] = []any{"Contradictory assertion"}
		}},
		{"unrelated module detail", "package_scope", func(p map[string]any) {
			projectionObject(p["implementation"])["algorithms"] = []any{map[string]any{"id": "ALG-foreign", "module_ids": []any{"M-2"}, "operation": "foreign", "steps": []any{"Do foreign work"}, "pseudocode": nil, "complexity": "O(1)", "input_size": "One", "edge_cases": []any{}}}
		}},
		{"unrelated contract detail", "package_scope", func(p map[string]any) {
			projectionObject(projectionArray(p["tests"])[0])["contract_ids"] = []any{"C-4"}
		}},
		{"acceptance attribution", "acceptance", func(p map[string]any) {
			projectionObject(projectionArray(p["acceptance"])[0])["module_ids"] = []any{"M-2"}
		}},
		{"missing applicable acceptance", "acceptance", func(p map[string]any) { p["source_acceptance_ids"] = []any{} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			docs := isolatedProjectionDocuments(t)
			mutateProjectionDocument(t, docs, KindPackage, "step-04", "P01", func(e map[string]any) {
				test.mutate(projectionObject(projectionArray(projectionObject(e["blocks"])["packages"])[0]))
			})
			requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), test.code)
		})
	}
}

func TestStage5ProjectionNeverSubstitutesCombinedPackage(t *testing.T) {
	docs := projectionTestDocuments(t)
	delete(docs, projectionDocKey(KindPackage, "step-04", "P01"))
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), "missing_source")
	docs = projectionTestDocuments(t)
	mutateProjectionDocument(t, docs, KindPackage, "step-04", "P01", func(e map[string]any) { e["package_id"] = "P02" })
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), "target")
	docs = projectionTestDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-04", "", func(e map[string]any) {
		projectionObject(projectionArray(projectionObject(e["blocks"])["packages"])[0])["goal"] = "STALE_COMBINED_GOAL"
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	requireProjectionReady(t, out)
	if projectionObject(out.Data["package"])["goal"] == "STALE_COMBINED_GOAL" {
		t.Fatal("combined source replaced authoritative saved package")
	}
}

func TestProjectionApplicabilityLinksAndEmptyLinkedScope(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		b := projectionObject(e["blocks"])
		constraints := projectionArray(b["constraints"])
		for i, key := range words("requirement_ids component_ids module_ids contract_ids") {
			id := []string{"FR-1", "CMP-1", "M-1", "C-3"}[i]
			r := cloneProjectionMap(t, constraints[0])
			r["id"], r["statement"], r["applies_to"] = fmt.Sprintf("CON-%d", i+2), "SELECTED_"+key, linkedProjectionApplicability(key, id)
			constraints = append(constraints, r)
		}
		b["constraints"] = constraints
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	requireProjectionReady(t, out)
	if len(projectionArray(out.Data["constraints"])) != 5 {
		t.Fatal("explicit applicability did not match all four link dimensions")
	}
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		projectionObject(projectionArray(projectionObject(e["blocks"])["constraints"])[0])["applies_to"] = linkedProjectionApplicability("module_ids", "")
	})
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), "reference")
	docs = projectionTestDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		app := linkedProjectionApplicability("module_ids", "M-1")
		app["module_ids"] = []any{}
		projectionObject(projectionArray(projectionObject(e["blocks"])["constraints"])[0])["applies_to"] = app
	})
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), "applicability")
}

func TestStage3AcceptedOperationalChangesReachStage5(t *testing.T) {
	docs := projectionTestDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		context := projectionObject(projectionObject(e["blocks"])["implementation_context"])
		tech := projectionObject(context["technology"])
		projectionObject(projectionArray(tech["stack"])[0])["version"] = "1.26-accepted"
		projectionObject(tech["commands"])["build"] = "go build -trimpath ./..."
		projectionObject(projectionArray(projectionObject(context["system_context"])["invariants"])[0])["rule"] = "Accepted exact invariant.\nPreserve {{literal}} and café."
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	requireProjectionReady(t, out)
	encoded, _ := json.Marshal(out.Data)
	for _, wanted := range []string{"1.26-accepted", "go build -trimpath ./...", "Accepted exact invariant.", "{{literal}}", "café"} {
		if !strings.Contains(string(encoded), wanted) {
			t.Fatalf("effective change lost: %s", wanted)
		}
	}
	for _, source := range out.Sources {
		if strings.Contains(source.Path, "step-02") {
			t.Fatalf("Stage 5 recovered obsolete normal-path context: %s", source.Path)
		}
	}
}

func TestStage3ContractSupersessionAndEvidenceDedup(t *testing.T) {
	docs := projectionTestDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-02-5", "", func(e map[string]any) {
		projectionObject(projectionArray(projectionObject(e["blocks"])["contracts"])[0])["declaration"] = "func EffectiveSave() error"
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-03"})
	requireProjectionReady(t, out)
	if len(projectionArray(out.Data["contracts"])) != 2 || len(projectionArray(out.Data["sources"])) != 1 {
		t.Fatal("canonical definitions were duplicated")
	}
	if projectionObject(projectionArray(out.Data["contracts"])[0])["declaration"] != "func EffectiveSave() error" {
		t.Fatal("Stage 2.5 contract did not supersede Stage 2")
	}
	mutateProjectionDocument(t, docs, KindOutput, "step-02-5", "", func(e map[string]any) {
		projectionObject(projectionArray(projectionObject(e["blocks"])["sources"])[0])["locator"] = "Conflicting evidence"
	})
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-03"}), "conflict")
}

func TestStage5SelectedReferenceFreshness(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	baseline := runProjection(t, docs, ProjectionTarget{Stage: "step-04"})
	requireProjectionReady(t, baseline)
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		projectionObject(projectionArray(projectionObject(e["blocks"])["requirements"])[1])["statement"] = "Unrelated accepted change"
	})
	target := ProjectionTarget{Stage: "step-05", PackageID: "P01", Baseline: baseline.Sources}
	out := runProjection(t, docs, target)
	requireProjectionReady(t, out)
	found := false
	for _, d := range out.Diagnostics {
		if d.Code == "source_changed" {
			found = true
		}
	}
	if !found {
		t.Fatal("source edit was not reported")
	}
	for _, test := range []struct{ collection, field, value string }{{"requirements", "statement", "Incompatible selected requirement"}, {"contracts", "declaration", "func ChangedSignature() error"}} {
		t.Run(test.collection, func(t *testing.T) {
			changed := isolatedProjectionDocuments(t)
			mutateProjectionDocument(t, changed, KindOutput, "step-03", "", func(e map[string]any) {
				projectionObject(projectionArray(projectionObject(e["blocks"])[test.collection])[0])[test.field] = test.value
			})
			requireProjectionBlocked(t, runProjection(t, changed, target), "package_review_required")
		})
	}
	docs = isolatedProjectionDocuments(t)
	doc := docs[projectionDocKey(KindOutput, "step-03", "")]
	doc.ModTime = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	docs[projectionDocKey(KindOutput, "step-03", "")] = doc
	pkg := docs[projectionDocKey(KindPackage, "step-04", "P01")]
	pkg.ModTime = doc.ModTime.Add(-time.Hour)
	docs[projectionDocKey(KindPackage, "step-04", "P01")] = pkg
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), "package_review_required")
}

func TestProjectionMissingCommandsNullContextAndContractClosureBlock(t *testing.T) {
	for _, test := range []struct {
		name, code string
		mutate     func(map[string]any)
	}{
		{"command", "commands", func(b map[string]any) {
			projectionObject(projectionObject(projectionObject(b["implementation_context"])["technology"])["commands"])["build"] = nil
		}},
		{"context", "schema", func(b map[string]any) { b["implementation_context"] = nil }},
		{"contract dependency", "reference", func(b map[string]any) {
			projectionObject(projectionArray(b["contracts"])[0])["referenced_contract_ids"] = []any{"C-missing-type"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			docs := projectionTestDocuments(t)
			mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) { test.mutate(projectionObject(e["blocks"])) })
			requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), test.code)
		})
	}
}

func TestProjectionEvidenceUsesOnlySelectedBasis(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		b := projectionObject(e["blocks"])
		unrelated := cloneProjectionMap(t, projectionArray(b["sources"])[0])
		unrelated["id"], unrelated["locator"] = "SRC-unused", "UNREFERENCED_SOURCE"
		b["sources"] = append(projectionArray(b["sources"]), unrelated)
		required := map[string]any{"id": "AS-required", "statement": "Required evidence assumption", "consequence_if_wrong": "Revisit the requirement", "validation": "Check explicitly", "applies_to": linkedProjectionApplicability("module_ids", "M-2")}
		b["assumptions"] = []any{required}
		basis := projectionObject(projectionObject(projectionArray(b["requirements"])[0])["basis"])
		basis["confidence"], basis["assumption_ids"] = "assumption", []any{"AS-required"}
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	requireProjectionReady(t, out)
	if len(projectionArray(out.Data["sources"])) != 1 || len(projectionArray(out.Data["assumptions"])) != 1 {
		t.Fatalf("evidence closure: %+v", out.Data)
	}
	encoded, _ := json.Marshal(out.Data)
	if strings.Contains(string(encoded), "UNREFERENCED_SOURCE") || !strings.Contains(string(encoded), "Required evidence assumption") {
		t.Fatal("evidence selection did not follow the selected basis")
	}
}

func TestProjectionContractCyclesAreDeduplicated(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	mutateProjectionDocument(t, docs, KindOutput, "step-03", "", func(e map[string]any) {
		projectionObject(projectionArray(projectionObject(e["blocks"])["contracts"])[2])["referenced_contract_ids"] = []any{"C-2"}
	})
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"})
	requireProjectionReady(t, out)
	if len(projectionArray(out.Data["contracts"])) != 3 {
		t.Fatal("recursive type closure duplicated records")
	}
}

func TestStage5PreflightRequiresResolvablePrerequisites(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	delete(docs, projectionDocKey(KindOutput, "step-04", ""))
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), "prerequisite")
}

func TestStage5PreflightRejectsUnrelatedSignalOwner(t *testing.T) {
	docs := isolatedProjectionDocuments(t)
	mutateProjectionDocument(t, docs, KindPackage, "step-04", "P01", func(e map[string]any) {
		p := projectionObject(projectionArray(projectionObject(e["blocks"])["packages"])[0])
		projectionObject(p["implementation"])["observability"] = []any{map[string]any{"id": "SIG-foreign", "owner_ids": []any{"M-2"}, "kind": "log", "name": "foreign.log", "level_or_unit": "info", "emitted_when": "On work", "diagnostic_use": "Foreign work"}}
	})
	requireProjectionBlocked(t, runProjection(t, docs, ProjectionTarget{Stage: "step-05", PackageID: "P01"}), "package_scope")
}

func TestProjectionLegacyAdapterProvenance(t *testing.T) {
	docs := projectionTestDocuments(t)
	content, err := os.ReadFile(filepath.Join("..", "structured", "testdata", "legacy-v1-research.md"))
	if err != nil {
		t.Fatal(err)
	}
	d := docs[projectionDocKey(KindOutput, "step-01", "")]
	d.Content, d.Hash = string(content), hashBytes(content)
	docs[projectionDocKey(KindOutput, "step-01", "")] = d
	out := runProjection(t, docs, ProjectionTarget{Stage: "step-02"})
	requireProjectionReady(t, out)
	if out.Sources[0].Mode != "json_legacy_adapter" {
		t.Fatalf("unlabelled adapted source: %+v", out.Sources[0])
	}
}

func TestAssemblePromptHandoffUsesProjectionAndReportsBlockers(t *testing.T) {
	s := NewService()
	if _, err := s.EnsureWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	project, err := s.CreateProject("Projection assembly", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveTemplate("step-05", "local", project.ID, "{{handoff}}", "", true); err != nil {
		t.Fatal(err)
	}
	docs := projectionTestDocuments(t)
	for _, key := range []string{projectionDocKey(KindOutput, "step-03", ""), projectionDocKey(KindPackage, "step-04", "P01")} {
		d := docs[key]
		if _, err := s.WriteDoc(WriteRequest{Kind: d.Kind, Stage: d.Stage, Package: d.Package, Project: project.ID, Content: d.Content, Force: true}); err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.AssemblePrompt(project.ID, "step-05", "P01")
	if err != nil {
		t.Fatal(err)
	}
	if out.Projection == nil {
		t.Fatal("projection metadata missing from assembly")
	}
	requireProjectionReady(t, *out.Projection)
	var data map[string]any
	if err := json.Unmarshal([]byte(renderedProjectionJSON(t, out.Content)), &data); err != nil {
		t.Fatal(err)
	}
	declaration := projectionObject(projectionArray(data["contracts"])[0])["declaration"]
	if !strings.Contains(projectionString(declaration), "{{literal}}") {
		t.Fatal("substituted value was recursively expanded")
	}
	if strings.Contains(out.Content, "NARRATIVE_SENTINEL") || strings.Contains(out.Content, "selectedPointers") {
		t.Fatal("narrative or provenance entered projected data")
	}
	// P02 has no saved package. Package documents are immutable history from
	// the deletion API's perspective, so build the missing-source case directly.
	blocked, err := s.AssemblePrompt(project.ID, "step-05", "P02")
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Projection == nil || blocked.Projection.State != "blocked" || !strings.Contains(blocked.Content, `"state":"blocked"`) {
		t.Fatal("assembly did not surface preflight blockers")
	}
	for _, warning := range blocked.Warnings {
		if warning.Required && warning.Token == "handoff" {
			return
		}
	}
	t.Fatal("assembly warnings omitted required projection blocker")
}
