package workspace

import (
	"buildflow/internal/structured"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// ProjectionTarget selects a fixed consumer and optional package. Baseline is
// application-owned provenance from packaging, never an AI-authored hash. It
// allows a caller to distinguish unrelated source edits from incompatible
// selected requirement/declaration changes. Without it, a newer design requires
// package review rather than an unsupported claim that the package is current.
type ProjectionTarget struct {
	Stage     string
	PackageID string
	Baseline  []AssemblySource
}

type ProjectionSelection struct {
	SourcePointer string   `json:"sourcePointer"`
	OutputPointer string   `json:"outputPointer"`
	Predicate     string   `json:"predicate"`
	Fields        []string `json:"fields"`
	SelectedIDs   []string `json:"selectedIDs"`
}

type ProjectionDiagnostic struct {
	SourcePath string `json:"sourcePath"`
	structured.Diagnostic
}

// PromptProjection is data plus provenance, independent of Markdown rendering
// and template/note wiring. Blocked results deliberately expose no partial Data.
type PromptProjection struct {
	Version     string                 `json:"version"`
	Stage       string                 `json:"stage"`
	Scope       string                 `json:"scope"`
	PackageID   string                 `json:"packageID,omitempty"`
	State       string                 `json:"state"`
	Data        map[string]any         `json:"data"`
	Sources     []AssemblySource       `json:"sources"`
	Diagnostics []ProjectionDiagnostic `json:"diagnostics"`
}

// ProjectPrompt reads only the registered sources. In particular Stage 5 starts
// with the saved package file and never substitutes a combined-answer package.
func (s *Service) ProjectPrompt(project string, target ProjectionTarget) (PromptProjection, error) {
	return projectPrompt(target, func(kind, stage, pkg string) (Doc, error) {
		return s.ReadDoc(kind, stage, pkg, project)
	})
}

type projectionDocument struct {
	doc                  Doc
	parsed               structured.Result
	blocks               map[string]any
	index                int
	sources, assumptions map[string]bool
}

type projectionContext struct {
	filtered                                                                    bool
	modules, requirements, components, contracts, boundaries, owners, decisions map[string]bool
	commands                                                                    map[string]bool
	files                                                                       []string
	pkg                                                                         map[string]any
	superseded                                                                  map[string]bool
}

type projectionBuilder struct {
	target    ProjectionTarget
	out       PromptProjection
	load      func(string, string, string) (Doc, error)
	documents map[string]*projectionDocument
	cache     map[string]*projectionDocument
	context   projectionContext
}

func projectPrompt(target ProjectionTarget, load func(string, string, string) (Doc, error)) (PromptProjection, error) {
	spec, ok := PromptProjectionSpec(target.Stage)
	if !ok {
		return PromptProjection{}, fmt.Errorf("no projection registered for %q", target.Stage)
	}
	if target.PackageID != "" {
		if target.Stage != "step-04" && target.Stage != "step-05" {
			return PromptProjection{}, fmt.Errorf("%s does not support package scope", target.Stage)
		}
		if err := ValidatePackageID(target.PackageID); err != nil {
			return PromptProjection{}, err
		}
	} else if target.Stage == "step-05" {
		return PromptProjection{}, fmt.Errorf("Stage 5 requires a selected package")
	}
	b := &projectionBuilder{target: target, load: load, documents: map[string]*projectionDocument{}, cache: map[string]*projectionDocument{},
		out:     PromptProjection{Version: spec.ID, Stage: target.Stage, Scope: "stage", State: "ready", PackageID: target.PackageID, Data: map[string]any{}, Sources: []AssemblySource{}, Diagnostics: []ProjectionDiagnostic{}},
		context: projectionContext{modules: map[string]bool{}, requirements: map[string]bool{}, components: map[string]bool{}, contracts: map[string]bool{}, boundaries: map[string]bool{}, owners: map[string]bool{}, decisions: map[string]bool{}, commands: map[string]bool{}, superseded: map[string]bool{}},
	}
	if target.PackageID != "" {
		b.out.Scope = "package"
	}
	// Source ordering is explicit: prerequisites are parsed once, before their
	// consumers. Their complete effective catalogues remain available to validation.
	var refs []*structured.Envelope
	for _, stage := range words("step-01 step-02 step-02-5 step-03 step-04") {
		needed := false
		for _, row := range spec.Fields {
			if row.SourceStage == stage {
				needed = true
				break
			}
		}
		if !needed {
			continue
		}
		kind, scope, pkg := KindOutput, "stage", ""
		if stage == "step-04" && target.Stage == "step-05" {
			kind, scope, pkg = KindPackage, "package", target.PackageID
		}
		d, err := b.read(kind, stage, scope, pkg, true, refs)
		if err != nil {
			return PromptProjection{}, err
		}
		if d.parsed.Artifact != nil {
			refs = append(refs, d.parsed.Artifact)
		}
	}
	if target.Stage == "step-03" {
		if d := b.documents["step-02-5"]; d != nil {
			for _, c := range projectionArray(d.blocks["contracts"]) {
				b.context.superseded[projectionString(projectionObject(c)["id"])] = true
			}
		}
	}
	if target.Stage == "step-04" || target.Stage == "step-05" {
		if err := b.selectWork(); err != nil {
			return PromptProjection{}, err
		}
	}
	if b.hasErrors() {
		return b.finish(), nil
	}
	// Sources and evidence assumptions are deferred until all selected basis
	// records have been visited, including any other selected assumptions.
	for _, row := range spec.Fields {
		if row.Source == "sources[]" || row.Source == "assumptions[]" {
			continue
		}
		b.selectRow(row)
	}
	for _, row := range spec.Fields {
		if row.Source == "assumptions[]" {
			b.selectRow(row)
		}
	}
	for _, row := range spec.Fields {
		if row.Source == "sources[]" {
			b.selectRow(row)
		}
	}
	if target.Stage == "step-05" {
		b.checkFreshness()
	}
	return b.finish(), nil
}

func (b *projectionBuilder) diagnostic(d *projectionDocument, pointer, code, message, severity string) {
	source := ""
	if d != nil {
		source = d.doc.Path
	}
	b.out.Diagnostics = append(b.out.Diagnostics, ProjectionDiagnostic{source, structured.Diagnostic{Path: pointer, Code: code, Message: message, Severity: severity}})
}
func (b *projectionBuilder) hasErrors() bool {
	for _, d := range b.out.Diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}
func (b *projectionBuilder) finish() PromptProjection {
	if b.hasErrors() {
		b.out.State, b.out.Data = "blocked", nil
	}
	for i := range b.out.Sources {
		sort.Strings(b.out.Sources[i].SelectedPointers)
		b.out.Sources[i].SelectedPointers = uniqueStrings(b.out.Sources[i].SelectedPointers)
	}
	return b.out
}

func (b *projectionBuilder) read(kind, stage, scope, pkg string, required bool, refs []*structured.Envelope) (*projectionDocument, error) {
	key := kind + "/" + stage + "/" + pkg
	if d, exists := b.cache[key]; exists {
		return d, nil
	}
	doc, err := b.load(kind, stage, pkg)
	if err != nil {
		return nil, err
	}
	d := &projectionDocument{doc: doc, index: len(b.out.Sources), sources: map[string]bool{}, assumptions: map[string]bool{}}
	b.cache[key] = d
	if required && b.documents[stage] == nil {
		b.documents[stage] = d
	}
	// The registered source for Stage 5 is always its selected saved package.
	if kind == KindPackage && b.target.Stage == "step-05" && pkg == b.target.PackageID {
		b.documents[stage] = d
	}
	b.out.Sources = append(b.out.Sources, AssemblySource{Token: "handoff", Label: stage, Path: doc.Path, Exists: doc.Exists, Hash: doc.Hash, Active: required, ProjectionVersion: b.out.Version, Mode: "missing", RecordHashes: map[string]string{}})
	if !doc.Exists || strings.TrimSpace(doc.Content) == "" {
		if required {
			b.diagnostic(d, "", "missing_source", "save the required "+stage+" source before projecting", "error")
		}
		return d, nil
	}
	d.parsed = structured.Parse(doc.Content, structured.Target{Stage: stage, Scope: scope, PackageID: pkg, References: refs})
	if d.parsed.Status != structured.Valid {
		if required {
			for _, diagnostic := range d.parsed.Diagnostics {
				b.out.Diagnostics = append(b.out.Diagnostics, ProjectionDiagnostic{doc.Path, diagnostic})
			}
			b.diagnostic(d, "", "invalid_source", "validated "+stage+" data is unavailable ("+string(d.parsed.Status)+"); structured projection cannot select fields", "error")
		}
		return d, nil
	}
	data, err := json.Marshal(d.parsed.Blocks)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &d.blocks); err != nil {
		return nil, err
	}
	b.out.Sources[d.index].Mode = "json_projection"
	if d.parsed.Adapted {
		b.out.Sources[d.index].Mode = "json_legacy_adapter"
	}
	if required {
		for _, diagnostic := range d.parsed.Diagnostics {
			b.out.Diagnostics = append(b.out.Diagnostics, ProjectionDiagnostic{doc.Path, diagnostic})
		}
		if d.parsed.Artifact.State != "ready" {
			b.diagnostic(d, "/state", "blocked_source", stage+" is blocked; resolve its prerequisites first", "error")
		}
	}
	return d, nil
}

