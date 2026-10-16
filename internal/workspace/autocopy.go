package workspace

import "strings"

// Legacy auto-copy markers remain readable so old copied answers never become
// user notes. New saves do not copy answers into inputs; handoffs are projected.

// autoCopyOpenPrefix opens every auto-copy marker. User notes are everything
// before the first one.
const autoCopyOpenPrefix = "<!-- buildflow:auto:"

// UserNotes returns the part of an input document that the user wrote, with any
// legacy auto-copied tail excluded. This also excludes incomplete legacy copies
// whose closing marker was lost, preserving the original note boundary.
func UserNotes(content string) string {
	idx := strings.Index(content, autoCopyOpenPrefix)
	if idx < 0 {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(content[:idx])
}

// StripFrontMatter returns the body of a Markdown document with any leading
// YAML front matter block removed. Custom whole-output placeholders use the
// body because the front matter is metadata, not prompt content.
func StripFrontMatter(content string) string {
	_, body := SplitFrontMatter(content)
	return body
}
