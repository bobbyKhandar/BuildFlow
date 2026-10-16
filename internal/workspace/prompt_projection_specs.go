package workspace

import "strings"

// ProjectionField describes a fixed source row, not an executable selector.
// Fields are explicit leaf paths; [] denotes an array. Empty Fields means the
// source itself is a scalar or an array of scalars, never an object wildcard.
type ProjectionField struct {
	SourceStage    string   `json:"sourceStage"`
	Source         string   `json:"source"`
	Output         string   `json:"output"`
	Fields         []string `json:"fields"`
	Predicate      string   `json:"predicate"`
	Required       bool     `json:"required"`
	NullHandling   string   `json:"nullHandling"`
	LegacyHeadings []string `json:"legacyHeadings"`
	Verbatim       []string `json:"verbatim"`
}

type ProjectionSpec struct {
	ID     string            `json:"id"`
	Stage  string            `json:"stage"`
	Fields []ProjectionField `json:"fields"`
}

// These profiles implement plan-v3 §5. They are deliberately compiled into Go.
// Callers can inspect a copy through PromptProjectionSpec, but cannot change
// the registry used by assembly.
var (
	basisFields       = words("confidence source_ids[] assumption_ids[]")
	appFields         = words("scope requirement_ids[] component_ids[] module_ids[] contract_ids[]")
	valueFieldFields  = words("name type required rules[]")
	requirementCode   = with(words("id kind statement measurable_target"), "basis", basisFields)
	requirementDesign = appendFields(requirementCode, "area")
	constraintCode    = with(words("id statement strength"), "basis", basisFields)
	constraintDesign  = appendFields(constraintCode, "negotiable")
	contractFields    = with(with(words("id kind name language declaration version stability failure_modes[] referenced_contract_ids[] owner.kind owner.id configuration.has_default configuration.default_literal configuration.validation_rules[]"), "input_fields[]", valueFieldFields), "output_fields[]", valueFieldFields)
	fileFields        = words("path exists action purpose")
	testFields        = words("id name level file fixtures[] assertions[] failure_path command required_for_acceptance requirement_ids[] contract_ids[]")
	acceptanceFields  = words("id statement requirement_ids[] module_ids[] test_ids[]")
	algorithmFields   = words("id module_ids[] operation steps[] pseudocode complexity input_size edge_cases[].condition edge_cases[].handling")
	errorFields       = words("id contract_ids[] condition detected_at code user_message log_level retry_policy fatal")
	concurrencyFields = words("id module_ids[] execution_context shared_state[] synchronization ordering race_prevention idempotency safe_rerun")
	stateFields       = words("id module_ids[] entity states[] transitions[].from transitions[].to transitions[].trigger transitions[].persistence terminal_states[] crash_recovery")
	surfaceFields     = words("id module_ids[] kind name regions[] states[].name states[].copy states[].actions[] keyboard_behavior[] destructive_actions[]")
	signalFields      = words("id owner_ids[] kind name level_or_unit emitted_when diagnostic_use")
	riskCode          = words("id title early_signal mitigation")
	riskDesign        = appendFields(riskCode, "likelihood impact")
	assumptionFields  = words("id statement consequence_if_wrong validation")
	questionFields    = words("id question blocking recommended_default cheapest_check resolve_by")
	sourceFields      = words("id kind locator checked_on confidence")
)

func words(s string) []string { return strings.Fields(s) }
func appendFields(fields []string, extra string) []string {
	return append(append([]string{}, fields...), words(extra)...)
}
func with(fields []string, prefix string, leaves []string) []string {
	out := append([]string{}, fields...)
	for _, leaf := range leaves {
		if prefix != "" {
			leaf = prefix + "." + leaf
		}
		out = append(out, leaf)
	}
	return out
}
func scoped(fields []string) []string { return with(fields, "applies_to", appFields) }

func projectionRow(stage, source, output, predicate, legacy string, fields []string, verbatim string) ProjectionField {
	return ProjectionField{stage, source, output, append([]string{}, fields...), predicate, true, "preserve; required undetermined source blocks projection", strings.Split(legacy, "|"), words(verbatim)}
}

