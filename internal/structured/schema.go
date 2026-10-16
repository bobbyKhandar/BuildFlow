package structured

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

func pointer(path, key string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
func keys(m map[string]any) []string {
	k := make([]string, 0, len(m))
	for s := range m {
		k = append(k, s)
	}
	sort.Strings(k)
	return k
}

// validateSchema interprets the vocabulary used by our embedded Draft 2020-12
// schemas. Semantic joins, graph checks, and readiness are handled separately.
// It is intentionally not an API for arbitrary user-supplied JSON schemas.
func validateSchema(value any, schema map[string]any, defs map[string]any, path string) []Diagnostic {
	var out []Diagnostic
	add := func(msg string) { out = append(out, Diagnostic{path, "schema", msg, "error"}) }
	if ref, ok := schema["$ref"].(string); ok {
		name := strings.TrimPrefix(strings.TrimPrefix(ref, "common.json"), "#/$defs/")
		def, ok := defs[name].(map[string]any)
		if !ok {
			add("unresolved schema reference " + ref)
			return out
		}
		return validateSchema(value, def, defs, path)
	}
	if want, ok := schema["const"]; ok && !reflect.DeepEqual(value, want) {
		add(fmt.Sprintf("expected %v", want))
	}
	if choices, ok := schema["enum"].([]any); ok {
		found := false
		for _, c := range choices {
			if reflect.DeepEqual(value, c) {
				found = true
			}
		}
		if !found {
			add("value is outside the allowed enum")
		}
	}
	if choices, ok := schema["anyOf"].([]any); ok {
		valid := false
		for _, c := range choices {
			if len(validateSchema(value, c.(map[string]any), defs, path)) == 0 {
				valid = true
				break
			}
		}
		if !valid {
			add("value has no allowed type")
		}
		return out
	}
	if all, ok := schema["allOf"].([]any); ok {
		for _, c := range all {
			out = append(out, validateSchema(value, c.(map[string]any), defs, path)...)
		}
	}
	if cond, ok := schema["if"].(map[string]any); ok {
		branch := "else"
		if len(validateSchema(value, cond, defs, path)) == 0 {
			branch = "then"
		}
		if b, ok := schema[branch].(map[string]any); ok {
			out = append(out, validateSchema(value, b, defs, path)...)
		}
	}
	typ, _ := schema["type"].(string)
	typeOK := true
	switch typ {
	case "null":
		typeOK = value == nil
	case "string":
		_, typeOK = value.(string)
	case "boolean":
		_, typeOK = value.(bool)
	case "number", "integer":
		_, typeOK = value.(float64)
	case "array":
		_, typeOK = value.([]any)
	case "object":
		_, typeOK = value.(map[string]any)
	}
	if !typeOK {
		add("expected " + typ)
		return out
	}
	if m, ok := value.(map[string]any); ok {
		props, _ := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, key := range required {
				k := key.(string)
				if _, ok := m[k]; !ok {
					out = append(out, Diagnostic{pointer(path, k), "schema", "required field is missing", "error"})
				}
			}
		}
		for _, k := range keys(m) {
			if s, ok := props[k].(map[string]any); ok {
				out = append(out, validateSchema(m[k], s, defs, pointer(path, k))...)
			} else if schema["additionalProperties"] == false {
				out = append(out, Diagnostic{pointer(path, k), "schema", "unknown field", "error"})
			}
		}
	}
	if a, ok := value.([]any); ok {

		if sub, ok := schema["contains"].(map[string]any); ok {
			found := false
			for _, item := range a {
				if len(validateSchema(item, sub, defs, path)) == 0 {
					found = true
					break
				}
			}
			if !found {
				add("array must contain a matching item")
			}
		}
		if n, ok := schema["minItems"].(float64); ok && len(a) < int(n) {
			add("too few items")
		}
		if n, ok := schema["maxItems"].(float64); ok && len(a) > int(n) {
			add("too many items")
		}
		if s, ok := schema["items"].(map[string]any); ok {
			for i, v := range a {
				out = append(out, validateSchema(v, s, defs, fmt.Sprintf("%s/%d", path, i))...)
			}
		}
	}
	if s, ok := value.(string); ok {
		if p, ok := schema["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(s) {
			add("string does not match " + p)
		}
		if n, ok := schema["minLength"].(float64); ok && len([]rune(s)) < int(n) {
			add("string is too short")
		}
	}
	return out
}
func rawObject(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
