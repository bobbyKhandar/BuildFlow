package structured

// SchemaVersion is independent of the plan/projection version.
const SchemaVersion = 2

type Status string

const (
	Missing            Status = "missing"
	Valid              Status = "valid"
	Invalid            Status = "invalid"
	UnsupportedVersion Status = "unsupported_version"
)

// Target is the expected document identity. References supplies validated
// upstream artifacts for cross-stage joins; absent upstream catalogues are
// reported as unchecked, never inferred from prose. Evidence is always local.
type Target struct {
	Stage      string      `json:"stage"`
	Scope      string      `json:"scope"`
	PackageID  string      `json:"package_id,omitempty"`
	References []*Envelope `json:"-"`
}

// Envelope.Blocks is a pointer to the stage's concrete blocks type.
type Envelope struct {
	SchemaVersion int     `json:"schema_version"`
	Stage         string  `json:"stage"`
	Scope         string  `json:"scope"`
	PackageID     *string `json:"package_id"`
	State         string  `json:"state"`
	Blocks        any     `json:"blocks"`
}

type Diagnostic struct {
	Path     string `json:"path"` // JSON pointer, or empty for document-level errors
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // error or warning
}

// Result exposes structured data only after all available checks succeed.
// Bounds are byte offsets into the unchanged input, including with CRLF/UTF-8.
// If metadata cannot be bounded confidently, NarrativeEnd is len(content).
type Result struct {
	Status              Status       `json:"status"`
	SourceSchemaVersion int          `json:"source_schema_version,omitempty"`
	Adapted             bool         `json:"adapted,omitempty"`
	Artifact            *Envelope    `json:"artifact,omitempty"`
	Blocks              any          `json:"blocks,omitempty"`
	NarrativeEnd        int          `json:"narrative_end"`
	FooterStart         int          `json:"footer_start"`
	FooterEnd           int          `json:"footer_end"`
	Diagnostics         []Diagnostic `json:"diagnostics"`
}

func blockValue(stage string) any {
	switch stage {
	case "step-01":
		return new(ResearchBlocks)
	case "step-02":
		return new(HighLevelBlocks)
	case "step-02-5":
		return new(StackBlocks)
	case "step-03":
		return new(LowLevelBlocks)
	case "step-04":
		return new(WorkPackageBlocks)
	case "step-05":
		return new(CodingPromptBlocks)
	}
	return nil
}