func commonProjection(stage string, design, applicability bool) []ProjectionField {
	risk := riskCode
	if design {
		risk = riskDesign
	}
	assumption, question := assumptionFields, questionFields
	if applicability {
		risk, assumption, question = scoped(risk), scoped(assumption), scoped(question)
	}
	return []ProjectionField{
		projectionRow(stage, "risks[]", "risks", "applicable", "Risks|Open risks carried forward|Risks and mitigations", risk, ""),
		projectionRow(stage, "assumptions[]", "assumptions", "applicable_or_evidence", "Assumptions", assumption, ""),
		projectionRow(stage, "open_questions[]", "open_questions", "applicable", "Open questions", question, ""),
		projectionRow(stage, "sources[]", "sources", "referenced_evidence", "Sources and confidence|Sources", sourceFields, ""),
	}
}

func researchProjection() []ProjectionField {
	s := "step-01"
	return append([]ProjectionField{
		projectionRow(s, "problem", "problem", "all", "Problem statement", nil, "problem"),
		projectionRow(s, "goals[]", "goals", "all", "Goals and non-goals", words("id outcome success_measure"), ""),
		projectionRow(s, "non_goals[]", "non_goals", "all", "Goals and non-goals", words("item"), ""),
		projectionRow(s, "scenarios[]", "scenarios", "all", "Users and key scenarios", words("id user trigger goal steps[] unhappy_path outcome"), ""),
		projectionRow(s, "requirements[]", "requirements", "all", "Functional requirements|Non-functional requirements", appendFields(requirementDesign, "rationale"), "statement measurable_target"),
		projectionRow(s, "constraints[]", "constraints", "all", "Constraints", constraintDesign, "statement"),
		projectionRow(s, "integrations[]", "integrations", "all", "Interfaces and integration points", with(words("id name direction expected_volume failure_behavior"), "data_shape[]", valueFieldFields), "failure_behavior data_shape[].rules[]"),
		projectionRow(s, "data_state[]", "data_state", "all", "Data and state", words("id data owner location retention sensitivity design_implications[]"), ""),
		projectionRow(s, "options[]", "options", "all", "Options worth considering", words("id name description advantage disadvantage riskiest_assumption"), ""),
		projectionRow(s, "evaluation_criteria[]", "evaluation_criteria", "all", "Evaluation criteria for the eventual decision", words("id criterion weight_percent rewards penalizes"), ""),
	}, commonProjection(s, true, false)...)
}

func highLevelProjection(consumer string) []ProjectionField {
	s := "step-02"
	decision := with(words("id choice revisit_when"), "basis", basisFields)
	constraint := constraintDesign
	settled := scoped(with(words("id choice revisit_when"), "basis", basisFields))
	if consumer == "step-02-5" {
		decision = appendFields(decision, "tradeoffs[]")
	} else {
		constraint = scoped(constraint)
		settled = scoped(words("id choice revisit_when"))
	}
	return append([]ProjectionField{
		projectionRow(s, "decision", "decision", "all", "The decision", decision, ""),
		projectionRow(s, "requirements[]", "requirements", "all", "Requirements|Functional requirements|Non-functional requirements", requirementDesign, "statement measurable_target"),
		projectionRow(s, "constraints[]", "constraints", "all", "Constraints", constraint, "statement"),
		projectionRow(s, "design.components[]", "design.components", "all", "System overview", words("id name responsibilities[] owns[]"), ""),
		projectionRow(s, "design.interactions[]", "design.interactions", "all", "System overview", words("from_component_id to_component_id purpose data"), ""),
		projectionRow(s, "design.entities[]", "design.entities", "all", "Data model", with(words("id name relationships[] ownership storage normalization_rules[] deletion_rules[]"), "fields[]", valueFieldFields), "fields[].rules[] deletion_rules[]"),
		projectionRow(s, "design.key_flows[]", "design.key_flows", "all", "Key flows", words("id trigger steps[] state_changes[] failure_branches[].condition failure_branches[].user_result"), ""),
		projectionRow(s, "design.interfaces[]", "contracts", "not_superseded", "Interfaces", contractFields, "declaration input_fields[].rules[] output_fields[].rules[] failure_modes[] configuration.default_literal configuration.validation_rules[]"),
		projectionRow(s, "design.error_handling[]", "design.error_handling", "all", "Error handling and edge cases", errorFields, "code user_message retry_policy"),
		projectionRow(s, "design.security", "design.security", "all", "Security and privacy", words("threats[] controls[] sensitive_data[] trust_boundaries[]"), ""),
		projectionRow(s, "design.data_policies", "design.data_policies", "all", "Security and privacy", words("ownership[] retention[] privacy[]"), "ownership[] retention[] privacy[]"),
		projectionRow(s, "scope", "scope", "all", "Scope boundaries", words("included[] deferred[] never[]"), ""),
		projectionRow(s, "delivery_plan[]", "delivery_plan", "all", "Delivery plan", words("id outcome requirement_ids[] acceptance[]"), "acceptance[]"),
		projectionRow(s, "settled_decisions[]", "settled_decisions", "all", "Decisions a reader should not re-litigate", settled, ""),
	}, commonProjection(s, true, consumer == "step-03")...)
}

