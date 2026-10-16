package workspace

import (
	"buildflow/internal/structured"
	"encoding/json"
	"fmt"
	"strings"
)

// InspectDocument exposes validated blocks without changing the saved source.
// Package documents are independent of the combined Stage 4 answer. In
// particular, a slot edit is checked against Stage 3, not an older stage copy.
func (s *Service) InspectDocument(project string, doc Doc) *structured.Result {
	target := structured.Target{Stage: doc.Stage, Scope: "stage"}
	switch doc.Kind {
	case KindPackage:
		target.Stage, target.Scope, target.PackageID = "step-04", "package", doc.Package
	case KindPackageOutput:
		target.Stage, target.Scope, target.PackageID = "step-05", "package", doc.Package
	case KindOutput:
	default:
		return nil
	}
	if !IsStage(target.Stage) {
		return nil
	}
	if target.Stage == "step-04" || target.Stage == "step-05" {
		if design, err := s.ReadDoc(KindOutput, "step-03", "", project); err == nil && design.Exists {
			parsed := structured.Parse(design.Content, structured.Target{Stage: "step-03", Scope: "stage"})
			if parsed.Status == structured.Valid {
				target.References = append(target.References, parsed.Artifact)
			}
		}
	}
	if target.Stage == "step-05" {
		if pkg, err := s.ReadDoc(KindPackage, "", target.PackageID, project); err == nil && pkg.Exists {
			parsed := structured.Parse(pkg.Content, structured.Target{Stage: "step-04", Scope: "package", PackageID: target.PackageID, References: target.References})
			if parsed.Status == structured.Valid {
				target.References = append(target.References, parsed.Artifact)
			}
		}
	}
	result := structured.Parse(doc.Content, target)
	return &result
}

// deliveryOverviewOffset ignores headings inside examples and code fences.
func deliveryOverviewOffset(content string) int {
	return outsideHeadingOffset(content, "## Delivery Overview")
}

func outsideHeadingOffset(content, heading string) int {
	lines := strings.Split(content, "\n")
	var candidates []headingHit
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "##")), heading) {
			candidates = append(candidates, headingHit{line: i})
		}
	}
	candidates = dropFencedHits(lines, candidates)
	if len(candidates) == 0 {
		return -1
	}
	return offsetOfLine(content, candidates[0].line)
}

// projectPackageFooter copies complete package records while narrowing the
// stage-level collections. Field selection stays in Go and never uses model
// supplied selectors. The source hash identifies the exact combined answer.
func projectPackageFooter(source *structured.Envelope, id, title, path, hash string, design *structured.Envelope) (string, *AssemblySource, error) {
	raw, err := json.Marshal(source.Blocks)
	if err != nil {
		return "", nil, err
	}
	var blocks map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", nil, err
	}
	pkg := findProjectionRecord(blocks["packages"], id)
	if pkg == nil || !strings.EqualFold(strings.Join(strings.Fields(title), " "), strings.Join(strings.Fields(projectionString(pkg["title"])), " ")) {
		return "", nil, fmt.Errorf("%s heading must match a structured package ID and title; its Markdown is preserved as unassigned content", id)
	}
	context := projectionContext{filtered: true, requirements: map[string]bool{}, components: map[string]bool{}, modules: map[string]bool{}, contracts: map[string]bool{}}
	addProjectionIDs(context.requirements, pkg["requirement_ids"])
	addProjectionIDs(context.modules, pkg["module_ids"])
	addProjectionIDs(context.contracts, pkg["consumed_contract_ids"])
	addProjectionIDs(context.contracts, pkg["produced_contract_ids"])
	addProjectionIDs(context.contracts, projectionObject(pkg["implementation"])["contract_ids"])
	if design != nil {
		encoded, _ := json.Marshal(design.Blocks)
		var upstream map[string]any
		_ = json.Unmarshal(encoded, &upstream)
		for mid := range context.modules {
			module := findProjectionRecord(upstream["modules"], mid)
			addProjectionIDs(context.components, module["component_ids"])
		}
		visited := map[string]bool{}
		var visit func(string)
		visit = func(id string) {
			if visited[id] {
				return
			}
			visited[id] = true
			contract := findProjectionRecord(upstream["contracts"], id)
			for _, dep := range projectionStrings(contract["referenced_contract_ids"]) {
				context.contracts[dep] = true
				visit(dep)
			}
		}
		for _, id := range sortedProjectionIDs(context.contracts) {
			visit(id)
		}
	}
	builder := projectionBuilder{context: context}
	out := map[string]any{"packages": []any{pkg}, "build_batches": [][]string{{id}}, "deliberately_not_created": []any{}}
	provenance := &AssemblySource{Token: "package-import", Label: "Combined Stage 4 answer", Path: path, Exists: true, Hash: hash, Active: true, Mode: "structured", ProjectionVersion: "package-footer/v2", RecordHashes: map[string]string{}}
	selectRecord := func(collection string, i int, record any) {
		pointer := fmt.Sprintf("/blocks/%s/%d", collection, i)
		provenance.SelectedPointers = append(provenance.SelectedPointers, pointer)
		encoded, _ := json.Marshal(record)
		provenance.RecordHashes[pointer] = hashBytes(encoded)
	}
	for i, record := range projectionArray(blocks["packages"]) {
		if projectionString(projectionObject(record)["id"]) == id {
			selectRecord("packages", i, record)
		}
	}
	evidence := &projectionDocument{sources: map[string]bool{}, assumptions: map[string]bool{}}
	collectProjectionEvidence(pkg, evidence)
	integrations := []any{}
	for i, record := range projectionArray(blocks["integration_points"]) {
		if intersectsProjectionIDs(projectionObject(record)["package_ids"], map[string]bool{id: true}) {
			integrations = append(integrations, record)
			selectRecord("integration_points", i, record)
			collectProjectionEvidence(record, evidence)
		}
	}
	out["integration_points"] = integrations
	for _, key := range []string{"risks", "open_questions", "assumptions"} {
		records := []any{}
		for i, record := range projectionArray(blocks[key]) {
			row := projectionObject(record)
			if builder.applicable(row) || (key == "assumptions" && evidence.assumptions[projectionString(row["id"])]) {
				records = append(records, record)
				selectRecord(key, i, record)
				collectProjectionEvidence(record, evidence)
			}
		}
		out[key] = records
	}
	sources := []any{}
	for i, record := range projectionArray(blocks["sources"]) {
		if evidence.sources[projectionString(projectionObject(record)["id"])] {
			sources = append(sources, record)
			selectRecord("sources", i, record)
		}
	}
	out["sources"] = sources
	envelope := structured.Envelope{SchemaVersion: structured.SchemaVersion, Stage: "step-04", Scope: "package", PackageID: &id, State: source.State, Blocks: out}
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return "", nil, err
	}
	return "## BuildFlow Structured View\n\n<!-- buildflow:view:start -->\n```json\n" + string(encoded) + "\n```\n<!-- buildflow:view:end -->\n", provenance, nil
}