func (b *projectionBuilder) selectWork() error {
	design := b.documents["step-03"]
	if design == nil || design.blocks == nil {
		return nil
	}
	c := &b.context
	c.filtered = true
	if b.target.PackageID != "" {
		var selected *projectionDocument
		if b.target.Stage == "step-05" {
			selected = b.documents["step-04"]
		} else {
			// Approved workflow: generate all packages before specializing a slot.
			refs := []*structured.Envelope{design.parsed.Artifact}
			combined, err := b.read(KindOutput, "step-04", "stage", "", true, refs)
			if err != nil {
				return err
			}
			selected = combined
			if combined.blocks != nil && findProjectionRecord(combined.blocks["packages"], b.target.PackageID) == nil {
				b.diagnostic(combined, "/blocks/packages", "package_mapping", "generate the combined Stage 4 package mapping containing "+b.target.PackageID+" first", "error")
			}
			b.recordPackageJoin(combined, b.target.PackageID, words("id"), "established_package")
			// A saved slot may refine the established mapping. Missing slots are
			// expected during initial generation; malformed saved slots are blockers.
			slot, err := b.read(KindPackage, "step-04", "package", b.target.PackageID, false, refs)
			if err != nil {
				return err
			}
			if slot.doc.Exists {
				selected = slot
				for _, diagnostic := range slot.parsed.Diagnostics {
					b.out.Diagnostics = append(b.out.Diagnostics, ProjectionDiagnostic{slot.doc.Path, diagnostic})
				}
				if slot.blocks == nil {
					b.diagnostic(slot, "", "invalid_source", "the saved package mapping is invalid", "error")
				} else if slot.parsed.Artifact.State != "ready" {
					b.diagnostic(slot, "/state", "blocked_source", "the saved package mapping is blocked; resolve its prerequisites first", "error")
				}
			}
		}
		if selected == nil || selected.blocks == nil {
			return nil
		}
		c.pkg = findProjectionRecord(selected.blocks["packages"], b.target.PackageID)
		if c.pkg == nil {
			b.diagnostic(selected, "/blocks/packages", "package_mapping", "selected package does not exist in the established mapping", "error")
			return nil
		}
		if b.target.Stage == "step-04" {
			b.recordPackageJoin(selected, b.target.PackageID, words("id module_ids[] requirement_ids[] consumed_contract_ids[] produced_contract_ids[] implementation.contract_ids[]"), "package_mapping")
		}
		addProjectionIDs(c.modules, c.pkg["module_ids"])
		addProjectionIDs(c.requirements, c.pkg["requirement_ids"])
		addProjectionIDs(c.contracts, c.pkg["consumed_contract_ids"])
		addProjectionIDs(c.contracts, c.pkg["produced_contract_ids"])
		impl := projectionObject(c.pkg["implementation"])
		addProjectionIDs(c.contracts, impl["contract_ids"])
		addProjectionIDs(c.decisions, impl["decision_ids"])
		addProjectionIDs(c.commands, c.pkg["command_keys"])
		for _, f := range projectionArray(c.pkg["files"]) {
			c.files = append(c.files, projectionString(projectionObject(f)["path"]))
		}
	} else {
		for _, coverage := range projectionArray(design.blocks["requirement_coverage"]) {
			r := projectionObject(coverage)
			if r["disposition"] == "implemented" {
				addProjectionIDs(c.modules, r["module_ids"])
			}
		}
	}
	for _, mid := range sortedProjectionIDs(c.modules) {
		m := findProjectionRecord(design.blocks["modules"], mid)
		if m == nil {
			b.diagnostic(design, "/blocks/modules", "reference", "unknown assigned module "+mid, "error")
			continue
		}
		addProjectionIDs(c.components, m["component_ids"])
		c.owners[mid] = true
		if b.target.Stage == "step-04" {
			addProjectionIDs(c.contracts, m["consumed_contract_ids"])
			addProjectionIDs(c.contracts, m["produced_contract_ids"])
			if b.target.PackageID == "" {
				addProjectionIDs(c.requirements, m["requirement_ids"])
			}
		}
	}
	for _, rid := range sortedProjectionIDs(c.requirements) {
		if findProjectionRecord(design.blocks["requirements"], rid) == nil {
			b.diagnostic(design, "/blocks/requirements", "reference", "unknown assigned requirement "+rid, "error")
		}
	}
	// Only explicit declaration references grow this set. Module dependencies
	// never import algorithms, files, tests, or transitive implementation context.
	visited := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		contract := findProjectionRecord(design.blocks["contracts"], id)
		if contract == nil {
			b.diagnostic(design, "/blocks/contracts", "contract_reference", "contract "+id+" must be recorded in the effective Stage 3 design before Stage 5", "error")
			return
		}
		owner := projectionObject(contract["owner"])
		if owner["kind"] == "module" && !c.modules[projectionString(owner["id"])] {
			c.boundaries[projectionString(owner["id"])] = true
		}
		for _, dep := range projectionStrings(contract["referenced_contract_ids"]) {
			c.contracts[dep] = true
			visit(dep)
		}
	}
	for _, id := range sortedProjectionIDs(c.contracts) {
		visit(id)
	}
	ctx := projectionObject(design.blocks["implementation_context"])
	if ctx == nil {
		b.diagnostic(design, "/blocks/implementation_context", "required_null", "effective implementation context is undetermined", "error")
		return nil
	}
	for _, service := range projectionArray(projectionObject(ctx["technology"])["services"]) {
		r := projectionObject(service)
		if b.applicable(r) {
			c.owners[projectionString(r["id"])] = true
			if b.target.Stage == "step-05" {
				for _, cid := range projectionStrings(r["contract_ids"]) {
					if !c.contracts[cid] {
						b.diagnostic(design, "/blocks/implementation_context/technology/services", "package_scope", "applicable service contract "+cid+" must be linked by the package", "error")
					}
				}
			}
		}
	}
	if b.target.Stage == "step-05" {
		selected := b.documents["step-04"]
		for _, signal := range projectionArray(projectionObject(c.pkg["implementation"])["observability"]) {
			for _, owner := range projectionStrings(projectionObject(signal)["owner_ids"]) {
				if !c.owners[owner] {
					b.diagnostic(selected, "/blocks/packages/0/implementation/observability", "package_scope", "package signal references unrelated module/service owner "+owner, "error")
				}
			}
		}
		for _, point := range projectionArray(selected.blocks["integration_points"]) {
			r := projectionObject(point)
			if b.matches("integration_points", r, selected) {
				for _, cid := range projectionStrings(r["contract_ids"]) {
					if !c.contracts[cid] {
						b.diagnostic(selected, "/blocks/integration_points", "package_scope", "integration contract "+cid+" must be linked by the selected package", "error")
					}
				}
			}
		}
		if err := b.checkPrerequisites(); err != nil {
			return err
		}
	}
	return nil
}

