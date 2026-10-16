package structured

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const startMarker = "<!-- buildflow:view:start -->"
const endMarker = "<!-- buildflow:view:end -->"
const viewHeading = "## BuildFlow Structured View"

type sourceLine struct {
	text       string
	start, end int
}

func sourceLines(s string) []sourceLine {
	var lines []sourceLine
	for offset := 0; offset < len(s); {
		end := strings.IndexByte(s[offset:], '\n')
		if end < 0 {
			end = len(s)
		} else {
			end += offset + 1
		}
		lines = append(lines, sourceLine{strings.TrimSuffix(strings.TrimSuffix(s[offset:end], "\n"), "\r"), offset, end})
		offset = end
	}
	return lines
}

// fence recognizes CommonMark-style fenced blocks with up to three leading
// spaces. Closing runs must match the character, be at least as long, and have
// no info string. A shorter inner fence cannot close an outer example.
func fence(line string) (byte, int, string) {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	if n > 3 || n == len(line) {
		return 0, 0, ""
	}
	s := line[n:]
	ch := s[0]
	if ch != '`' && ch != '~' {
		return 0, 0, ""
	}
	i := 0
	for i < len(s) && s[i] == ch {
		i++
	}
	if i < 3 {
		return 0, 0, ""
	}
	return ch, i, strings.TrimSpace(s[i:])
}

