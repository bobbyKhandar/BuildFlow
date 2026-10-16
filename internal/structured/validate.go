package structured

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
)

type record struct {
	value map[string]any
	path  string
}
type catalogue map[string]map[string]record

type validator struct {
	doc         *Envelope
	blocks      map[string]any
	local       catalogue
	upstream    catalogue
	available   map[string]bool
	diagnostics []Diagnostic
	warned      map[string]bool
}

func (v *validator) add(path, code, msg string) {
	v.diagnostics = append(v.diagnostics, Diagnostic{path, code, msg, "error"})
}
func (v *validator) warn(path, kind string) {
	if v.warned[kind] {
		return
	}
	v.warned[kind] = true
	v.diagnostics = append(v.diagnostics, Diagnostic{path, "reference_unchecked", "provide upstream " + kind + " catalogue to validate cross-stage links", "warning"})
}
func array(x any) []any           { a, _ := x.([]any); return a }
func object(x any) map[string]any { m, _ := x.(map[string]any); return m }
func text(x any) string           { s, _ := x.(string); return s }
func stringsOf(x any) []string {
	var ss []string
	for _, a := range array(x) {
		ss = append(ss, text(a))
	}
	return ss
}
func contains(ss []string, s string) bool {
	for _, a := range ss {
		if a == s {
			return true
		}
	}
	return false
}
func intersects(a, b []string) bool {
	for _, s := range a {
		if contains(b, s) {
			return true
		}
	}
	return false
}
func pathAt(path string, i int) string { return fmt.Sprintf("%s/%d", path, i) }

func recordPackagePath(path string) string {
	parts := strings.SplitN(path, "/", 5)
	if len(parts) == 5 && parts[0] == "" && parts[1] == "blocks" && parts[2] == "packages" && parts[3] != "" {
		return strings.Join(parts[:4], "/")
	}
	return ""
}

// Validate checks the schema as well as semantic links. References must be
// validated upstream artifacts, in any order. Local effective catalogues take
// precedence. No external files, URLs, or commands in the data are followed.
func Validate(doc *Envelope, references ...*Envelope) []Diagnostic {
	if doc == nil {
		return []Diagnostic{{"", "schema", "artifact is nil", "error"}}
	}
	target := Target{Stage: doc.Stage, Scope: doc.Scope}
	if doc.PackageID != nil {
		target.PackageID = *doc.PackageID
	}
	schema, err := Schema(target)
	if err != nil {
		return []Diagnostic{{"", "target", err.Error(), "error"}}
	}
	var s map[string]any
	_ = json.Unmarshal(schema, &s)
	ds := validateSchema(rawObject(doc), s, s["$defs"].(map[string]any), "")
	if hasErrors(ds) {
		return ds
	}
	v := &validator{doc: doc, blocks: rawObject(doc.Blocks), local: catalogue{}, upstream: catalogue{}, available: map[string]bool{}, warned: map[string]bool{}}
	// Pick effective records by stage, rather than caller ordering.
	for _, stage := range []string{"step-01", "step-02", "step-02-5", "step-03", "step-04"} {
		if stageRank(stage) > stageRank(doc.Stage) || (stage == doc.Stage && !(doc.Stage == "step-04" && doc.Scope == "package")) {
			continue
		}
		stageCatalogue := catalogue{}
		for _, ref := range references {
			if ref != nil && ref.Stage == stage && ref.SchemaVersion == SchemaVersion && (stage != doc.Stage || ref.Scope == "stage") {
				c := catalogue{}
				collectCatalogues(rawObject(ref.Blocks), "/blocks", c, nil)
				if ref.Stage == "step-03" {
					if ctx := object(rawObject(ref.Blocks)["implementation_context"]); ctx != nil {
						c["context"] = map[string]record{"effective": {ctx, "/blocks/implementation_context"}}
					}
				}
				for kind, records := range c {
					if stageCatalogue[kind] == nil {
						stageCatalogue[kind] = map[string]record{}
					}
					for id, row := range records {
						if old, ok := stageCatalogue[kind][id]; ok && !reflect.DeepEqual(old.value, row.value) {
							v.add(row.path, "conflict", "conflicting upstream "+stage+" "+kind+" definitions for "+id)
						} else {
							stageCatalogue[kind][id] = row
						}
					}
				}
			}
		}
		for kind, records := range stageCatalogue {
			v.upstream[kind] = records
			v.available[kind] = true
		}
	}
	seenPackageRecords := map[[3]string]bool{}
	collectCatalogues(v.blocks, "/blocks", v.local, func(kind, id, path string, old record, row map[string]any) {
		if doc.Stage == "step-04" && (kind == "acceptance" || kind == "test") {
			oldPackage, pkg := recordPackagePath(old.path), recordPackagePath(path)
			if oldPackage != "" && pkg != "" {
				seenPackageRecords[[3]string{kind, id, oldPackage}] = true
				key := [3]string{kind, id, pkg}
				repeated := seenPackageRecords[key]
				seenPackageRecords[key] = true
				equivalent := reflect.DeepEqual(old.value, row)
				// Criteria may narrow their links to each package, but both
				// copies must agree with the canonical Stage 3 criterion.
				if kind == "acceptance" {
					if source, ok := v.upstream[kind][id]; ok {
						equivalent = criterionCopy(old.value, source.value) && criterionCopy(row, source.value)
					}
				}
				if !repeated && equivalent {
					return
				}
			}
		}
		v.add(pointer(path, "id"), "duplicate_id", "duplicate "+kind+" ID "+id)
		if !reflect.DeepEqual(old.value, row) {
			v.add(path, "conflict", "conflicting definitions for "+kind+" ID "+id)
		}
	})
	for _, kind := range localKinds(doc.Stage) {
		v.available[kind] = true
		if v.local[kind] == nil {
			v.local[kind] = map[string]record{}
		}
	}
	v.walk(v.blocks, "/blocks", "")
	v.validateReadiness()
	v.validateEvidenceIdentity()
	switch doc.Stage {
	case "step-01":
		v.validateResearch()
	case "step-02":
		v.preserveIDs("requirement")
	case "step-02-5":
		v.preserveIDs("component")
		v.validateLibraries()
	case "step-03":
		v.validateDesign()
	case "step-04":
		v.validatePackages()
	case "step-05":
		v.validatePrompt()
	}
	sort.SliceStable(v.diagnostics, func(i, j int) bool {
		a, b := v.diagnostics[i], v.diagnostics[j]
		return a.Path+"\x00"+a.Code+"\x00"+a.Message < b.Path+"\x00"+b.Code+"\x00"+b.Message
	})
	return append(ds, v.diagnostics...)
}

