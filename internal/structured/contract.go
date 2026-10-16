package structured

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed schemas/*.json
var schemaFiles embed.FS
var packageIDPattern = regexp.MustCompile(`^P(?:0[1-9]|[1-9][0-9]{1,2})$`)
var schemaOnce sync.Once
var sharedDefinitions map[string]any
var stageSchemas map[string]map[string]any
var schemaLoadError error

func loadSchemas() {
	var common map[string]any
	b, err := schemaFiles.ReadFile("schemas/common.json")
	if err == nil {
		err = json.Unmarshal(b, &common)
	}
	if err != nil {
		schemaLoadError = err
		return
	}
	sharedDefinitions = common["$defs"].(map[string]any)
	stageSchemas = map[string]map[string]any{}
	for _, stage := range []string{"step-01", "step-02", "step-02-5", "step-03", "step-04", "step-05"} {
		b, err = schemaFiles.ReadFile("schemas/" + stage + ".json")
		if err != nil {
			schemaLoadError = err
			return
		}
		var s map[string]any
		if err = json.Unmarshal(b, &s); err != nil {
			schemaLoadError = err
			return
		}
		stageSchemas[stage] = s
	}
}
func checkedTarget(t Target) (Target, error) {
	if blockValue(t.Stage) == nil {
		return t, fmt.Errorf("unknown stage %q", t.Stage)
	}
	if t.Scope == "" {
		t.Scope = "stage"
		if t.Stage == "step-05" {
			t.Scope = "package"
		}
	}
	if t.Scope != "stage" && t.Scope != "package" {
		return t, fmt.Errorf("unknown scope %q", t.Scope)
	}
	if t.Scope == "package" && t.Stage != "step-04" && t.Stage != "step-05" {
		return t, fmt.Errorf("%s requires stage scope", t.Stage)
	}
	if t.Stage == "step-05" && t.Scope != "package" {
		return t, fmt.Errorf("Stage 5 requires package scope")
	}
	if t.Scope == "package" && !packageIDPattern.MatchString(t.PackageID) {
		return t, fmt.Errorf("package target requires a normalized ID such as P01")
	}
	if t.Scope == "stage" && t.PackageID != "" {
		return t, fmt.Errorf("stage target cannot select a package")
	}
	return t, nil
}

// Schema returns the complete, compact Draft 2020-12 output schema, with local
// definitions included once and scope/package identity fixed to the target.
func Schema(target Target) ([]byte, error) {
	target, err := checkedTarget(target)
	if err != nil {
		return nil, err
	}
	schemaOnce.Do(loadSchemas)
	if schemaLoadError != nil {
		return nil, schemaLoadError
	}
	// Clone: specialization must not mutate the shared embedded schema.
	b, _ := json.Marshal(stageSchemas[target.Stage])
	var s map[string]any
	_ = json.Unmarshal(b, &s)
	p := s["properties"].(map[string]any)
	p["scope"] = map[string]any{"const": target.Scope}
	var id any
	if target.Scope == "package" {
		id = target.PackageID
	}
	p["package_id"] = map[string]any{"const": id}
	if target.Stage == "step-04" && target.Scope == "package" {
		s["allOf"] = append(s["allOf"].([]any), map[string]any{"properties": map[string]any{"blocks": map[string]any{"properties": map[string]any{"packages": map[string]any{"minItems": 1, "maxItems": 1, "items": map[string]any{"properties": map[string]any{"id": map[string]any{"const": target.PackageID}}}}}}}})
	}
	if target.Stage == "step-05" {
		s["allOf"] = append(s["allOf"].([]any), map[string]any{"properties": map[string]any{"blocks": map[string]any{"properties": map[string]any{"target": map[string]any{"properties": map[string]any{"id": map[string]any{"const": target.PackageID}}}}}}})
	}
	// Include only reachable definitions, keeping the output contract compact.
	used := map[string]any{}
	var collect func(any)
	collect = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, a := range v {
				if k == "$ref" {
					name := strings.TrimPrefix(strings.TrimPrefix(a.(string), "common.json"), "#/$defs/")
					if _, ok := used[name]; !ok {
						used[name] = sharedDefinitions[name]
						collect(sharedDefinitions[name])
					}
				} else {
					collect(a)
				}
			}
		case []any:
			for _, a := range v {
				collect(a)
			}
		}
	}
	collect(s)
	s["$defs"] = used
	b, err = json.Marshal(s)
	return []byte(strings.ReplaceAll(string(b), "common.json#/$defs/", "#/$defs/")), err
}

// Contract is static protocol text plus the full target schema. It performs no
// model calls and never resolves or executes declarations supplied by a model.
func Contract(target Target) (string, error) {
	b, err := Schema(target)
	if err != nil {
		return "", err
	}
	return "Write the complete Markdown answer first. Append exactly one ## BuildFlow Structured View section containing <!-- buildflow:view:start -->, one fenced json block, and <!-- buildflow:view:end -->, each on its own line. End after the closing marker.\n" +
		"Match this schema and target exactly. Do not add a summary field. Include every required field; use empty arrays for no items and null only where permitted. Keep Markdown and JSON consistent; preserve complete contracts, rules, commands, and implementation details. Distinguish verified facts, inferences, and assumptions; do not invent sources or executed work. If prerequisites block this stage, set state to blocked and record blocking questions. Stage 5 produces only a standalone coding prompt and planned checks, never implementation or execution reports. Contracts introduced during Stage 4 must first be recorded in the effective Stage 3 design.\n\n```json\n" + string(b) + "\n```", nil
}
