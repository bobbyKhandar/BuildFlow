package structured

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestContractSpecializesPackageAndResolvesDefinitions(t *testing.T) {
	for _, stage := range []string{"step-01", "step-02", "step-02-5", "step-03", "step-04", "step-05"} {
		target := Target{Stage: stage, Scope: "stage"}
		if stage == "step-05" {
			target.Scope = "package"
			target.PackageID = "P01"
		}
		b, err := Schema(target)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err = json.Unmarshal(b, &schema); err != nil {
			t.Fatal(err)
		}
		value := fixtureJSON(t, stage)
		if ds := validateSchema(value, schema, schema["$defs"].(map[string]any), ""); len(ds) != 0 {
			t.Fatalf("%s: %+v", stage, ds)
		}
		if strings.Contains(string(b), "common.json") || strings.Contains(string(b), "\n") {
			t.Fatal("schema must have only local refs and compact rendering")
		}
		contract, err := Contract(target)
		if err != nil || !strings.Contains(contract, string(b)) {
			t.Fatalf("incomplete output contract: %v", err)
		}
	}
	b, err := Schema(Target{Stage: "step-04", Scope: "package", PackageID: "P01"})
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	_ = json.Unmarshal(b, &schema)
	v := fixtureJSON(t, "step-04-package")
	v["blocks"].(map[string]any)["packages"].([]any)[0].(map[string]any)["id"] = "P02"
	if len(validateSchema(v, schema, schema["$defs"].(map[string]any), "")) == 0 {
		t.Fatal("schema accepted a package body with the wrong ID")
	}
	for _, target := range []Target{{Stage: "step-99"}, {Stage: "step-05", Scope: "stage"}, {Stage: "step-04", Scope: "package", PackageID: "p1"}, {Stage: "step-01", Scope: "package", PackageID: "P01"}, {Stage: "step-04", Scope: "stage", PackageID: "P01"}} {
		if _, err := Contract(target); err == nil {
			t.Fatalf("invalid target accepted: %+v", target)
		}
	}
}

// This catches schema/Go drift independently of the JSON-schema interpreter:
// fixtures must round-trip through every concrete Go type without losing a
// field, changing a null, or turning [] into null.
func TestSchemaAndGoTypesAgree(t *testing.T) {
	for _, stage := range []string{"step-01", "step-02", "step-02-5", "step-03", "step-04", "step-05"} {
		v := fixtureJSON(t, stage)
		b, _ := json.Marshal(v["blocks"])
		typed := blockValue(stage)
		if err := json.Unmarshal(b, typed); err != nil {
			t.Fatal(err)
		}
		actual := rawObject(typed)
		if !reflect.DeepEqual(v["blocks"], actual) {
			t.Fatalf("%s schema/Go fields differ", stage)
		}
		s, err := Schema(Target{Stage: stage, PackageID: map[bool]string{true: "P01"}[stage == "step-05"]})
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]any
		_ = json.Unmarshal(s, &root)
		definitions := root["$defs"].(map[string]any)
		var check func(reflect.Type, map[string]any, string)
		check = func(typ reflect.Type, schema map[string]any, path string) {
			if typ.Kind() == reflect.Pointer {
				typ = typ.Elem()
				if choices, ok := schema["anyOf"].([]any); ok {
					for _, c := range choices {
						if object(c)["type"] != "null" {
							schema = object(c)
							break
						}
					}
				}
			}
			if ref, ok := schema["$ref"].(string); ok {
				schema = object(definitions[strings.TrimPrefix(ref, "#/$defs/")])
			}
			if typ.Kind() == reflect.Struct {
				props := object(schema["properties"])
				if len(props) != typ.NumField() {
					t.Fatalf("%s: field count differs", path)
				}
				for i := 0; i < typ.NumField(); i++ {
					f := typ.Field(i)
					name := f.Tag.Get("json")
					s, ok := props[name].(map[string]any)
					if !ok {
						t.Fatalf("%s/%s absent in schema", path, name)
					}
					check(f.Type, s, path+"/"+name)
				}
			}
			if typ.Kind() == reflect.Slice {
				check(typ.Elem(), object(schema["items"]), path+"/*")
			}
		}
		check(reflect.TypeOf(typed), object(object(root["properties"])["blocks"]), stage)
	}
}

func TestConcurrentContractSpecialization(t *testing.T) {
	var wg sync.WaitGroup
	for _, id := range []string{"P01", "P02", "P03"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				s, err := Schema(Target{Stage: "step-05", Scope: "package", PackageID: id})
				if err != nil || !strings.Contains(string(s), `"id":{"const":"`+id+`"}`) {
					t.Errorf("specialization leaked across calls: %s %v", id, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestOutputSchemaRejectsMissingDeliverables(t *testing.T) {
	for _, tt := range []struct {
		stage, field string
		value        any
	}{{"step-01", "requirements", []any{}}, {"step-02", "design", nil}, {"step-03", "implementation_context", nil}, {"step-04", "packages", []any{}}, {"step-05", "coding_prompt", nil}} {
		target := Target{Stage: tt.stage}
		if tt.stage == "step-05" {
			target.PackageID = "P01"
		}
		b, err := Schema(target)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		_ = json.Unmarshal(b, &schema)
		v := fixtureJSON(t, tt.stage)
		object(v["blocks"])[tt.field] = tt.value
		if ds := validateSchema(v, schema, object(schema["$defs"]), ""); len(ds) == 0 {
			t.Errorf("%s output schema accepts ready artifact missing %s", tt.stage, tt.field)
		}
	}
}