func (v *validator) preserveIDs(kind string) {
	if v.doc.State != "ready" {
		return
	}
	for id := range v.upstream[kind] {
		if _, ok := v.local[kind][id]; !ok {
			v.add("/blocks", "identity", "upstream "+kind+" ID "+id+" was lost; preserve stable identities when carrying effective records forward")
		}
	}
}
func stageRank(s string) int {
	return map[string]int{"step-01": 1, "step-02": 2, "step-02-5": 3, "step-03": 4, "step-04": 5, "step-05": 6}[s]
}
func localKinds(stage string) []string {
	common := []string{"source", "assumption", "risk", "question"}
	switch stage {
	case "step-01":
		return append(common, "requirement", "constraint")
	case "step-02":
		return append(common, "requirement", "constraint", "component", "contract", "decision")
	case "step-02-5":
		return append(common, "component", "contract", "service")
	case "step-03":
		return append(common, "requirement", "constraint", "module", "contract", "decision", "test", "acceptance", "service")
	case "step-04":
		return append(common, "package", "test", "acceptance")
	case "step-05":
		return append(common, "contract", "acceptance")
	}
	return common
}

var collectionKinds = map[string]string{
	"requirements": "requirement", "constraints": "constraint", "components": "component", "modules": "module", "contracts": "contract", "interfaces": "contract", "interfaces_consumed": "contract", "interfaces_produced": "contract",
	"services": "service", "tests": "test", "acceptance": "acceptance", "packages": "package", "sources": "source", "assumptions": "assumption", "risks": "risk", "open_questions": "question", "decisions": "decision", "settled_decisions": "decision",
	"goals": "goal", "scenarios": "scenario", "integrations": "integration", "data_state": "data", "options": "option", "alternatives": "alternative", "evaluation_criteria": "criterion", "delivery_plan": "delivery", "stack": "stack", "tools": "tool", "libraries": "library", "invariants": "invariant", "algorithms": "algorithm", "error_cases": "error", "error_handling": "error", "concurrency": "concurrency", "state_machines": "state", "surfaces": "surface", "observability": "signal", "design_changes": "change",
	"prerequisites": "environment", "installation": "environment", "version_pinning": "environment", "clean_machine_setup": "environment",
}

