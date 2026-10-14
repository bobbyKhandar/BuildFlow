package workspace

import "strings"

// This file implements the "output flows into the next step's input" feature.
//
// When an output is saved, its content is copied into the next step's input so
// the next step starts with the previous result in front of it. The copied block
// is wrapped in a marker comment pair:
//
//	<!-- buildflow:auto:step-01 -->
//	<output content>
//	<!-- /buildflow:auto:step-01 -->
//
// The marker makes the block visible (so the copy is never mysterious), reversible
// (delete it and it stays gone until the output is saved again) and updatable
// (saving the output again replaces the block instead of appending a second one).
//
// The block is appended *after* the user's own notes, separated by three blank
// lines, so the notes stay at the top where the user left them.

// autoCopyOpenPrefix opens every auto-copy marker. User notes are everything
// before the first one.
const autoCopyOpenPrefix = "<!-- buildflow:auto:"

// autoCopySeparator is three blank lines: the notes, then three empty lines,
// then the auto-copied block.
const autoCopySeparator = "\n\n\n\n"

func autoCopyOpen(stage string) string  { return autoCopyOpenPrefix + stage + " -->" }
func autoCopyClose(stage string) string { return "<!-- /buildflow:auto:" + stage + " -->" }

// MergeAutoCopy returns existing with any earlier auto-copy of fromStage's
// output replaced by output. When existing holds nothing but an earlier copy,
// the result is just the new block. The user's own notes are preserved above
// the block, separated by three blank lines.
func MergeAutoCopy(existing, output, fromStage string) string {
	existing = removeAutoCopyBlock(existing, fromStage)
	block := autoCopyOpen(fromStage) + "\n" + strings.TrimSpace(output) + "\n" + autoCopyClose(fromStage)
	notes := strings.TrimSpace(existing)
	if notes == "" {
		return block + "\n"
	}
	return notes + autoCopySeparator + block + "\n"
}

// removeAutoCopyBlock deletes the auto-copy block for fromStage from content
// and returns what is left, with the seam tidied up. Unknown blocks are left
// alone so a hand-edited file never loses text.
func removeAutoCopyBlock(content, fromStage string) string {
	open := autoCopyOpen(fromStage)
	close := autoCopyClose(fromStage)
	start := strings.Index(content, open)
	if start < 0 {
		return content
	}
	rel := strings.Index(content[start:], close)
	if rel < 0 {
		return content
	}
	end := start + rel + len(close)
	before := strings.TrimSpace(content[:start])
	after := strings.TrimSpace(content[end:])
	switch {
	case before == "" && after == "":
		return ""
	case before == "":
		return after
	case after == "":
		return before
	default:
		return before + "\n\n" + after
	}
}

// UserNotes returns the part of an input document that the user wrote, with any
// auto-copied block removed. The assembled prompt substitutes {{input}} with
// this, so the previous output reaches the prompt through the dedicated
// {{output_step_XX}} tokens instead of being duplicated inside the notes, and
// the "you have not written any notes" warning still fires when the notes are
// empty even though the file as a whole is not.
func UserNotes(content string) string {
	idx := strings.Index(content, autoCopyOpenPrefix)
	if idx < 0 {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(content[:idx])
}

// StripFrontMatter returns the body of a Markdown document with any leading
// YAML front matter block removed. It is used when copying a package document
// into a Step 5 input: the front matter is metadata, not prompt content.
func StripFrontMatter(content string) string {
	_, body := SplitFrontMatter(content)
	return body
}
