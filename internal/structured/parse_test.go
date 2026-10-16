package structured

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".md")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func footer(raw string) string {
	return "## BuildFlow Structured View\n\n<!-- buildflow:view:start -->\n```json\n" + raw + "\n```\n<!-- buildflow:view:end -->\n"
}
func fixtureJSON(t *testing.T, name string) map[string]any {
	t.Helper()
	s := fixture(t, name)
	a := strings.Index(s, "```json\n") + 8
	b := strings.LastIndex(s, "\n```")
	var v map[string]any
	if err := json.Unmarshal([]byte(s[a:b]), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func encoded(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return footer(string(b))
}
func hasCode(r Result, code string) bool {
	for _, d := range r.Diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestParseFooterBoundaries(t *testing.T) {
	s := fixture(t, "step-01")
	for _, newline := range []string{"\n", "\r\n"} {
		content := strings.ReplaceAll(s, "\n", newline)
		r := Parse(content, Target{Stage: "step-01", Scope: "stage"})
		if r.Status != Valid {
			t.Fatalf("%q: %+v", newline, r.Diagnostics)
		}
		if r.Artifact == nil || r.Blocks == nil {
			t.Fatal("missing validated data")
		}
		if r.NarrativeEnd <= 0 || content[:r.NarrativeEnd] != strings.Split(content, "## BuildFlow Structured View")[0] {
			t.Fatal("incorrect original byte bounds")
		}
		if r.FooterEnd != len(content) {
			t.Fatal("incorrect footer end")
		}
	}
}
func TestParseRejectsUntrustedMetadata(t *testing.T) {
	s := fixture(t, "step-01")
	cases := []struct {
		name, content, code string
		target              Target
		status              Status
	}{
		{"legacy", "# Old answer\nKeep all of this.\n", "", Target{Stage: "step-01", Scope: "stage"}, Missing},
		{"malformed", footer("{broken}"), "json", Target{Stage: "step-01", Scope: "stage"}, Invalid},
		{"truncated", strings.Replace(s, "<!-- buildflow:view:end -->", "", 1), "footer", Target{Stage: "step-01", Scope: "stage"}, Invalid},
		{"duplicate key", strings.Replace(s, `"schema_version": 2`, `"schema_version": 2, "schema_version": 2`, 1), "duplicate_key", Target{Stage: "step-01", Scope: "stage"}, Invalid},
		{"trailing value", strings.Replace(s, "\n```\n<!-- buildflow:view:end", " {}\n```\n<!-- buildflow:view:end", 1), "json", Target{Stage: "step-01", Scope: "stage"}, Invalid},
		{"wrong stage", s, "target", Target{Stage: "step-02", Scope: "stage"}, Invalid},
		{"ambiguous", s + s, "footer", Target{Stage: "step-01", Scope: "stage"}, Invalid},
		{"after footer", s + "Unbounded prose\n", "footer", Target{Stage: "step-01", Scope: "stage"}, Invalid},
		{"unknown version", strings.Replace(s, `"schema_version": 2`, `"schema_version": 99`, 1), "version", Target{Stage: "step-01", Scope: "stage"}, UnsupportedVersion},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := Parse(tt.content, tt.target)
			if r.Status != tt.status || (tt.code != "" && !hasCode(r, tt.code)) {
				t.Fatalf("%s: %+v", r.Status, r.Diagnostics)
			}
			if r.Status != Valid && (r.Artifact != nil || r.Blocks != nil) {
				t.Fatal("partially trusted invalid document")
			}
			if tt.name == "truncated" && r.NarrativeEnd != len(tt.content) {
				t.Fatal("discarded uncertain text")
			}
		})
	}
}
func TestFooterMarkersInsideExamplesAreNotMetadata(t *testing.T) {
	s := fixture(t, "step-01")
	for _, fence := range []string{"````", "~~~~"} {
		example := fence + "markdown\n" + s + fence + "\n"
		if r := Parse(example, Target{Stage: "step-01", Scope: "stage"}); r.Status != Missing {
			t.Fatalf("example trusted: %+v", r)
		}
		content := example + "\n" + s
		if r := Parse(content, Target{Stage: "step-01", Scope: "stage"}); r.Status != Valid {
			t.Fatal(r.Diagnostics)
		}
	}
}

func TestLegacyResearchAdapterUsesOnlyExistingRelationships(t *testing.T) {
	v := fixtureJSON(t, "step-01")
	v["schema_version"] = float64(1)
	object(v["blocks"])["constraints"] = []any{}
	r := Parse(encoded(t, v), Target{Stage: "step-01", Scope: "stage"})
	if r.Status != Valid || !r.Adapted || !hasCode(r, "legacy_adapter") || r.SourceSchemaVersion != 1 {
		t.Fatalf("safe version-1 research was not adapted: %+v", r)
	}
	// Legacy scoped records do not specify global/linked applicability. Do not
	// silently add it, even when an English statement sounds project-wide.
	v = fixtureJSON(t, "step-01")
	v["schema_version"] = float64(1)
	delete(object(object(v["blocks"])["constraints"].([]any)[0]), "applies_to")
	r = Parse(encoded(t, v), Target{Stage: "step-01", Scope: "stage"})
	if r.Status != UnsupportedVersion || r.Artifact != nil {
		t.Fatal(r)
	}
}

func TestEdgeCaseFixtureCorpus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status Status
	}{{"legacy", Missing}, {"malformed", Invalid}, {"truncated", Invalid}, {"duplicate-key", Invalid}, {"nested-fence", Valid}, {"crlf", Valid}, {"legacy-v1-research", Valid}, {"unsupported-version", UnsupportedVersion}} {
		t.Run(tt.name, func(t *testing.T) {
			s := fixture(t, tt.name)
			r := Parse(s, Target{Stage: "step-01", Scope: "stage"})
			if r.Status != tt.status {
				t.Fatalf("%s: %+v", r.Status, r.Diagnostics)
			}
		})
	}
}
func TestStrictJSONAndFooterSyntax(t *testing.T) {
	s := fixture(t, "step-01")
	cases := []struct{ name, content string }{
		{"duplicate nested key", strings.Replace(s, `"confidence": "verified"`, `"confidence": "verified", "confidence": "verified"`, 1)},
		{"unknown envelope field", strings.Replace(s, `"schema_version": 2`, `"summary":"no", "schema_version": 2`, 1)},
		{"JSON string envelope", footer(`"text"`)},
		{"missing version", strings.Replace(s, `"schema_version": 2,`, "", 1)},
		{"fractional version", strings.Replace(s, `"schema_version": 2`, `"schema_version": 2.5`, 1)},
		{"null required field", strings.Replace(s, `"problem": "Document saves can lose data."`, `"problem": null`, 1)},
		{"nonfinite number", strings.Replace(s, `"weight_percent": 100`, `"weight_percent": 1e1000`, 1)},
		{"second block", strings.Replace(s, "\n<!-- buildflow:view:end -->", "\n```json\n{}\n```\n<!-- buildflow:view:end -->", 1)},
		{"no heading", strings.Replace(s, viewHeading, "## Wrong heading", 1)},
		{"text inside markers", strings.Replace(s, "<!-- buildflow:view:end -->", "Prose inside footer\n<!-- buildflow:view:end -->", 1)},
		{"orphan end", endMarker + "\n"},
		{"wrong fence type", strings.Replace(s, "```json", "```go", 1)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := Parse(tt.content, Target{Stage: "step-01", Scope: "stage"})
			if r.Status == Valid || r.Status == Missing {
				t.Fatalf("trusted invalid input: %+v", r)
			}
			if r.Blocks != nil || r.Artifact != nil {
				t.Fatal("untrusted data exposed")
			}
		})
	}
	// Encoded markers and fence runs are legitimate declaration data.
	v := fixtureJSON(t, "step-02")
	contract := object(object(object(v["blocks"])["design"])["interfaces"].([]any)[0])
	decl := "line one\r\n```json\n" + startMarker + "\n" + endMarker + "\n~~~\n{{literal}} café"
	contract["declaration"] = decl
	r := Parse(encoded(t, v), Target{Stage: "step-02", Scope: "stage"})
	if r.Status != Valid {
		t.Fatal(r.Diagnostics)
	}
	if got := r.Blocks.(*HighLevelBlocks).Design.Interfaces[0].Declaration; got != decl {
		t.Fatalf("declaration changed: %q", got)
	}
}
func FuzzParsePreservesBounds(f *testing.F) {
	for _, s := range []string{"", "# Legacy\r\n", "````\n" + startMarker + "\n```\n", footer(`{}`)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r := Parse(s, Target{Stage: "step-01", Scope: "stage"})
		if r.NarrativeEnd < 0 || r.NarrativeEnd > len(s) {
			t.Fatal("invalid narrative bounds")
		}
		if r.Status != Valid && (r.Blocks != nil || r.Artifact != nil) {
			t.Fatal("untrusted data exposed")
		}
		if r.FooterStart >= 0 && (r.FooterEnd < r.FooterStart || r.FooterEnd > len(s)) {
			t.Fatal("invalid footer bounds")
		}
	})
}

func TestSafeBoundsDoNotDiscardTrailingProse(t *testing.T) {
	s := fixture(t, "step-01")
	content := s + "Retain this trailing narrative.\n"
	r := Parse(content, Target{Stage: "step-01", Scope: "stage"})
	if r.Status != Invalid || content[r.FooterEnd:] != "Retain this trailing narrative.\n" {
		t.Fatalf("unsafe footer bounds: %+v", r)
	}
}