// operationalProjection narrows the operational profile for packaging; coding
// receives the full operational fields, filtered to its selected package.
func operationalProjection(stage, technology, context, consumer string) []ProjectionField {
	row := func(source, output, predicate, legacy string, fields []string, verbatim string) ProjectionField {
		return projectionRow(stage, source, output, predicate, legacy, fields, verbatim)
	}
	techOut, ctxOut := "implementation_context.technology.", "implementation_context.system_context."
	stack := scoped(with(words("id layer technology version"), "basis", basisFields))
	library := scoped(with(words("id language ecosystem name version capability used_in[] licence concerns[]"), "basis", basisFields))
	service := scoped(words("id name direction protocol authentication rate_limits failure_behavior offline_strategy contract_ids[]"))
	mission := words("purpose must_never_do[]")
	layout := scoped(words("path purpose example_paths[]"))
	conventions := words("naming[] file_layout[] errors[] logging[] comments[] time[] paths[] configuration[]")
	testing := words("frameworks[] levels[] naming[] acceptance_bar[]")
	budget := scoped(with(words("resource target measurement"), "basis", basisFields))
	if consumer == "step-04" {
		stack = scoped(words("id layer technology version"))
		library = scoped(words("id language name version capability concerns[]"))
		service = scoped(words("id name contract_ids[] offline_strategy"))
		mission = words("must_never_do[]")
		layout = scoped(words("path purpose"))
		conventions = words("file_layout[]")
		testing = words("frameworks[] levels[] acceptance_bar[]")
		budget = scoped(words("resource target measurement"))
	}
	if consumer == "step-05" {
		// Selection metadata is used by predicates, then omitted from the handoff.
		stack = with(words("id layer technology version"), "basis", basisFields)
		library = with(words("id language ecosystem name version capability used_in[] licence concerns[]"), "basis", basisFields)
		service = words("id name direction protocol authentication rate_limits failure_behavior offline_strategy contract_ids[]")
		layout = words("path purpose")
		budget = words("resource target measurement")
	}
	rows := []ProjectionField{
		row(technology+".stack[]", techOut+"stack", "applicable", "Technology decisions", stack, "version"),
		row(technology+".libraries[]", techOut+"libraries", "applicable", "Main libraries|Dependency list", library, "version"),
		row(technology+".services[]", techOut+"services", "applicable", "APIs and services", service, "authentication rate_limits failure_behavior offline_strategy"),
		row(technology+".commands", techOut+"commands", "command_keys", "Development environment", words("build test lint run release"), "build test lint run release"),
		row(context+".mission", ctxOut+"mission", "all", "Reusable System Context", mission, "must_never_do[]"),
		row(context+".repository_layout[]", ctxOut+"repository_layout", "applicable_or_ancestor", "Reusable System Context", layout, "path"),
		row(context+".conventions", ctxOut+"conventions", "all", "Reusable System Context", conventions, strings.Join(conventions, " ")),
		row(context+".invariants[]", ctxOut+"invariants", "applicable", "Reusable System Context", scoped(words("id rule enforcement")), "rule enforcement"),
		row(context+".testing_standards", ctxOut+"testing_standards", "all", "Reusable System Context", testing, "acceptance_bar[]"),
		row(context+".budgets[]", ctxOut+"budgets", "applicable", "Reusable System Context", budget, "target measurement"),
	}
	if consumer != "step-04" {
		tool, setup := scoped(words("id category name purpose used_in[]")), scoped(words("id text"))
		pattern := scoped(words("name rule example_language example"))
		anti := scoped(words("rule reason"))
		glossary := scoped(words("term definition"))
		if consumer == "step-05" {
			tool, setup = words("id category name purpose used_in[]"), words("id text")
			pattern, anti, glossary = words("name rule example_language example"), words("rule"), words("term definition")
		}
		rows = append(rows,
			row(technology+".tools[]", techOut+"tools", "applicable", "Suggested tools|Technology decisions", tool, ""),
			row(technology+".security", techOut+"security", "all", "Security and compliance", words("secrets[] dependency_updates[] vulnerability_scanning[] data_handling[] compliance[]"), "secrets[] dependency_updates[] vulnerability_scanning[] data_handling[] compliance[]"),
			row(technology+".observability[]", techOut+"observability", "owners", "Observability", signalFields, "name"),
			row(context+".patterns[]", ctxOut+"patterns", "applicable", "Reusable System Context", pattern, "example"),
			row(context+".anti_patterns[]", ctxOut+"anti_patterns", "applicable", "Reusable System Context", anti, "rule"),
			row(context+".glossary[]", ctxOut+"glossary", "applicable", "Reusable System Context", glossary, ""),
		)
		for _, section := range words("prerequisites installation version_pinning clean_machine_setup") {
			rows = append(rows, row(technology+".environment."+section+"[]", techOut+"environment."+section, "applicable", "Development environment", setup, "text"))
		}
	}
	if consumer == "step-03" {
		rows = append(rows,
			row(context+".public_contract_ids[]", ctxOut+"public_contract_ids", "all", "Reusable System Context", nil, ""),
			row(context+".extension_recipe[]", ctxOut+"extension_recipe", "all", "Reusable System Context", nil, "extension_recipe[]"),
		)
	}
	if consumer == "step-05" {
		for i := range rows {
			if strings.HasSuffix(rows[i].Source, ".invariants[]") {
				rows[i].Fields = words("id rule enforcement")
			}
		}
	}
	return rows
}