func (b *projectionBuilder) checkPrerequisites() error {
	deps := projectionStrings(b.context.pkg["depends_on"])
	if len(deps) == 0 {
		return nil
	}
	// Combined data supplies identities only. It never supplies selected package
	// content, and is not traversed to collect dependencies' implementations.
	combined, err := b.read(KindOutput, "step-04", "stage", "", false, nil)
	if err != nil {
		return err
	}
	for _, id := range deps {
		if combined.blocks != nil && combined.parsed.Artifact.State == "ready" && findProjectionRecord(combined.blocks["packages"], id) != nil {
			b.recordPackageJoin(combined, id, words("id"), "prerequisite_identity")
			continue
		}
		d, err := b.read(KindPackage, "step-04", "package", id, false, nil)
		if err != nil {
			return err
		}
		if d.blocks == nil || d.parsed.Artifact.State != "ready" {
			b.diagnostic(d, "", "prerequisite", "cannot resolve prerequisite package "+id, "error")
		} else {
			b.recordPackageJoin(d, id, words("id"), "prerequisite_identity")
		}
	}
	return nil
}

// Package mappings and prerequisite identities affect selection without being
// copied into the handoff. Record those joins alongside the projected leaves so
// every document that influenced selection is inspectable.
func (b *projectionBuilder) recordPackageJoin(d *projectionDocument, id string, fields []string, predicate string) {
	for i, value := range projectionArray(d.blocks["packages"]) {
		if projectionString(projectionObject(value)["id"]) != id {
			continue
		}
		pointer := "/blocks/packages/" + strconv.Itoa(i)
		source := &b.out.Sources[d.index]
		projectLeaves(value, fields, pointer, &source.SelectedPointers)
		source.Active = true
		source.Selections = append(source.Selections, ProjectionSelection{
			SourcePointer: pointer, Predicate: predicate,
			Fields: append([]string{}, fields...), SelectedIDs: []string{id},
		})
		return
	}
}

