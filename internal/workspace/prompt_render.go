package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// compactJSON preserves decoded strings and uses the standard library's stable
// map-key order. Disabling HTML escaping keeps code and Unicode readable.
func compactJSON(value any) (string, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// RenderPromptProjection renders data once, followed by explicitly labelled
// compatibility sources. Provenance stays outside the model's structured data.
func RenderPromptProjection(p PromptProjection) (string, error) {
	var data any = p.Data
	if p.State == "blocked" {
		blockers := []string{}
		for _, d := range p.Diagnostics {
			if d.Severity == "error" {
				blockers = append(blockers, d.Message)
			}
		}
		data = map[string]any{"state": "blocked", "blockers": blockers}
	}
	text, err := compactJSON(data)
	if err != nil {
		return "", err
	}
	label := "Selected handoff — " + p.Stage
	if p.PackageID != "" {
		label += " / " + p.PackageID
	}
	text = "## " + label + "\n" + fencedJSON(text)
	for _, legacy := range p.Legacy {
		text += "\n\n" + legacy.Rendered
	}
	return text, nil
}

// A long fence also handles literal Markdown examples inside JSON strings.
func fencedJSON(payload string) string {
	longest, run := 0, 0
	for _, ch := range payload {
		if ch == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	width := longest + 1
	if width < 3 {
		width = 3
	}
	fence := strings.Repeat("`", width)
	return fence + "json\n" + payload + "\n" + fence
}

type PromptSizeContribution struct {
	Category        string `json:"category"`
	Chars           int    `json:"chars"`
	EstimatedTokens int    `json:"estimatedTokens"`
}

type PromptSize struct {
	Chars            int                      `json:"chars"`
	EstimatedTokens  int                      `json:"estimatedTokens"`
	EstimateMethod   string                   `json:"estimateMethod"`
	WarningThreshold int                      `json:"warningThreshold"`
	Contributions    []PromptSizeContribution `json:"contributions"`
}

func estimatedTokens(chars int) int { return (chars + 3) / 4 }

// EstimatePromptSize also accepts manually edited prompt text. Contributions
// are supplied only when assembly knows their origin; edits are never guessed.
func EstimatePromptSize(content, stage string) PromptSize {
	n := utf8.RuneCountInString(content)
	threshold := 32000
	if stage == "step-04" {
		threshold = 48000
	}
	return PromptSize{n, estimatedTokens(n), "character estimate (~4 characters/token)", threshold, nil}
}

func assemblySize(content, stage, body string, values map[string]string, p *PromptProjection) PromptSize {
	size := EstimatePromptSize(content, stage)
	counts := map[string]int{"fixed_instructions_schema": 0, "projected_data": 0, "notes": 0, "legacy_fallback": 0}
	for _, match := range tokenRe.FindAllStringSubmatch(body, -1) {
		token := match[1]
		value, ok := values[token]
		if !ok {
			continue
		}
		n := utf8.RuneCountInString(value)
		switch {
		case token == "handoff":
			legacy := 0
			if p != nil {
				for _, l := range p.Legacy {
					legacy += utf8.RuneCountInString("\n\n" + l.Rendered)
				}
			}
			counts["legacy_fallback"] += legacy
			counts["projected_data"] += n - legacy
		case token == "input" || token == "package_input" || strings.HasPrefix(token, "user."):
			counts["notes"] += n
		case strings.HasPrefix(token, "output_") || token == "previous_output" || token == "system_context" || token == "package_content":
			counts["legacy_fallback"] += n
		}
	}
	counts["fixed_instructions_schema"] = size.Chars - counts["projected_data"] - counts["notes"] - counts["legacy_fallback"]
	for _, category := range []string{"fixed_instructions_schema", "projected_data", "notes", "legacy_fallback"} {
		n := counts[category]
		size.Contributions = append(size.Contributions, PromptSizeContribution{category, n, estimatedTokens(n)})
	}
	return size
}

func sizeWarning(size PromptSize) *AssemblyWarning {
	if size.EstimatedTokens < size.WarningThreshold {
		return nil
	}
	return &AssemblyWarning{Token: "prompt_size", Label: "Estimated prompt size", Reason: fmt.Sprintf("approximately %d input tokens (%d characters) reaches the %d-token warning threshold; content has been preserved in full", size.EstimatedTokens, size.Chars, size.WarningThreshold), Severity: "warning", Hint: "Review the size contributors and destination context capacity. No content was trimmed."}
}