func lowLevelProjection(consumer string) []ProjectionField {
	s := "step-03"
	requirement, constraint := requirementCode, scoped(constraintCode)
	decision := scoped(words("id choice revisit_when"))
	module := with(words("id component_ids[] requirement_ids[] name responsibility why_separate depends_on[] consumed_contract_ids[] produced_contract_ids[]"), "files[]", fileFields)
	scope := words("included[] deferred[] never[]")
	if consumer == "step-05" {
		constraint, decision = constraintCode, words("id choice")
		module, scope = words("id name responsibility"), words("never[]")
	}
	rows := []ProjectionField{
		projectionRow(s, "requirements[]", "requirements", "requirements", "Requirements|Functional requirements|Non-functional requirements", requirement, "statement measurable_target"),
		projectionRow(s, "constraints[]", "constraints", "applicable", "Constraints", constraint, "statement"),
		projectionRow(s, "decisions[]", "decisions", "decisions", "Decisions|Settled decisions", decision, "choice"),
		projectionRow(s, "scope", "scope", "all", "Scope boundaries", scope, "never[]"),
		projectionRow(s, "modules[]", "modules", "modules", "Module breakdown|File and directory plan", module, "files[].path"),
		projectionRow(s, "contracts[]", "contracts", "contracts", "Types, signatures and schemas", contractFields, "declaration input_fields[].rules[] output_fields[].rules[] failure_modes[] configuration.default_literal configuration.validation_rules[]"),
	}
	rows = append(rows, operationalProjection(s, "implementation_context.technology", "implementation_context.system_context", consumer)...)
	if consumer == "step-05" {
		rows = append(rows, projectionRow(s, "modules[]", "boundary_modules", "boundary_modules", "Module breakdown", words("id name"), ""))
	} else {
		for _, detail := range []struct {
			source, predicate, legacy, verbatim string
			fields                              []string
		}{
			{"algorithms", "module_links", "Algorithms", "steps[] pseudocode edge_cases[].handling", algorithmFields},
			{"error_cases", "contract_links", "Error taxonomy", "code user_message retry_policy", errorFields},
			{"concurrency", "module_links", "Concurrency and ordering", "synchronization ordering race_prevention idempotency safe_rerun", concurrencyFields},
			{"state_machines", "module_links", "State transitions", "transitions[].from transitions[].to transitions[].trigger transitions[].persistence crash_recovery", stateFields},
			{"surfaces", "module_links", "User interface specification", "states[].name states[].copy states[].actions[] keyboard_behavior[] destructive_actions[]", surfaceFields},
			{"observability", "owners", "Observability hooks", "name", signalFields},
			{"test_plan", "test_modules", "Test plan", "tests[].file tests[].fixtures[] tests[].assertions[] tests[].command", with(words("module_id"), "tests[]", testFields)},
			{"acceptance", "module_links", "Acceptance criteria", "statement", acceptanceFields},
			{"requirement_coverage", "coverage", "Requirement coverage|Deliberately excluded", "", words("requirement_id module_ids[] disposition reason")},
		} {
			rows = append(rows, projectionRow(s, detail.source+"[]", detail.source, detail.predicate, detail.legacy, detail.fields, detail.verbatim))
		}
		rows = append(rows, projectionRow(s, "build_batches", "build_batches", "batches", "Implementation sequence", nil, ""))
	}
	return append(rows, commonProjection(s, false, consumer == "step-04")...)
}

