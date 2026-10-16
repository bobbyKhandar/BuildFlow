package workspace

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"buildflow/internal/structured"
)

// FallbackBoundary uses byte offsets into the unchanged saved document.
type FallbackBoundary struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Heading string `json:"heading,omitempty"`
}

type LegacyPromptSource struct {
	Path     string `json:"path"`
	Mode     string `json:"mode"`
	Rendered string `json:"rendered"`
}

type markdownHeading struct {
	FallbackBoundary
	level int
}

var numberedHeading = regexp.MustCompile(`^\s*(?:\d+(?:\.\d+)+[.)]?\s+|\d+[.)]\s+)`)

func normalizedHeading(s string) string {
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), "#"))
	return strings.ToLower(numberedHeading.ReplaceAllString(s, ""))
}

// markdownHeadings ignores fenced examples, including longer outer fences and
// tilde fences, so example headings cannot become fallback boundaries.
func markdownHeadings(content string) []markdownHeading {
	var out []markdownHeading
	var fence byte
	width, offset := 0, 0
	for _, line := range strings.SplitAfter(content, "\n") {
		raw := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		trimmed := strings.TrimLeft(raw, " ")
		if len(raw)-len(trimmed) <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			n := 0
			for n < len(trimmed) && trimmed[n] == trimmed[0] {
				n++
			}
			if n >= 3 {
				info := strings.TrimSpace(trimmed[n:])
				if fence == 0 && !(trimmed[0] == '`' && strings.Contains(info, "`")) {
					fence, width = trimmed[0], n
				} else if fence == trimmed[0] && n >= width && info == "" {
					fence = 0
				}
				offset += len(line)
				continue
			}
		}
		if fence == 0 && len(raw)-len(trimmed) <= 3 {
			if level := headingLevel(trimmed); level > 0 {
				out = append(out, markdownHeading{FallbackBoundary{offset, len(content), strings.TrimSpace(trimmed[level:])}, level})
			}
		}
		offset += len(line)
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j].level <= out[i].level {
				out[i].End = out[j].Start
				break
			}
		}
	}
	return out
}

func fallbackNarrative(d *projectionDocument) string {
	content := d.doc.Content
	if d.parsed.FooterStart >= 0 && d.parsed.FooterEnd >= d.parsed.FooterStart && strings.TrimSpace(content[d.parsed.FooterEnd:]) == "" {
		// Keep offsets into the original document. Unexpected trailing text is
		// preserved with its metadata rather than discarded.
		content = content[:d.parsed.FooterStart]
	}
	return content
}

func (b *projectionBuilder) legacyFallback(d *projectionDocument, rows []ProjectionField) {
	if !d.doc.Exists || strings.TrimSpace(d.doc.Content) == "" {
		return
	}
	for _, existing := range b.out.Legacy {
		if existing.Path == d.doc.Path {
			return
		}
	}
	content := fallbackNarrative(d)
	headings := markdownHeadings(content)
	var bounds []FallbackBoundary
	document := false
	// Effective context is authoritative for legacy recovery too. Explicit
	// amendments must accompany the baseline when no effective section exists.
	if d.doc.Stage == "step-03" || b.out.Sources[d.index].Label == "step-03" {
		effective := false
		for _, h := range headings {
			if normalizedHeading(h.Heading) == "effective implementation context" {
				effective = true
				bounds = append(bounds, h.FallbackBoundary)
			}
		}
		var adjusted []ProjectionField
		for _, row := range rows {
			if effective && strings.HasPrefix(row.Source, "implementation_context.") {
				continue
			}
			adjusted = append(adjusted, row)
		}
		rows = adjusted
		for _, h := range headings {
			name := normalizedHeading(h.Heading)
			if name == "design changes" || name == "replacements" || name == "design changes/replacements" {
				bounds = append(bounds, h.FallbackBoundary)
			}
		}
	}
	for _, row := range rows {
		found := false
		for _, h := range headings {
			for _, wanted := range row.LegacyHeadings {
				if normalizedHeading(h.Heading) == normalizedHeading(wanted) {
					bounds = append(bounds, h.FallbackBoundary)
					found = true
					break
				}
			}
		}
		if !found && row.Required {
			document = true
		}
	}
	mode := "markdown_section"
	if document || len(bounds) == 0 {
		mode = "markdown_document"
		_, body := SplitFrontMatter(content)
		bounds = []FallbackBoundary{{len(content) - len(body), len(content), ""}}
	}
	sort.Slice(bounds, func(i, j int) bool {
		if bounds[i].Start == bounds[j].Start {
			return bounds[i].End > bounds[j].End
		}
		return bounds[i].Start < bounds[j].Start
	})
	var unique []FallbackBoundary
	for _, bound := range bounds {
		if len(unique) > 0 && bound.Start < unique[len(unique)-1].End {
			continue
		}
		unique = append(unique, bound)
	}
	label := "legacy section; field selection unavailable"
	if mode == "markdown_document" {
		label = "unstructured legacy context; field selection unavailable"
	}
	if b.target.Stage == "step-05" {
		label += "; unfiltered legacy context; package membership is not established by prose"
	}
	rendered := fmt.Sprintf("### Source: %s (%s)\n", d.doc.Path, label)
	for _, bound := range unique {
		rendered += "\n" + content[bound.Start:bound.End]
	}
	source := &b.out.Sources[d.index]
	source.Mode, source.FallbackBoundaries = mode, unique
	for _, row := range rows {
		source.Selections = append(source.Selections, ProjectionSelection{SourcePointer: "/blocks/" + strings.ReplaceAll(strings.TrimSuffix(row.Source, "[]"), ".", "/"), OutputPointer: "/" + strings.ReplaceAll(row.Output, ".", "/"), Predicate: "legacy_unfiltered", Fields: append([]string{}, row.Fields...)})
	}
	source.Chars = utf8.RuneCountInString(rendered)
	source.EstimatedTokens = estimatedTokens(source.Chars)
	b.out.Legacy = append(b.out.Legacy, LegacyPromptSource{d.doc.Path, mode, rendered})
	b.diagnostic(d, "", "legacy_fallback", label, "warning")
	if d.parsed.FooterStart < 0 && strings.Contains(content, "<!-- buildflow:view:") {
		b.diagnostic(d, "", "damaged_metadata", "metadata bounds are uncertain; original text was preserved", "warning")
	}
}