func collectCatalogues(x any, path string, c catalogue, duplicate func(string, string, string, record, map[string]any)) {
	switch x := x.(type) {
	case map[string]any:
		for _, key := range keys(x) {
			value := x[key]
			p := pointer(path, key)
			if kind := collectionKinds[key]; kind != "" {
				if c[kind] == nil {
					c[kind] = map[string]record{}
				}
				for i, a := range array(value) {
					row := object(a)
					id := text(row["id"])
					if id == "" {
						continue
					}
					at := pathAt(p, i)
					if old, exists := c[kind][id]; exists && duplicate != nil {
						duplicate(kind, id, at, old, row)
					} else {
						c[kind][id] = record{row, at}
					}
				}
			}
			collectCatalogues(value, p, c, duplicate)
		}
	case []any:
		for i, a := range x {
			collectCatalogues(a, pathAt(path, i), c, duplicate)
		}
	}
}
func (v *validator) resolve(kind, id, path string) bool {
	if id == "" {
		v.add(path, "reference", "empty "+kind+" reference")
		return false
	}
	if kind == "package" && v.doc.Stage == "step-04" && v.doc.Scope == "package" {
		if _, ok := v.local[kind][id]; ok {
			return true
		}
		if records, known := v.upstream[kind]; known {
			if _, ok := records[id]; ok {
				return true
			}
			v.add(path, "reference", "unknown external package ID "+id)
			return false
		}
		v.warn(path, "external package")
		return true
	}
	records, local := v.local[kind]
	// Tests and acceptance may add package-specific entries to an upstream catalogue.
	if local {
		if _, ok := records[id]; ok {
			return true
		}
		if kind != "test" && kind != "acceptance" {
			v.add(path, "reference", "unknown "+kind+" ID "+id)
			return false
		}
	}
	if r, ok := v.upstream[kind]; ok {
		if _, ok = r[id]; ok {
			return true
		}
		v.add(path, "reference", "unknown "+kind+" ID "+id)
		return false
	}
	if v.available[kind] {
		v.add(path, "reference", "unknown "+kind+" ID "+id)
		return false
	}
	v.warn(path, kind)
	return true
}
func (v *validator) links(kind string, x any, path string) {
	seen := map[string]bool{}
	for i, id := range stringsOf(x) {
		p := pathAt(path, i)
		if seen[id] {
			v.add(p, "duplicate_reference", "duplicate "+kind+" reference "+id)
		}
		seen[id] = true
		v.resolve(kind, id, p)
	}
}

func (v *validator) upstreamLink(kind, id, path string) {
	if records, available := v.upstream[kind]; available {
		if _, ok := records[id]; !ok {
			v.add(path, "reference", "unknown upstream "+kind+" ID "+id)
		}
	} else {
		v.warn(path, kind)
	}
}

var referenceKinds = map[string]string{
	"requirement_ids": "requirement", "component_ids": "component", "module_ids": "module", "contract_ids": "contract", "referenced_contract_ids": "contract", "public_contract_ids": "contract", "consumed_contract_ids": "contract", "produced_contract_ids": "contract", "test_ids": "test", "source_test_ids": "test", "source_ids": "source", "assumption_ids": "assumption", "decision_ids": "decision", "source_acceptance_ids": "acceptance", "package_ids": "package",
	"requirement_id": "requirement", "from_component_id": "component", "to_component_id": "component", "module_id": "module",
}