// applicability never treats empty or unknown linked scopes as global. Source
// validation separately rejects links which do not resolve in their catalogue.
func (b *projectionBuilder) applicable(row map[string]any) bool {
	if !b.context.filtered {
		return true
	}
	app := projectionObject(row["applies_to"])
	if app["scope"] == "global" {
		return true
	}
	if app["scope"] != "linked" {
		return false
	}
	for key, ids := range map[string]map[string]bool{"requirement_ids": b.context.requirements, "component_ids": b.context.components, "module_ids": b.context.modules, "contract_ids": b.context.contracts} {
		if intersectsProjectionIDs(app[key], ids) {
			return true
		}
	}
	return false
}

func (b *projectionBuilder) matches(predicate string, row map[string]any, d *projectionDocument) bool {
	id, c := projectionString(row["id"]), &b.context
	switch predicate {
	case "all", "batches", "command_keys":
		return true
	case "not_superseded":
		return !c.superseded[id]
	case "selected_package":
		return id == b.target.PackageID
	case "integration_points":
		return intersectsProjectionIDs(row["package_ids"], map[string]bool{b.target.PackageID: true})
	case "referenced_evidence":
		return d.sources[id]
	case "applicable_or_evidence":
		return b.applicable(row) || d.assumptions[id]
	case "applicable":
		return b.applicable(row)
	case "applicable_or_ancestor":
		if b.applicable(row) {
			return true
		}
		if b.target.Stage != "step-05" {
			return false
		}
		ancestor := strings.TrimSuffix(projectionString(row["path"]), "/")
		for _, file := range c.files {
			if ancestor == "." || file == ancestor || strings.HasPrefix(file, ancestor+"/") {
				return true
			}
		}
		return false
	case "requirements":
		return c.requirements[id]
	case "modules":
		return c.modules[id]
	case "boundary_modules":
		return c.boundaries[id]
	case "contracts":
		return c.contracts[id]
	case "decisions":
		if b.target.Stage == "step-05" {
			return c.decisions[id] || projectionObject(row["applies_to"])["scope"] == "global"
		}
		return b.applicable(row)
	case "module_links":
		return intersectsProjectionIDs(row["module_ids"], c.modules)
	case "contract_links":
		return intersectsProjectionIDs(row["contract_ids"], c.contracts)
	case "owners":
		return !c.filtered || intersectsProjectionIDs(row["owner_ids"], c.owners)
	case "test_modules":
		return c.modules[projectionString(row["module_id"])]
	case "coverage":
		return b.target.PackageID == "" || c.requirements[projectionString(row["requirement_id"])] || intersectsProjectionIDs(row["module_ids"], c.modules)
	}
	panic("unregistered projection predicate: " + predicate)
}