func packageProjection() []ProjectionField {
	fields := words("id title goal module_ids[] requirement_ids[] depends_on[] prerequisite_reasons[].package_id prerequisite_reasons[].reason scope_in[] scope_out[] consumed_contract_ids[] produced_contract_ids[] implementation.contract_ids[] implementation.notes[] implementation.decision_ids[] source_acceptance_ids[] command_keys[]")
	for _, profile := range []struct {
		path   string
		fields []string
	}{
		{"files[]", fileFields}, {"tests[]", testFields}, {"acceptance[]", acceptanceFields}, {"risk", riskCode},
		{"implementation.algorithms[]", algorithmFields}, {"implementation.error_cases[]", errorFields},
		{"implementation.concurrency[]", concurrencyFields}, {"implementation.state_machines[]", stateFields},
		{"implementation.surfaces[]", surfaceFields}, {"implementation.observability[]", signalFields},
	} {
		fields = with(fields, profile.path, profile.fields)
	}
	return append([]ProjectionField{
		projectionRow("step-04", "packages[]", "package", "selected_package", "Selected saved package", fields, "scope_in[] scope_out[] implementation.notes[]"),
		projectionRow("step-04", "integration_points[]", "integration_points", "integration_points", "Delivery Overview", words("package_ids[] contract_ids[] verification"), "verification"),
	}, commonProjection("step-04", false, false)...)
}

func makeProjectionSpecs() map[string]ProjectionSpec {
	stage3 := highLevelProjection("step-03")
	stage3 = append(stage3, operationalProjection("step-02-5", "technology", "system_context", "step-03")...)
	stage3 = append(stage3,
		projectionRow("step-02-5", "contracts[]", "contracts", "all", "Reusable System Context|APIs and services", contractFields, "declaration input_fields[].rules[] output_fields[].rules[] failure_modes[] configuration.default_literal configuration.validation_rules[]"),
		projectionRow("step-02-5", "costs.scaling_triggers[]", "scaling_triggers", "all", "Cost estimates", words("metric threshold action"), ""),
		projectionRow("step-02-5", "costs.ten_times_load", "ten_times_load", "all", "Cost estimates", words("bottlenecks[] changes_required[]"), ""),
	)
	stage3 = append(stage3, commonProjection("step-02-5", true, true)...)
	out := map[string]ProjectionSpec{}
	for stage, rows := range map[string][]ProjectionField{
		"step-02": researchProjection(), "step-02-5": highLevelProjection("step-02-5"),
		"step-03": stage3, "step-04": lowLevelProjection("step-04"),
		"step-05": append(packageProjection(), lowLevelProjection("step-05")...),
	} {
		out[stage] = ProjectionSpec{stage + "/v3", stage, rows}
	}
	return out
}

var promptProjectionSpecs = makeProjectionSpecs()

// PromptProjectionSpec returns an independent, inspectable specification.
func PromptProjectionSpec(stage string) (ProjectionSpec, bool) {
	spec, ok := promptProjectionSpecs[stage]
	spec.Fields = append([]ProjectionField{}, spec.Fields...)
	for i := range spec.Fields {
		spec.Fields[i].Fields = append([]string{}, spec.Fields[i].Fields...)
		spec.Fields[i].Verbatim = append([]string{}, spec.Fields[i].Verbatim...)
		spec.Fields[i].LegacyHeadings = append([]string{}, spec.Fields[i].LegacyHeadings...)
	}
	return spec, ok
}