// Parse recognizes exactly one footer outside examples, checks the entire JSON
// value, and returns no partially validated blocks on any error.
func Parse(content string, target Target) Result {
	r := Result{Status: Missing, NarrativeEnd: len(content), FooterStart: -1, FooterEnd: -1, Diagnostics: []Diagnostic{}}
	fail := func(code, msg string) Result {
		r.Status = Invalid
		r.Diagnostics = append(r.Diagnostics, Diagnostic{"", code, msg, "error"})
		return r
	}
	var err error
	target, err = checkedTarget(target)
	if err != nil {
		return fail("target", err.Error())
	}
	lines := sourceLines(content)
	var starts, ends []int
	var ch byte
	var width int
	for i, l := range lines {
		fc, fw, info := fence(l.text)
		if ch != 0 {
			if fc == ch && fw >= width && info == "" {
				ch = 0
			}
			continue
		}
		if fc != 0 {
			if fc == '`' && strings.Contains(info, "`") {
				continue
			}
			ch, width = fc, fw
			continue
		}
		switch strings.TrimSpace(l.text) {
		case startMarker:
			starts = append(starts, i)
		case endMarker:
			ends = append(ends, i)
		}
	}
	if len(starts) == 0 && len(ends) == 0 {
		return r
	}
	if len(starts) != 1 || len(ends) != 1 || starts[0] >= ends[0] {
		return fail("footer", "expected exactly one complete, unambiguous footer outside code fences")
	}
	a, b := starts[0], ends[0]
	h := a - 1
	for h >= 0 && strings.TrimSpace(lines[h].text) == "" {
		h--
	}
	if h < 0 || lines[h].text != viewHeading {
		return fail("footer", "footer must be preceded by the BuildFlow Structured View heading")
	}
	// These bounds are safe even if the bounded JSON is malformed.
	r.FooterStart = lines[h].start
	r.NarrativeEnd = r.FooterStart
	r.FooterEnd = lines[b].end
	if strings.TrimSpace(content[lines[b].end:]) != "" {
		return fail("footer", "content after the footer closing marker is not allowed")
	}
	i := a + 1
	for i < b && strings.TrimSpace(lines[i].text) == "" {
		i++
	}
	if i >= b {
		return fail("footer", "missing fenced json block")
	}
	fc, fw, info := fence(lines[i].text)
	if fc == 0 || info != "json" {
		return fail("footer", "expected one fenced json block")
	}
	j := i + 1
	for j < b {
		ec, ew, ei := fence(lines[j].text)
		if ec == fc && ew >= fw && ei == "" {
			break
		}
		j++
	}
	if j == b {
		return fail("footer", "unclosed json fence")
	}
	for k := j + 1; k < b; k++ {
		if strings.TrimSpace(lines[k].text) != "" {
			return fail("footer", "only one json block is allowed inside the footer")
		}
	}
	raw := content[lines[i].end:lines[j].start]
	value, path, code, err := decodeUnique(raw)
	if err != nil {
		r = fail(code, err.Error())
		r.Diagnostics[len(r.Diagnostics)-1].Path = path
		return r
	}
	m, ok := value.(map[string]any)
	if !ok {
		return fail("schema", "envelope must be an object")
	}
	version, ok := m["schema_version"].(float64)
	if ok && version != float64(int(version)) {
		return fail("schema", "schema_version must be an integer")
	}
	if ok {
		r.SourceSchemaVersion = int(version)
	}
	// Only research with no scoped records can be lifted losslessly from v1.
	// Its original fields already establish every required relationship. Later
	// stages require new joins; accepting them would invent project knowledge.
	if version == 1 && target.Stage == "step-01" {
		blocks := object(m["blocks"])
		if len(array(blocks["constraints"])) == 0 && len(array(blocks["risks"])) == 0 && len(array(blocks["assumptions"])) == 0 && len(array(blocks["open_questions"])) == 0 {
			m["schema_version"] = float64(SchemaVersion)
			version = SchemaVersion
			r.Adapted = true
			r.Diagnostics = append(r.Diagnostics, Diagnostic{"/schema_version", "legacy_adapter", "lossless version-1 research adapter; no scoped records or missing relationships", "warning"})
		}
	}
	if ok && version != SchemaVersion {
		r.Status = UnsupportedVersion
		msg := "unsupported schema version; use labelled legacy fallback"
		if version == 1 {
			msg = "version 1 lacks explicit applicability and linking relationships; use legacy fallback without guessing missing links"
		}
		r.Diagnostics = append(r.Diagnostics, Diagnostic{"/schema_version", "version", msg, "warning"})
		return r
	}
	bSchema, err := Schema(target)
	if err != nil {
		return fail("schema", err.Error())
	}
	var s map[string]any
	_ = json.Unmarshal(bSchema, &s)
	// Target mismatches get a dedicated diagnostic, rather than a generic const error.
	var pkg any
	if target.Scope == "package" {
		pkg = target.PackageID
	}
	if m["stage"] != target.Stage || m["scope"] != target.Scope || m["package_id"] != pkg {
		return fail("target", "footer stage, scope, or package does not match the requested target")
	}
	r.Diagnostics = append(r.Diagnostics, validateSchema(value, s, s["$defs"].(map[string]any), "")...)
	if hasErrors(r.Diagnostics) {
		r.Status = Invalid
		return r
	}
	data := blockValue(target.Stage)
	bytes, _ := json.Marshal(m["blocks"])
	if err = json.Unmarshal(bytes, data); err != nil {
		return fail("schema", err.Error())
	}
	artifact := &Envelope{SchemaVersion: SchemaVersion, Stage: target.Stage, Scope: target.Scope, State: m["state"].(string), Blocks: data}
	if target.Scope == "package" {
		id := target.PackageID
		artifact.PackageID = &id
	}
	r.Diagnostics = append(r.Diagnostics, Validate(artifact, target.References...)...)
	if hasErrors(r.Diagnostics) {
		r.Status = Invalid
		return r
	}
	r.Status = Valid
	r.Artifact = artifact
	r.Blocks = data
	return r
}
func hasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// decodeUnique rejects duplicate object keys at every depth and trailing values.
func decodeUnique(raw string) (any, string, string, error) {
	d := json.NewDecoder(strings.NewReader(raw))
	var walk func(string) (any, string, string, error)
	walk = func(path string) (any, string, string, error) {
		t, err := d.Token()
		if err != nil {
			return nil, path, "json", err
		}
		delim, is := t.(json.Delim)
		if !is {
			return t, "", "", nil
		}
		switch delim {
		case '{':
			m := map[string]any{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return nil, path, "json", err
				}
				k, ok := key.(string)
				if !ok {
					return nil, path, "json", fmt.Errorf("object key must be a string")
				}
				p := pointer(path, k)
				if _, ok = m[k]; ok {
					return nil, p, "duplicate_key", fmt.Errorf("duplicate JSON key %q", k)
				}
				v, p, c, err := walk(p)
				if err != nil {
					return nil, p, c, err
				}
				m[k] = v
			}
			if _, err = d.Token(); err != nil {
				return nil, path, "json", err
			}
			return m, "", "", nil
		case '[':
			a := []any{}
			for d.More() {
				v, p, c, err := walk(pointer(path, strconv.Itoa(len(a))))
				if err != nil {
					return nil, p, c, err
				}
				a = append(a, v)
			}
			if _, err = d.Token(); err != nil {
				return nil, path, "json", err
			}
			return a, "", "", nil
		}
		return nil, path, "json", fmt.Errorf("unexpected JSON delimiter")
	}
	v, p, c, err := walk("")
	if err != nil {
		return nil, p, c, err
	}
	if _, err = d.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing JSON value")
		}
		return nil, "", "json", err
	}
	return v, "", "", nil
}