func (b *projectionBuilder) selectRow(row ProjectionField) {
	d := b.documents[row.SourceStage]
	if d == nil || d.blocks == nil {
		return
	}
	value, pointer, exists := projectionLookup(d.blocks, row.Source)
	if !exists || value == nil {
		if row.Required {
			b.diagnostic(d, pointer, "required_null", "required source "+row.Source+" is undetermined", "error")
		}
		return
	}
	selection := ProjectionSelection{SourcePointer: pointer, OutputPointer: "/" + strings.ReplaceAll(row.Output, ".", "/"), Predicate: row.Predicate, Fields: append([]string{}, row.Fields...), SelectedIDs: []string{}}
	var pointers []string
	var selected any
	if strings.HasSuffix(row.Source, "[]") {
		items := []any{}
		for i, v := range projectionArray(value) {
			r := projectionObject(v)
			if r != nil && !b.matches(row.Predicate, r, d) {
				continue
			}
			at := pointer + "/" + strconv.Itoa(i)
			projected := projectLeaves(v, row.Fields, at, &pointers)
			items = append(items, projected)
			if id := projectionString(r["id"]); id != "" {
				selection.SelectedIDs = append(selection.SelectedIDs, id)
				if row.Output == "requirements" || row.Output == "contracts" {
					encoded, _ := json.Marshal(projected)
					b.out.Sources[d.index].RecordHashes[row.Output+"/"+id] = hashBytes(encoded)
				}
			}
			collectProjectionEvidence(projected, d)
		}
		selected = items
		if len(items) == 0 {
			pointers = append(pointers, pointer)
		}
		if row.Predicate == "selected_package" {
			if len(items) != 1 {
				b.diagnostic(d, pointer, "target", "exactly one selected saved package is required", "error")
				return
			}
			selected = items[0]
		}
	} else {
		if row.Predicate == "command_keys" && b.target.Stage == "step-05" {
			row.Fields = nil
			for _, key := range words("build test lint run release") {
				if b.context.commands[key] {
					row.Fields = append(row.Fields, key)
				}
			}
			selection.Fields = append([]string{}, row.Fields...)
		}
		if row.Predicate == "command_keys" && b.target.Stage == "step-05" && len(row.Fields) == 0 {
			// An empty command selection is an empty object, never a wildcard.
			selected = map[string]any{}
			pointers = append(pointers, pointer)
		} else {
			selected = projectLeaves(value, row.Fields, pointer, &pointers)
		}
		if row.Predicate == "batches" && b.context.filtered {
			batches := []any{}
			allowed := map[string]bool{}
			for id := range b.context.modules {
				allowed[id] = true
			}
			for _, m := range projectionArray(d.blocks["modules"]) {
				if b.context.modules[projectionString(projectionObject(m)["id"])] {
					addProjectionIDs(allowed, projectionObject(m)["depends_on"])
				}
			}
			for _, batch := range projectionArray(value) {
				ids := []any{}
				for _, id := range projectionStrings(batch) {
					if allowed[id] {
						ids = append(ids, id)
					}
				}
				if len(ids) > 0 {
					batches = append(batches, ids)
				}
			}
			selected = batches
		}
		collectProjectionEvidence(selected, d)
	}
	b.out.Sources[d.index].Active = true
	b.out.Sources[d.index].Selections = append(b.out.Sources[d.index].Selections, selection)
	b.out.Sources[d.index].SelectedPointers = append(b.out.Sources[d.index].SelectedPointers, pointers...)
	b.mergeOutput(row.Output, selected, d)
}