func (b *projectionBuilder) sourceRows(stage string) []ProjectionField {
	spec, _ := PromptProjectionSpec(b.target.Stage)
	var rows []ProjectionField
	for _, row := range spec.Fields {
		if row.SourceStage == stage {
			rows = append(rows, row)
		}
	}
	return rows
}

// Context recovery is legacy-only. It never replaces an explicit valid null.
func (b *projectionBuilder) recoverLegacyContext() error {
	d := b.documents["step-03"]
	if d == nil || d.blocks != nil || !d.doc.Exists {
		return nil
	}
	for _, h := range markdownHeadings(fallbackNarrative(d)) {
		if normalizedHeading(h.Heading) == "effective implementation context" {
			return nil
		}
	}
	baseline, err := b.read(KindOutput, "step-02-5", "stage", "", false, nil)
	if err != nil {
		return err
	}
	rows := operationalProjection("step-02-5", "technology", "system_context", b.target.Stage)
	if baseline.blocks != nil {
		data := map[string]any{}
		for _, row := range rows {
			value, pointer, exists := projectionLookup(baseline.blocks, row.Source)
			if !exists {
				continue
			}
			var pointers []string
			selected := projectLeaves(value, row.Fields, pointer, &pointers)
			data[row.Source] = selected
			source := &b.out.Sources[baseline.index]
			source.Selections = append(source.Selections, ProjectionSelection{SourcePointer: pointer, OutputPointer: row.Output, Predicate: "legacy_unfiltered", Fields: append([]string{}, row.Fields...)})
			b.out.Sources[baseline.index].SelectedPointers = append(b.out.Sources[baseline.index].SelectedPointers, pointers...)
		}
		payload, err := compactJSON(data)
		if err != nil {
			return err
		}
		rendered := "### Source: " + baseline.doc.Path + " (legacy context recovery; unfiltered baseline)\n" + payload
		b.out.Legacy = append(b.out.Legacy, LegacyPromptSource{baseline.doc.Path, "json_projection", rendered})
		b.out.Sources[baseline.index].Chars = utf8.RuneCountInString(rendered)
	} else if baseline.doc.Exists {
		b.legacyFallback(baseline, rows)
	}
	b.out.Sources[baseline.index].Active = true
	if !baseline.doc.Exists || strings.TrimSpace(baseline.doc.Content) == "" {
		b.diagnostic(baseline, "", "missing_source", "legacy Stage 3 has no effective context section and the Stage 2.5 recovery source is missing or empty", "error")
		return nil
	}
	b.diagnostic(d, "", "legacy_context_recovery", "effective context recovered from Stage 2.5 and explicit Stage 3 amendments; legacy amendments may be incomplete", "warning")
	return nil
}

// Legacy invalid sources can support design discussion; semantic link errors
// remain blockers, as do all unverifiable Stage 5 essentials and package maps.
func (b *projectionBuilder) fallbackSeverity(diagnostic structured.Diagnostic) string {
	if b.target.Stage == "step-05" || b.target.PackageID != "" {
		return "error"
	}
	if diagnostic.Code == "reference" || diagnostic.Code == "conflict" {
		return "error"
	}
	return "warning"
}