func (v *validator) walk(x any, path, parent string) {
	switch x := x.(type) {
	case string:
		if v.doc.State == "ready" && strings.TrimSpace(x) == "" {
			v.add(path, "readiness", "substantive strings must not be empty in a ready artifact")
		}

	case map[string]any:
		if app, ok := x["applies_to"].(map[string]any); ok {
			total := 0
			for _, k := range []string{"requirement_ids", "component_ids", "module_ids", "contract_ids"} {
				total += len(array(app[k]))
			}
			if app["scope"] == "linked" && total == 0 {
				v.add(pointer(path, "applies_to"), "applicability", "linked applicability requires at least one valid link")
			}
		}
		if basis, ok := x["basis"].(map[string]any); ok {
			confidence := text(basis["confidence"])
			if (confidence == "assumption" || (confidence == "inferred" && parent == "requirements")) && len(array(basis["assumption_ids"])) == 0 {
				v.add(pointer(path, "basis"), "basis", "guessed requirements and assumption-based facts must link to an explicit assumption")
			}
		}
		if owner, ok := x["owner"].(map[string]any); ok {
			v.resolve(text(owner["kind"]), text(owner["id"]), pointer(pointer(path, "owner"), "id"))
		}
		if cfg, ok := x["configuration"].(map[string]any); ok && cfg["has_default"] == false && cfg["default_literal"] != nil {
			v.add(pointer(pointer(path, "configuration"), "default_literal"), "configuration", "a configuration without a default must use null default_literal")
		}
		for _, key := range keys(x) {
			value := x[key]
			p := pointer(path, key)
			if kind := referenceKinds[key]; kind != "" {
				if key == "source_acceptance_ids" {
					for i, id := range stringsOf(value) {
						v.upstreamLink("acceptance", id, pathAt(p, i))
					}
				} else if strings.HasSuffix(key, "_ids") {
					v.links(kind, value, p)
				} else {
					v.resolve(kind, text(value), p)
				}
			}
			if key == "owner_ids" {
				for i, id := range stringsOf(value) {
					if !v.hasOwner(id) {
						if v.available["module"] || v.available["service"] {
							v.add(pathAt(p, i), "reference", "unknown signal owner "+id)
						} else {
							v.warn(p, "module/service")
						}
					}
				}
			}
			v.walk(value, p, key)
		}
	case []any:
		for i, a := range x {
			v.walk(a, pathAt(path, i), parent)
		}
	}
}
func (v *validator) hasOwner(id string) bool {
	for _, kind := range []string{"module", "service", "component"} {
		if _, ok := v.local[kind][id]; ok {
			return true
		}
		if _, ok := v.upstream[kind][id]; ok {
			return true
		}
	}
	return false
}
func (v *validator) validateEvidenceIdentity() {
	for _, kind := range []string{"source", "assumption"} {
		for id, row := range v.local[kind] {
			if old, ok := v.upstream[kind][id]; ok && !reflect.DeepEqual(row.value, old.value) {
				v.add(pointer(row.path, "id"), "conflict", "carried "+kind+" ID "+id+" has a different definition")
			}
		}
	}
}
func (v *validator) validateReadiness() {
	blocking := false
	for _, q := range array(v.blocks["open_questions"]) {
		if object(q)["blocking"] == true {
			blocking = true
		}
	}
	if v.doc.State == "blocked" {
		if !blocking {
			v.add("/blocks/open_questions", "readiness", "blocked artifacts require a blocking question")
		}
		return
	}
	if blocking {
		v.add("/blocks/open_questions", "readiness", "ready artifacts cannot contain unresolved blocking questions")
	}
	required := map[string][]string{"step-01": {"goals", "requirements", "options", "evaluation_criteria"}, "step-02": {"requirements"}, "step-02-5": {}, "step-03": {"requirements", "modules", "test_plan", "acceptance", "requirement_coverage", "build_batches"}, "step-04": {"packages", "build_batches"}, "step-05": {"planned_checks", "acceptance", "implementation_instructions"}}
	for _, key := range required[v.doc.Stage] {
		if len(array(v.blocks[key])) == 0 {
			v.add(pointer("/blocks", key), "readiness", "ready artifact requires a substantive deliverable")
		}
	}
	nullable := map[string][]string{"step-02": {"decision", "design"}, "step-02-5": {"system_context"}, "step-03": {"implementation_context"}, "step-05": {"coding_prompt"}}
	for _, key := range nullable[v.doc.Stage] {
		if v.blocks[key] == nil {
			v.add(pointer("/blocks", key), "readiness", "ready artifact requires a determined value")
		}
	}
}
func (v *validator) validateResearch() {
	if v.doc.State != "ready" {
		return
	}
	n := len(array(v.blocks["options"]))
	if n < 3 || n > 6 {
		v.add("/blocks/options", "options", "ready research requires three to six unranked options")
	}
	total := 0.0
	for i, a := range array(v.blocks["evaluation_criteria"]) {
		w, _ := object(a)["weight_percent"].(float64)
		total += w
		if w < 0 || w > 100 {
			v.add(pathAt("/blocks/evaluation_criteria", i), "weights", "weight must be between zero and 100")
		}
	}
	if math.Abs(total-100) > 0.000001 {
		v.add("/blocks/evaluation_criteria", "weights", "criterion weights must total 100")
	}
	for _, r := range v.local["requirement"] {
		id := text(r.value["id"])
		if r.value["kind"] == "functional" && (!strings.HasPrefix(id, "FR-") || len(id) <= 3) {
			v.add(pointer(r.path, "id"), "requirement_id", "functional requirements must preserve FR-* identifiers")
		}
	}
}
func (v *validator) validateLibraries() {
	tech := object(v.blocks["technology"])
	for i, a := range array(tech["libraries"]) {
		m := object(a)
		if len(array(m["design_impact"])) == 0 || len(array(m["used_in"])) == 0 {
			v.add(pathAt("/blocks/technology/libraries", i), "library", "main libraries require a design impact and usage location")
		}
	}
}
func (v *validator) validateDesign() {
	modules := array(v.blocks["modules"])
	v.graph(modules, v.blocks["build_batches"], "/blocks", "module", false)
	if v.doc.State == "ready" {
		for id := range v.upstream["requirement"] {
			if _, ok := v.local["requirement"][id]; !ok {
				v.add("/blocks/requirements", "coverage", "effective design loses upstream requirement "+id+"; retain it with an explicit disposition")
			}
		}
	}
	coverage := map[string]bool{}
	for i, a := range array(v.blocks["requirement_coverage"]) {
		row := object(a)
		id := text(row["requirement_id"])
		p := pathAt("/blocks/requirement_coverage", i)
		if coverage[id] {
			v.add(p, "coverage", "requirement has multiple dispositions")
		}
		coverage[id] = true
		assigned := stringsOf(row["module_ids"])
		if row["disposition"] == "implemented" && len(assigned) == 0 {
			v.add(p, "coverage", "implemented requirement must be assigned to modules")
		}
		if row["disposition"] == "excluded" && (len(assigned) != 0 || strings.TrimSpace(text(row["reason"])) == "") {
			v.add(p, "coverage", "excluded requirement must give a reason and no implementing modules")
		}
		for _, mid := range assigned {
			if m, ok := v.local["module"][mid]; ok && !contains(stringsOf(m.value["requirement_ids"]), id) {
				v.add(p, "coverage", "coverage and module requirement assignments disagree")
			}
		}
	}
	if v.doc.State == "ready" {
		for id := range v.local["requirement"] {
			if !coverage[id] {
				v.add("/blocks/requirement_coverage", "coverage", "requirement "+id+" has no disposition")
			}
		}
	}
	for _, m := range v.local["module"] {
		for _, id := range stringsOf(m.value["requirement_ids"]) {
			if !coverage[id] {
				v.add(pointer(m.path, "requirement_ids"), "coverage", "assigned requirement has no coverage record")
			}
			for _, entry := range array(v.blocks["requirement_coverage"]) {
				c := object(entry)
				if c["requirement_id"] == id && (c["disposition"] != "implemented" || !contains(stringsOf(c["module_ids"]), text(m.value["id"]))) {
					v.add(pointer(m.path, "requirement_ids"), "coverage", "module assignment conflicts with requirement disposition")
				}
			}
		}
		if v.doc.State == "ready" && (len(array(m.value["component_ids"])) == 0 || len(array(m.value["requirement_ids"])) == 0) {
			v.add(m.path, "coverage", "ready modules require component and requirement links")
		}
	}
	for _, a := range v.local["acceptance"] {
		v.validateCriterion(a, nil, nil)
	}
	if v.doc.State == "ready" {
		for id := range v.local["module"] {
			found := false
			for _, a := range v.local["acceptance"] {
				if contains(stringsOf(a.value["module_ids"]), id) {
					found = true
				}
			}
			if !found {
				v.add("/blocks/acceptance", "acceptance", "module "+id+" has no acceptance criterion")
			}
		}
	}
}
func (v *validator) graph(rows []any, batches any, path, kind string, external bool) {
	byID := map[string]map[string]any{}
	for _, a := range rows {
		m := object(a)
		byID[text(m["id"])] = m
	}
	positions := map[string]int{}
	for i, b := range array(batches) {
		for j, id := range stringsOf(b) {
			p := fmt.Sprintf("%s/build_batches/%d/%d", path, i, j)
			if _, ok := byID[id]; !ok {
				v.add(p, "dependency", "build batch contains an unassigned "+kind+" "+id)
			}
			if _, ok := positions[id]; ok {
				v.add(p, "dependency", "duplicate build-batch entry "+id)
			}
			positions[id] = i
		}
	}
	state := map[string]int{}
	var visit func(string)
	visit = func(id string) {
		if state[id] == 1 {
			v.add(path, "dependency", "dependency cycle includes "+id)
			return
		}
		if state[id] == 2 {
			return
		}
		state[id] = 1
		for _, dep := range stringsOf(byID[id]["depends_on"]) {
			if _, ok := byID[dep]; !ok {
				if !external {
					v.add(path, "reference", "unknown dependency "+dep)
				}
				continue
			}
			visit(dep)
			if v.doc.State == "ready" {
				a, aok := positions[dep]
				b, bok := positions[id]
				if aok && bok && a >= b {
					v.add(path+"/build_batches", "dependency", dep+" must precede "+id)
				}
			}
		}
		state[id] = 2
	}
	for _, id := range keysForRecords(byID) {
		visit(id)
		if _, ok := positions[id]; !ok && v.doc.State == "ready" {
			v.add(path+"/build_batches", "dependency", "build order omits "+id)
		}
	}
}
func keysForRecords(m map[string]map[string]any) []string {
	a := map[string]any{}
	for k := range m {
		a[k] = nil
	}
	return keys(a)
}
func (v *validator) validateCriterion(a record, requirements, modules []string) {
	row := a.value
	rids := stringsOf(row["requirement_ids"])
	mids := stringsOf(row["module_ids"])
	if len(rids) == 0 || len(mids) == 0 || len(array(row["test_ids"])) == 0 {
		v.add(a.path, "acceptance", "acceptance requires explicit requirement, module, and test links")
	}
	if requirements != nil {
		for _, id := range rids {
			if !contains(requirements, id) {
				v.add(a.path, "acceptance", "criterion references a requirement outside the package")
			}
		}
	}
	if modules != nil {
		for _, id := range mids {
			if !contains(modules, id) {
				v.add(a.path, "acceptance", "criterion references a module outside the package")
			}
		}
	}
	for _, mid := range mids {
		m, ok := v.local["module"][mid]
		if !ok {
			m, ok = v.upstream["module"][mid]
		}
		if ok && !intersects(rids, stringsOf(m.value["requirement_ids"])) {
			v.add(a.path, "acceptance", "criterion is not attributable to the linked module requirements")
		}
	}
	for _, tid := range stringsOf(row["test_ids"]) {
		test, ok := v.local["test"][tid]
		if !ok {
			test, ok = v.upstream["test"][tid]
		}
		if ok && !intersects(rids, stringsOf(test.value["requirement_ids"])) {
			v.add(a.path, "acceptance", "criterion test "+tid+" does not cover its linked requirements")
		}
		if v.doc.Stage == "step-03" {
			for _, entry := range array(v.blocks["test_plan"]) {
				plan := object(entry)
				for _, t := range array(plan["tests"]) {
					if object(t)["id"] == tid && !contains(mids, text(plan["module_id"])) {
						v.add(a.path, "acceptance", "criterion test belongs to an unrelated module")
					}
				}
			}
		}
	}
}
func (v *validator) validatePackages() {
	packages := array(v.blocks["packages"])
	assigned := map[string]string{}
	for i, a := range packages {
		p := object(a)
		for _, mid := range stringsOf(p["module_ids"]) {
			if previous, ok := assigned[mid]; ok {
				v.add(pathAt("/blocks/packages", i), "coverage", "module "+mid+" is assigned to both "+previous+" and "+text(p["id"]))
			}
			assigned[mid] = text(p["id"])
		}
	}
	if v.doc.State == "ready" && v.doc.Scope == "stage" {
		for mid := range v.upstream["module"] {
			if _, ok := assigned[mid]; !ok {
				v.add("/blocks/packages", "coverage", "combined package mapping omits module "+mid)
			}
		}
	}
	if v.doc.Scope == "package" && (len(packages) != 1 || text(object(packages[0])["id"]) != *v.doc.PackageID) {
		v.add("/blocks/packages", "target", "package scope requires exactly the selected package")
	}
	v.graph(packages, v.blocks["build_batches"], "/blocks", "package", v.doc.Scope == "package")
	for i, a := range packages {
		p := object(a)
		path := pathAt("/blocks/packages", i)
		id := text(p["id"])
		if !packageIDPattern.MatchString(id) {
			v.add(pointer(path, "id"), "target", "package IDs must be normalized, for example P01")
		}
		rids, mids := stringsOf(p["requirement_ids"]), stringsOf(p["module_ids"])
		if v.doc.State == "ready" && (len(rids) == 0 || len(mids) == 0 || len(array(p["tests"])) == 0 || len(array(p["acceptance"])) == 0) {
			v.add(path, "readiness", "ready package requires requirements, modules, tests, and acceptance")
		}
		deps := stringsOf(p["depends_on"])
		seen := map[string]bool{}
		for j, a := range array(p["prerequisite_reasons"]) {
			reason := object(a)
			dep := text(reason["package_id"])
			seen[dep] = true
			if !contains(deps, dep) {
				v.add(pathAt(pointer(path, "prerequisite_reasons"), j), "dependency", "reason is not for a declared prerequisite")
			}
		}
		for _, dep := range deps {
			if !packageIDPattern.MatchString(dep) {
				v.add(pointer(path, "depends_on"), "target", "prerequisite package ID is not normalized")
			}
			if !seen[dep] {
				v.add(pointer(path, "prerequisite_reasons"), "dependency", "missing reason for prerequisite "+dep)
			}
			if v.doc.Scope == "package" {
				if _, known := v.local["package"][dep]; !known {
					if records, available := v.upstream["package"]; available {
						if _, ok := records[dep]; !ok {
							v.add(pointer(path, "depends_on"), "reference", "unknown prerequisite package "+dep)
						}
					} else {
						v.warn(pointer(path, "depends_on"), "external package")
					}
				}
			}
		}
		impl := object(p["implementation"])
		contracts := append(stringsOf(p["consumed_contract_ids"]), stringsOf(p["produced_contract_ids"])...)
		contracts = append(contracts, stringsOf(impl["contract_ids"])...)
		contracts = v.contractClosure(contracts)
		v.packageLinks(p, path, rids, mids, contracts)
		copied := stringsOf(p["source_acceptance_ids"])
		actual := map[string]bool{}
		for j, a := range array(p["acceptance"]) {
			row := object(a)
			at := pathAt(pointer(path, "acceptance"), j)
			aid := text(row["id"])
			if actual[aid] {
				v.add(at, "duplicate_id", "duplicate acceptance criterion ID "+aid+" within package")
			}
			actual[aid] = true
			v.validateCriterion(record{row, at}, rids, mids)
			if old, ok := v.upstream["acceptance"][aid]; ok && !criterionCopy(row, old.value) {
				v.add(at, "conflict", "copied criterion "+aid+" differs from the effective Stage 3 criterion")
			}
			for _, tid := range stringsOf(row["test_ids"]) {
				found := false
				for _, t := range array(p["tests"]) {
					if object(t)["id"] == tid {
						found = true
					}
				}
				if !found {
					v.add(at, "acceptance", "package must carry criterion test "+tid)
				}
			}
		}
		for _, aid := range copied {
			if !actual[aid] {
				v.add(pointer(path, "acceptance"), "acceptance", "source criterion "+aid+" is not carried by the package")
			}
		}
		for aid, a := range v.upstream["acceptance"] {
			if intersects(mids, stringsOf(a.value["module_ids"])) && intersects(rids, stringsOf(a.value["requirement_ids"])) {
				if !contains(copied, aid) || !actual[aid] {
					v.add(pointer(path, "source_acceptance_ids"), "acceptance", "package must carry applicable Stage 3 criterion "+aid)
				}
			}
		}
		for _, mid := range mids {
			if m, ok := v.upstream["module"][mid]; ok {
				for _, rid := range stringsOf(m.value["requirement_ids"]) {
					if !contains(rids, rid) {
						v.add(pointer(path, "requirement_ids"), "coverage", "package omits assigned module requirement "+rid)
					}
				}
				for _, key := range []string{"consumed_contract_ids", "produced_contract_ids"} {
					for _, cid := range stringsOf(m.value[key]) {
						if !contains(stringsOf(p[key]), cid) {
							v.add(pointer(path, key), "coverage", "package omits assigned module contract "+cid)
						}
					}
				}
			}
		}
		v.comparePackageCopies(p, path)
		v.checkPackageCommands(p, path)
	}
}
func (v *validator) contractClosure(ids []string) []string {
	out := []string{}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
		r, ok := v.local["contract"][id]
		if !ok {
			r, ok = v.upstream["contract"][id]
		}
		if ok {
			for _, dep := range stringsOf(r.value["referenced_contract_ids"]) {
				visit(dep)
			}
		}
	}
	for _, id := range ids {
		visit(id)
	}
	return out
}
func (v *validator) packageLinks(x any, path string, requirements, modules, contracts []string) {
	switch x := x.(type) {
	case map[string]any:
		for _, k := range keys(x) {
			value := x[k]
			var allowed []string
			switch k {
			case "requirement_ids":
				allowed = requirements
			case "module_ids":
				allowed = modules
			case "contract_ids", "consumed_contract_ids", "produced_contract_ids":
				allowed = contracts
			}
			if allowed != nil {
				for _, id := range stringsOf(value) {
					if !contains(allowed, id) {
						v.add(pointer(path, k), "package_scope", "package detail references unrelated ID "+id)
					}
				}
			}
			v.packageLinks(value, pointer(path, k), requirements, modules, contracts)
		}
	case []any:
		for i, a := range x {
			v.packageLinks(a, pathAt(path, i), requirements, modules, contracts)
		}
	}
}
func (v *validator) checkPackageCommands(p map[string]any, path string) {
	if v.doc.State != "ready" {
		return
	}
	keys := stringsOf(p["command_keys"])
	build := contains(keys, "build")
	test := contains(keys, "test")
	commands := v.upstreamCommands()
	for _, t := range array(p["tests"]) {
		m := object(t)
		if text(m["command"]) != "" {
			test = true
			if commands != nil && m["command"] == commands["build"] {
				build = true
			}
		} else if m["required_for_acceptance"] == true && !contains(keys, "test") {
			v.add(pointer(path, "tests"), "commands", "required acceptance test needs an explicit command or the selected test command")
		}
	}
	if !build || !test {
		v.add(pointer(path, "command_keys"), "commands", "ready package requires build and test checks through selected commands or explicit test commands")
	}
	// Resolve keys against the authoritative Stage 3 operational context.
	if commands != nil {
		for _, key := range keys {
			if strings.TrimSpace(text(commands[key])) == "" {
				v.add(pointer(path, "command_keys"), "commands", "essential command "+key+" is undetermined; report a blocker")
			}
		}
	}
}
func (v *validator) upstreamCommands() map[string]any {
	if c, ok := v.upstream["context"]["effective"]; ok {
		return object(object(c.value["technology"])["commands"])
	}
	return nil
}
func (v *validator) validatePrompt() {
	target := object(v.blocks["target"])
	if v.doc.PackageID == nil || text(target["id"]) != *v.doc.PackageID {
		v.add("/blocks/target/id", "target", "coding prompt target does not match selected package")
	}
	if v.doc.State == "blocked" {
		if v.blocks["coding_prompt"] != nil || len(array(v.blocks["blockers"])) == 0 {
			v.add("/blocks/coding_prompt", "readiness", "blocked Stage 5 requires blockers and a null coding_prompt")
		}
		return
	}
	if strings.TrimSpace(text(v.blocks["coding_prompt"])) == "" || len(array(v.blocks["blockers"])) != 0 {
		v.add("/blocks/coding_prompt", "readiness", "ready Stage 5 requires a nonempty coding prompt and no blockers")
	}
	for _, a := range v.local["acceptance"] {
		v.validateCriterion(a, nil, nil)
	}
	// Body copies may not override the effective catalogue.
	for id, c := range v.local["contract"] {
		if old, ok := v.upstream["contract"][id]; ok {
			if !reflect.DeepEqual(c.value, old.value) {
				v.add(c.path, "conflict", "contract differs from effective Stage 3; update the design and review the package")
			}
		} else if v.available["module"] {
			v.add(c.path, "reference", "contract "+id+" is absent from Stage 3; update the effective design first")
		}
	}
	if p, ok := v.upstream["package"][text(target["id"])]; ok {
		rids, mids := stringsOf(p.value["requirement_ids"]), stringsOf(p.value["module_ids"])
		for _, a := range v.local["acceptance"] {
			v.validateCriterion(a, rids, mids)
		}
		for _, a := range array(p.value["acceptance"]) {
			id := text(object(a)["id"])
			if got, ok := v.local["acceptance"][id]; !ok || !reflect.DeepEqual(got.value, a) {
				v.add("/blocks/acceptance", "acceptance", "coding prompt must retain the selected package criterion "+id)
			}
		}
		ids := append(stringsOf(p.value["consumed_contract_ids"]), stringsOf(p.value["produced_contract_ids"])...)
		ids = append(ids, stringsOf(object(p.value["implementation"])["contract_ids"])...)
		closure := v.contractClosure(ids)
		for _, id := range closure {
			if _, ok := v.local["contract"][id]; !ok {
				v.add("/blocks/interfaces_consumed", "reference", "standalone coding prompt omits required declaration "+id)
			}
		}
		for id, c := range v.local["contract"] {
			if !contains(closure, id) {
				v.add(c.path, "package_scope", "coding prompt includes unrelated contract "+id)
			}
		}
	} else if v.available["package"] {
		v.add("/blocks/target/id", "reference", "selected package is absent from the supplied package catalogue")
	} else {
		v.warn("/blocks/target/id", "package")
	}
}

// A copied criterion keeps its canonical identity and statement. Links may be
// narrowed to the package's assigned work, never reassigned to unrelated IDs.
func criterionCopy(copy, source map[string]any) bool {
	if copy["id"] != source["id"] || copy["statement"] != source["statement"] {
		return false
	}
	for _, k := range []string{"requirement_ids", "module_ids", "test_ids"} {
		for _, id := range stringsOf(copy[k]) {
			if !contains(stringsOf(source[k]), id) {
				return false
			}
		}
	}
	return true
}
func (v *validator) comparePackageCopies(p map[string]any, path string) {
	c := catalogue{}
	collectCatalogues(p, path, c, nil)
	for _, kind := range []string{"test", "algorithm", "error", "concurrency", "state", "surface", "signal"} {
		for id, row := range c[kind] {
			if source, ok := v.upstream[kind][id]; ok && !reflect.DeepEqual(source.value, row.value) {
				v.add(row.path, "conflict", "package copy differs from the effective Stage 3 "+kind+" "+id)
			}
		}
	}
}