func (b *projectionBuilder) mergeOutput(output string, value any, d *projectionDocument) {
	parts := strings.Split(output, ".")
	m := b.out.Data
	for _, part := range parts[:len(parts)-1] {
		if m[part] == nil {
			m[part] = map[string]any{}
		}
		m = projectionObject(m[part])
	}
	key := parts[len(parts)-1]
	previous, exists := m[key]
	if !exists {
		m[key] = value
		return
	}
	items, oldArray := previous.([]any)
	more, newArray := value.([]any)
	if oldArray && newArray {
		for _, item := range more {
			id := projectionString(projectionObject(item)["id"])
			duplicate := false
			for _, old := range items {
				if id != "" && projectionString(projectionObject(old)["id"]) == id {
					if !reflect.DeepEqual(old, item) {
						b.diagnostic(d, "/blocks/"+key, "conflict", "conflicting canonical "+key+" definitions for "+id, "error")
					}
					duplicate = true
					break
				}
				if reflect.DeepEqual(old, item) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				items = append(items, item)
			}
		}
		m[key] = items
	} else if !reflect.DeepEqual(previous, value) {
		b.diagnostic(d, "", "conflict", "conflicting projection output "+output, "error")
	}
}

func (b *projectionBuilder) checkFreshness() {
	design, pkg := b.documents["step-03"], b.documents["step-04"]
	if design == nil || pkg == nil {
		return
	}
	var baseline *AssemblySource
	for i := range b.target.Baseline {
		s := &b.target.Baseline[i]
		if s.Path == design.doc.Path && s.Hash != "" {
			baseline = s
			break
		}
	}
	if baseline != nil {
		if baseline.Hash == design.doc.Hash {
			return
		}
		b.diagnostic(design, "", "source_changed", "Stage 3 changed since packaging; selected requirements and declarations were checked against packaging provenance", "warning")
		current := b.out.Sources[design.index].RecordHashes
		for _, key := range sortedProjectionHashKeys(current) {
			if baseline.RecordHashes[key] == "" || baseline.RecordHashes[key] != current[key] {
				b.diagnostic(design, "/blocks/"+strings.SplitN(key, "/", 2)[0], "package_review_required", "selected "+key+" changed or lacks a packaging baseline; review and save the package against the effective Stage 3 design", "error")
			}
		}
	} else if !pkg.doc.ModTime.IsZero() && design.doc.ModTime.After(pkg.doc.ModTime) {
		b.diagnostic(design, "", "source_changed", "Stage 3 was saved after this package; packaging source hashes are unavailable", "warning")
		b.diagnostic(pkg, "", "package_review_required", "review and save the package against the newer effective Stage 3 design before generating a ready coding prompt", "error")
	}
}

func projectionLookup(blocks map[string]any, source string) (any, string, bool) {
	var value any = blocks
	pointer := "/blocks"
	for _, key := range strings.Split(source, ".") {
		key = strings.TrimSuffix(key, "[]")
		pointer += "/" + key
		m := projectionObject(value)
		var ok bool
		value, ok = m[key]
		if !ok {
			return nil, pointer, false
		}
	}
	return value, pointer, true
}

type projectionLeaf struct {
	terminal bool
	children map[string]*projectionLeaf
}

func projectLeaves(value any, fields []string, pointer string, pointers *[]string) any {
	root := &projectionLeaf{children: map[string]*projectionLeaf{}}
	for _, field := range fields {
		node := root
		for _, key := range strings.Split(field, ".") {
			key = strings.TrimSuffix(key, "[]")
			if node.children[key] == nil {
				node.children[key] = &projectionLeaf{children: map[string]*projectionLeaf{}}
			}
			node = node.children[key]
		}
		node.terminal = true
	}
	if len(fields) == 0 {
		root.terminal = true
	}
	return projectLeafTree(value, root, pointer, pointers)
}

func projectLeafTree(value any, node *projectionLeaf, pointer string, pointers *[]string) any {
	if value == nil || node.terminal {
		*pointers = append(*pointers, pointer)
		return value
	}
	if items, ok := value.([]any); ok {
		out := []any{}
		if len(items) == 0 {
			*pointers = append(*pointers, pointer)
		}
		for i, item := range items {
			out = append(out, projectLeafTree(item, node, pointer+"/"+strconv.Itoa(i), pointers))
		}
		return out
	}
	if object, ok := value.(map[string]any); ok {
		out := map[string]any{}
		keys := make([]string, 0, len(node.children))
		for key := range node.children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if v, exists := object[key]; exists {
				out[key] = projectLeafTree(v, node.children[key], pointer+"/"+key, pointers)
			}
		}
		return out
	}
	return value
}

func collectProjectionEvidence(value any, d *projectionDocument) {
	switch v := value.(type) {
	case map[string]any:
		if ids, ok := v["source_ids"]; ok {
			addProjectionIDs(d.sources, ids)
		}
		if ids, ok := v["assumption_ids"]; ok {
			addProjectionIDs(d.assumptions, ids)
		}
		for _, child := range v {
			collectProjectionEvidence(child, d)
		}
	case []any:
		for _, child := range v {
			collectProjectionEvidence(child, d)
		}
	}
}

func projectionObject(v any) map[string]any { m, _ := v.(map[string]any); return m }
func projectionArray(v any) []any           { a, _ := v.([]any); return a }
func projectionString(v any) string         { s, _ := v.(string); return s }
func projectionStrings(v any) []string {
	out := []string{}
	for _, id := range projectionArray(v) {
		out = append(out, projectionString(id))
	}
	return out
}
func addProjectionIDs(set map[string]bool, ids any) {
	for _, id := range projectionStrings(ids) {
		set[id] = true
	}
}
func intersectsProjectionIDs(ids any, set map[string]bool) bool {
	for _, id := range projectionStrings(ids) {
		if set[id] {
			return true
		}
	}
	return false
}
func sortedProjectionIDs(set map[string]bool) []string {
	ids := []string{}
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func sortedProjectionHashKeys(set map[string]string) []string {
	ids := []string{}
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func findProjectionRecord(collection any, id string) map[string]any {
	for _, v := range projectionArray(collection) {
		if row := projectionObject(v); projectionString(row["id"]) == id {
			return row
		}
	}
	return nil
}
func uniqueStrings(ss []string) []string {
	out := []string{}
	for _, s := range ss {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
