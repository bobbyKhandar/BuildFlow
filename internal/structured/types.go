// Package structured parses and validates BuildFlow answer footers without changing source documents.
package structured

type Applicability struct {
	Scope          string   `json:"scope"`
	RequirementIDs []string `json:"requirement_ids"`
	ComponentIDs   []string `json:"component_ids"`
	ModuleIDs      []string `json:"module_ids"`
	ContractIDs    []string `json:"contract_ids"`
}

type AcceptanceCriterion struct {
	ID             string   `json:"id"`
	Statement      string   `json:"statement"`
	RequirementIDs []string `json:"requirement_ids"`
	ModuleIDs      []string `json:"module_ids"`
	TestIDs        []string `json:"test_ids"`
}

type OperationalDecision struct {
	ID          string        `json:"id"`
	Choice      string        `json:"choice"`
	RevisitWhen string        `json:"revisit_when"`
	AppliesTo   Applicability `json:"applies_to"`
}

type Component struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Responsibilities []string `json:"responsibilities"`
	Owns             []string `json:"owns"`
}

type Confidence string

type Level string

type Basis struct {
	Confidence    Confidence `json:"confidence"`
	SourceIDs     []string   `json:"source_ids"`
	AssumptionIDs []string   `json:"assumption_ids"`
}

type Source struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Locator    string     `json:"locator"`
	CheckedOn  *string    `json:"checked_on"`
	Confidence Confidence `json:"confidence"`
}

type Assumption struct {
	AppliesTo          Applicability `json:"applies_to"`
	ID                 string        `json:"id"`
	Statement          string        `json:"statement"`
	ConsequenceIfWrong string        `json:"consequence_if_wrong"`
	Validation         string        `json:"validation"`
}

type Risk struct {
	AppliesTo   Applicability `json:"applies_to"`
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Category    string        `json:"category"`
	Likelihood  Level         `json:"likelihood"`
	Impact      Level         `json:"impact"`
	EarlySignal string        `json:"early_signal"`
	Mitigation  string        `json:"mitigation"`
}

type Question struct {
	AppliesTo          Applicability `json:"applies_to"`
	ID                 string        `json:"id"`
	Question           string        `json:"question"`
	WhyItMatters       string        `json:"why_it_matters"`
	CheapestCheck      string        `json:"cheapest_check"`
	Blocking           bool          `json:"blocking"`
	RecommendedDefault *string       `json:"recommended_default"`
	ResolveBy          *string       `json:"resolve_by"`
}

type CommonBlocks struct {
	Assumptions   []Assumption `json:"assumptions"`
	Risks         []Risk       `json:"risks"`
	OpenQuestions []Question   `json:"open_questions"`
	Sources       []Source     `json:"sources"`
}

type Requirement struct {
	ID               string  `json:"id"`
	Kind             string  `json:"kind"`
	Area             string  `json:"area"`
	Statement        string  `json:"statement"`
	MeasurableTarget *string `json:"measurable_target"`
	Rationale        string  `json:"rationale"`
	Basis            Basis   `json:"basis"`
}

type Constraint struct {
	AppliesTo  Applicability `json:"applies_to"`
	ID         string        `json:"id"`
	Statement  string        `json:"statement"`
	Strength   string        `json:"strength"`
	Negotiable bool          `json:"negotiable"`
	Basis      Basis         `json:"basis"`
}

type Decision struct {
	AppliesTo   Applicability `json:"applies_to"`
	ID          string        `json:"id"`
	Choice      string        `json:"choice"`
	Reasons     []string      `json:"reasons"`
	Tradeoffs   []string      `json:"tradeoffs"`
	RevisitWhen string        `json:"revisit_when"`
	Basis       Basis         `json:"basis"`
}

type Alternative struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Advantage          string  `json:"advantage"`
	Disadvantage       string  `json:"disadvantage"`
	RiskiestAssumption string  `json:"riskiest_assumption"`
	RejectionReason    *string `json:"rejection_reason"`
	ReconsiderWhen     *string `json:"reconsider_when"`
}

type Field struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Rules    []string `json:"rules"`
}

type InterfaceContract struct {
	ID    string `json:"id"`
	Owner struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"owner"`
	ReferencedContractIDs []string `json:"referenced_contract_ids"`
	Kind                  string   `json:"kind"`
	Name                  string   `json:"name"`
	Language              string   `json:"language"`
	Declaration           string   `json:"declaration"`
	InputFields           []Field  `json:"input_fields"`
	OutputFields          []Field  `json:"output_fields"`
	Version               *string  `json:"version"`
	Stability             string   `json:"stability"`
	FailureModes          []string `json:"failure_modes"`
	Configuration         *struct {
		HasDefault      bool     `json:"has_default"`
		DefaultLiteral  *string  `json:"default_literal"`
		ValidationRules []string `json:"validation_rules"`
	} `json:"configuration"`
}

type FileChange struct {
	Path    string `json:"path"`
	Exists  *bool  `json:"exists"`
	Action  string `json:"action"`
	Purpose string `json:"purpose"`
}

type PlannedTest struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Level                 string   `json:"level"`
	File                  *string  `json:"file"`
	Fixtures              []string `json:"fixtures"`
	Assertions            []string `json:"assertions"`
	FailurePath           bool     `json:"failure_path"`
	Command               *string  `json:"command"`
	RequiredForAcceptance bool     `json:"required_for_acceptance"`
	RequirementIDs        []string `json:"requirement_ids"`
	ContractIDs           []string `json:"contract_ids"`
}

type Effort struct {
	MinPersonDays  float64 `json:"min_person_days"`
	MaxPersonDays  float64 `json:"max_person_days"`
	Basis          string  `json:"basis"`
	BiggestUnknown string  `json:"biggest_unknown"`
}

type ErrorCase struct {
	ID          string   `json:"id"`
	ContractIDs []string `json:"contract_ids"`
	Condition   string   `json:"condition"`
	DetectedAt  string   `json:"detected_at"`
	Code        string   `json:"code"`
	UserMessage string   `json:"user_message"`
	LogLevel    string   `json:"log_level"`
	RetryPolicy string   `json:"retry_policy"`
	Fatal       bool     `json:"fatal"`
}

type Algorithm struct {
	ID         string   `json:"id"`
	ModuleIDs  []string `json:"module_ids"`
	Operation  string   `json:"operation"`
	Steps      []string `json:"steps"`
	Pseudocode *string  `json:"pseudocode"`
	Complexity string   `json:"complexity"`
	InputSize  string   `json:"input_size"`
	EdgeCases  []struct {
		Condition string `json:"condition"`
		Handling  string `json:"handling"`
	} `json:"edge_cases"`
}

type ConcurrencyRule struct {
	ID               string   `json:"id"`
	ModuleIDs        []string `json:"module_ids"`
	ExecutionContext string   `json:"execution_context"`
	SharedState      []string `json:"shared_state"`
	Synchronization  string   `json:"synchronization"`
	Ordering         string   `json:"ordering"`
	RacePrevention   string   `json:"race_prevention"`
	Idempotency      string   `json:"idempotency"`
	SafeRerun        string   `json:"safe_rerun"`
}

type StateMachine struct {
	ID          string   `json:"id"`
	ModuleIDs   []string `json:"module_ids"`
	Entity      string   `json:"entity"`
	States      []string `json:"states"`
	Transitions []struct {
		From        string `json:"from"`
		To          string `json:"to"`
		Trigger     string `json:"trigger"`
		Persistence string `json:"persistence"`
	} `json:"transitions"`
	TerminalStates []string `json:"terminal_states"`
	CrashRecovery  string   `json:"crash_recovery"`
}

type Surface struct {
	ID        string   `json:"id"`
	ModuleIDs []string `json:"module_ids"`
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Regions   []string `json:"regions"`
	States    []struct {
		Name    string   `json:"name"`
		Copy    string   `json:"copy"`
		Actions []string `json:"actions"`
	} `json:"states"`
	KeyboardBehavior   []string `json:"keyboard_behavior"`
	DestructiveActions []string `json:"destructive_actions"`
}

type Signal struct {
	ID            string   `json:"id"`
	OwnerIDs      []string `json:"owner_ids"`
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	LevelOrUnit   string   `json:"level_or_unit"`
	EmittedWhen   string   `json:"emitted_when"`
	DiagnosticUse string   `json:"diagnostic_use"`
}

type ResearchBlocks struct {
	Assumptions   []Assumption `json:"assumptions"`
	Risks         []Risk       `json:"risks"`
	OpenQuestions []Question   `json:"open_questions"`
	Sources       []Source     `json:"sources"`
	Problem       string       `json:"problem"`
	Goals         []struct {
		ID             string `json:"id"`
		Outcome        string `json:"outcome"`
		SuccessMeasure string `json:"success_measure"`
	} `json:"goals"`
	NonGoals []struct {
		Item   string `json:"item"`
		Reason string `json:"reason"`
	} `json:"non_goals"`
	Scenarios []struct {
		ID          string   `json:"id"`
		User        string   `json:"user"`
		Trigger     string   `json:"trigger"`
		Goal        string   `json:"goal"`
		Steps       []string `json:"steps"`
		UnhappyPath bool     `json:"unhappy_path"`
		Outcome     string   `json:"outcome"`
	} `json:"scenarios"`
	Requirements []Requirement `json:"requirements"`
	Constraints  []Constraint  `json:"constraints"`
	Integrations []struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		Direction       string  `json:"direction"`
		DataShape       []Field `json:"data_shape"`
		ExpectedVolume  *string `json:"expected_volume"`
		FailureBehavior string  `json:"failure_behavior"`
	} `json:"integrations"`
	DataState []struct {
		ID                 string   `json:"id"`
		Data               string   `json:"data"`
		Owner              string   `json:"owner"`
		Location           string   `json:"location"`
		Retention          *string  `json:"retention"`
		Sensitivity        string   `json:"sensitivity"`
		DesignImplications []string `json:"design_implications"`
	} `json:"data_state"`
	Options            []Alternative `json:"options"`
	EvaluationCriteria []struct {
		ID            string  `json:"id"`
		Criterion     string  `json:"criterion"`
		WeightPercent float64 `json:"weight_percent"`
		Rewards       string  `json:"rewards"`
		Penalizes     string  `json:"penalizes"`
	} `json:"evaluation_criteria"`
}

type HighLevelDesign struct {
	Components   []Component `json:"components"`
	Interactions []struct {
		FromComponentID string `json:"from_component_id"`
		ToComponentID   string `json:"to_component_id"`
		Purpose         string `json:"purpose"`
		Data            string `json:"data"`
	} `json:"interactions"`
	Entities []struct {
		ID                 string   `json:"id"`
		Name               string   `json:"name"`
		Fields             []Field  `json:"fields"`
		Relationships      []string `json:"relationships"`
		Ownership          string   `json:"ownership"`
		Storage            string   `json:"storage"`
		StorageRationale   string   `json:"storage_rationale"`
		NormalizationRules []string `json:"normalization_rules"`
		DeletionRules      []string `json:"deletion_rules"`
	} `json:"entities"`
	KeyFlows []struct {
		ID              string   `json:"id"`
		Trigger         string   `json:"trigger"`
		Steps           []string `json:"steps"`
		StateChanges    []string `json:"state_changes"`
		FailureBranches []struct {
			Condition  string `json:"condition"`
			UserResult string `json:"user_result"`
		} `json:"failure_branches"`
	} `json:"key_flows"`
	Interfaces    []InterfaceContract `json:"interfaces"`
	ErrorHandling []ErrorCase         `json:"error_handling"`
	Security      struct {
		Threats         []string `json:"threats"`
		Controls        []string `json:"controls"`
		SensitiveData   []string `json:"sensitive_data"`
		TrustBoundaries []string `json:"trust_boundaries"`
	} `json:"security"`
	DataPolicies struct {
		Ownership []string `json:"ownership"`
		Retention []string `json:"retention"`
		Privacy   []string `json:"privacy"`
	} `json:"data_policies"`
}

type HighLevelBlocks struct {
	Assumptions   []Assumption     `json:"assumptions"`
	Risks         []Risk           `json:"risks"`
	OpenQuestions []Question       `json:"open_questions"`
	Sources       []Source         `json:"sources"`
	Decision      *Decision        `json:"decision"`
	Alternatives  []Alternative    `json:"alternatives"`
	Requirements  []Requirement    `json:"requirements"`
	Constraints   []Constraint     `json:"constraints"`
	Design        *HighLevelDesign `json:"design"`
	Scope         struct {
		Included []string `json:"included"`
		Deferred []string `json:"deferred"`
		Never    []string `json:"never"`
	} `json:"scope"`
	DeliveryPlan []struct {
		ID             string   `json:"id"`
		Outcome        string   `json:"outcome"`
		RequirementIDs []string `json:"requirement_ids"`
		Acceptance     []string `json:"acceptance"`
	} `json:"delivery_plan"`
	SettledDecisions []Decision `json:"settled_decisions"`
}

type Technology struct {
	Stack []struct {
		ID         string        `json:"id"`
		AppliesTo  Applicability `json:"applies_to"`
		Layer      string        `json:"layer"`
		Technology string        `json:"technology"`
		Version    *string       `json:"version"`
		Rationale  string        `json:"rationale"`
		RunnerUp   *string       `json:"runner_up"`
		CostNote   string        `json:"cost_note"`
		Basis      Basis         `json:"basis"`
	} `json:"stack"`
	Tools []struct {
		ID        string        `json:"id"`
		AppliesTo Applicability `json:"applies_to"`
		Category  string        `json:"category"`
		Name      string        `json:"name"`
		Purpose   string        `json:"purpose"`
		UsedIn    []string      `json:"used_in"`
	} `json:"tools"`
	Libraries []struct {
		AppliesTo        Applicability `json:"applies_to"`
		ID               string        `json:"id"`
		Language         string        `json:"language"`
		Ecosystem        string        `json:"ecosystem"`
		Name             string        `json:"name"`
		Version          *string       `json:"version"`
		Capability       string        `json:"capability"`
		DesignImpact     []string      `json:"design_impact"`
		SelectionReason  string        `json:"selection_reason"`
		UsedIn           []string      `json:"used_in"`
		Licence          *string       `json:"licence"`
		LastRelease      *string       `json:"last_release"`
		MaintainerHealth *string       `json:"maintainer_health"`
		Concerns         []string      `json:"concerns"`
		Basis            Basis         `json:"basis"`
	} `json:"libraries"`
	Services []struct {
		AppliesTo       Applicability `json:"applies_to"`
		ID              string        `json:"id"`
		Name            string        `json:"name"`
		Direction       string        `json:"direction"`
		Protocol        string        `json:"protocol"`
		Authentication  string        `json:"authentication"`
		RateLimits      *string       `json:"rate_limits"`
		CostModel       *string       `json:"cost_model"`
		FailureBehavior string        `json:"failure_behavior"`
		OfflineStrategy string        `json:"offline_strategy"`
		ContractIDs     []string      `json:"contract_ids"`
	} `json:"services"`
	Environment struct {
		Prerequisites []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"prerequisites"`
		Installation []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"installation"`
		VersionPinning []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"version_pinning"`
		CleanMachineSetup []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"clean_machine_setup"`
	} `json:"environment"`
	Commands struct {
		Build   *string `json:"build"`
		Test    *string `json:"test"`
		Lint    *string `json:"lint"`
		Run     *string `json:"run"`
		Release *string `json:"release"`
	} `json:"commands"`
	Security struct {
		Secrets               []string `json:"secrets"`
		DependencyUpdates     []string `json:"dependency_updates"`
		VulnerabilityScanning []string `json:"vulnerability_scanning"`
		DataHandling          []string `json:"data_handling"`
		Compliance            []string `json:"compliance"`
	} `json:"security"`
	Observability []Signal `json:"observability"`
}

type SystemContext struct {
	Mission struct {
		Purpose     string   `json:"purpose"`
		MustNeverDo []string `json:"must_never_do"`
	} `json:"mission"`
	RepositoryLayout []struct {
		Path         string        `json:"path"`
		Purpose      string        `json:"purpose"`
		ExamplePaths []string      `json:"example_paths"`
		AppliesTo    Applicability `json:"applies_to"`
	} `json:"repository_layout"`
	PublicContractIDs []string `json:"public_contract_ids"`
	Conventions       struct {
		Naming        []string `json:"naming"`
		FileLayout    []string `json:"file_layout"`
		Errors        []string `json:"errors"`
		Logging       []string `json:"logging"`
		Comments      []string `json:"comments"`
		Time          []string `json:"time"`
		Paths         []string `json:"paths"`
		Configuration []string `json:"configuration"`
	} `json:"conventions"`
	Patterns []struct {
		AppliesTo       Applicability `json:"applies_to"`
		Name            string        `json:"name"`
		Rule            string        `json:"rule"`
		ExampleLanguage string        `json:"example_language"`
		Example         string        `json:"example"`
	} `json:"patterns"`
	AntiPatterns []struct {
		Rule      string        `json:"rule"`
		Reason    string        `json:"reason"`
		AppliesTo Applicability `json:"applies_to"`
	} `json:"anti_patterns"`
	Invariants []struct {
		ID          string        `json:"id"`
		Rule        string        `json:"rule"`
		Enforcement string        `json:"enforcement"`
		AppliesTo   Applicability `json:"applies_to"`
	} `json:"invariants"`
	TestingStandards struct {
		Frameworks    []string `json:"frameworks"`
		Levels        []string `json:"levels"`
		Naming        []string `json:"naming"`
		AcceptanceBar []string `json:"acceptance_bar"`
	} `json:"testing_standards"`
	Glossary []struct {
		Term       string        `json:"term"`
		Definition string        `json:"definition"`
		AppliesTo  Applicability `json:"applies_to"`
	} `json:"glossary"`
	Budgets []struct {
		AppliesTo   Applicability `json:"applies_to"`
		Resource    string        `json:"resource"`
		Target      string        `json:"target"`
		Measurement string        `json:"measurement"`
		Basis       Basis         `json:"basis"`
	} `json:"budgets"`
	ExtensionRecipe []string `json:"extension_recipe"`
}

type ImplementationContext struct {
	Technology    OperationalTechnology `json:"technology"`
	SystemContext SystemContext         `json:"system_context"`
}

type StackBlocks struct {
	Assumptions   []Assumption        `json:"assumptions"`
	Risks         []Risk              `json:"risks"`
	OpenQuestions []Question          `json:"open_questions"`
	Sources       []Source            `json:"sources"`
	Components    []Component         `json:"components"`
	Contracts     []InterfaceContract `json:"contracts"`
	Technology    Technology          `json:"technology"`
	SystemContext *SystemContext      `json:"system_context"`
	Costs         struct {
		Currency   *string `json:"currency"`
		BuildAreas []struct {
			Area   string `json:"area"`
			Effort Effort `json:"effort"`
		} `json:"build_areas"`
		BuildTotal                     *Effort  `json:"build_total"`
		ContingencyPercent             int      `json:"contingency_percent"`
		ContingencyPersonDays          *float64 `json:"contingency_person_days"`
		TotalWithContingencyPersonDays *float64 `json:"total_with_contingency_person_days"`
		Monthly                        []struct {
			Usage             string   `json:"usage"`
			VolumeAssumptions []string `json:"volume_assumptions"`
			Items             []struct {
				Name       string   `json:"name"`
				Units      *float64 `json:"units"`
				Unit       string   `json:"unit"`
				UnitPrice  *float64 `json:"unit_price"`
				Arithmetic string   `json:"arithmetic"`
				Amount     *float64 `json:"amount"`
				Basis      Basis    `json:"basis"`
			} `json:"items"`
			Total *float64 `json:"total"`
		} `json:"monthly"`
		ScalingTriggers []struct {
			Metric       string  `json:"metric"`
			Threshold    string  `json:"threshold"`
			Action       string  `json:"action"`
			NextStepCost *string `json:"next_step_cost"`
		} `json:"scaling_triggers"`
		TenTimesLoad struct {
			Bottlenecks     []string `json:"bottlenecks"`
			ChangesRequired []string `json:"changes_required"`
		} `json:"ten_times_load"`
	} `json:"costs"`
	RejectedTechnologies []Alternative `json:"rejected_technologies"`
}

type LowLevelBlocks struct {
	Assumptions   []Assumption           `json:"assumptions"`
	Risks         []Risk                 `json:"risks"`
	OpenQuestions []Question             `json:"open_questions"`
	Sources       []Source               `json:"sources"`
	Requirements  []EffectiveRequirement `json:"requirements"`
	Constraints   []Constraint           `json:"constraints"`
	Decisions     []OperationalDecision  `json:"decisions"`
	Scope         struct {
		Included []string `json:"included"`
		Deferred []string `json:"deferred"`
		Never    []string `json:"never"`
	} `json:"scope"`
	ImplementationContext *ImplementationContext `json:"implementation_context"`
	Modules               []struct {
		ID                  string       `json:"id"`
		Name                string       `json:"name"`
		Responsibility      string       `json:"responsibility"`
		WhySeparate         string       `json:"why_separate"`
		DependsOn           []string     `json:"depends_on"`
		PublicContractIDs   []string     `json:"public_contract_ids"`
		ComponentIDs        []string     `json:"component_ids"`
		RequirementIDs      []string     `json:"requirement_ids"`
		ConsumedContractIDs []string     `json:"consumed_contract_ids"`
		ProducedContractIDs []string     `json:"produced_contract_ids"`
		Files               []FileChange `json:"files"`
	} `json:"modules"`
	Contracts     []InterfaceContract `json:"contracts"`
	Algorithms    []Algorithm         `json:"algorithms"`
	ErrorCases    []ErrorCase         `json:"error_cases"`
	Concurrency   []ConcurrencyRule   `json:"concurrency"`
	StateMachines []StateMachine      `json:"state_machines"`
	Surfaces      []Surface           `json:"surfaces"`
	Observability []Signal            `json:"observability"`
	TestPlan      []struct {
		ModuleID string        `json:"module_id"`
		Tests    []PlannedTest `json:"tests"`
	} `json:"test_plan"`
	Acceptance          []AcceptanceCriterion `json:"acceptance"`
	RequirementCoverage []struct {
		RequirementID string   `json:"requirement_id"`
		ModuleIDs     []string `json:"module_ids"`
		Disposition   string   `json:"disposition"`
		Reason        string   `json:"reason"`
	} `json:"requirement_coverage"`
	BuildBatches         [][]string `json:"build_batches"`
	DeliberatelyExcluded []struct {
		RequirementID string `json:"requirement_id"`
		Reason        string `json:"reason"`
	} `json:"deliberately_excluded"`
	DesignChanges []struct {
		ID               string `json:"id"`
		AffectedSource   string `json:"affected_source"`
		PreviousDecision string `json:"previous_decision"`
		Replacement      string `json:"replacement"`
		Rationale        string `json:"rationale"`
	} `json:"design_changes"`
}

type PackageImplementation struct {
	Notes         []string          `json:"notes"`
	ContractIDs   []string          `json:"contract_ids"`
	Algorithms    []Algorithm       `json:"algorithms"`
	ErrorCases    []ErrorCase       `json:"error_cases"`
	Concurrency   []ConcurrencyRule `json:"concurrency"`
	StateMachines []StateMachine    `json:"state_machines"`
	Surfaces      []Surface         `json:"surfaces"`
	Observability []Signal          `json:"observability"`
	DecisionIDs   []string          `json:"decision_ids"`
}

type WorkPackage struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Goal                string   `json:"goal"`
	WhyNow              string   `json:"why_now"`
	Languages           []string `json:"languages"`
	ModuleIDs           []string `json:"module_ids"`
	RequirementIDs      []string `json:"requirement_ids"`
	DependsOn           []string `json:"depends_on"`
	PrerequisiteReasons []struct {
		PackageID string `json:"package_id"`
		Reason    string `json:"reason"`
	} `json:"prerequisite_reasons"`
	ScopeIn             []string              `json:"scope_in"`
	ScopeOut            []string              `json:"scope_out"`
	Files               []FileChange          `json:"files"`
	ConsumedContractIDs []string              `json:"consumed_contract_ids"`
	ProducedContractIDs []string              `json:"produced_contract_ids"`
	Implementation      PackageImplementation `json:"implementation"`
	Tests               []PlannedTest         `json:"tests"`
	Acceptance          []AcceptanceCriterion `json:"acceptance"`
	CommandKeys         []string              `json:"command_keys"`
	SourceAcceptanceIDs []string              `json:"source_acceptance_ids"`
	Effort              *Effort               `json:"effort"`
	Risk                *Risk                 `json:"risk"`
}

type WorkPackageBlocks struct {
	Assumptions       []Assumption  `json:"assumptions"`
	Risks             []Risk        `json:"risks"`
	OpenQuestions     []Question    `json:"open_questions"`
	Sources           []Source      `json:"sources"`
	Packages          []WorkPackage `json:"packages"`
	BuildBatches      [][]string    `json:"build_batches"`
	IntegrationPoints []struct {
		PackageIDs   []string `json:"package_ids"`
		ContractIDs  []string `json:"contract_ids"`
		Verification string   `json:"verification"`
	} `json:"integration_points"`
	DeliberatelyNotCreated []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"deliberately_not_created"`
}

type CodingPromptBlocks struct {
	Assumptions   []Assumption `json:"assumptions"`
	Risks         []Risk       `json:"risks"`
	OpenQuestions []Question   `json:"open_questions"`
	Sources       []Source     `json:"sources"`
	Target        struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Goal  string `json:"goal"`
	} `json:"target"`
	Scope struct {
		Included []string `json:"included"`
		Excluded []string `json:"excluded"`
	} `json:"scope"`
	Files                      []FileChange        `json:"files"`
	InterfacesConsumed         []InterfaceContract `json:"interfaces_consumed"`
	InterfacesProduced         []InterfaceContract `json:"interfaces_produced"`
	ImplementationInstructions []struct {
		Order       float64  `json:"order"`
		Files       []string `json:"files"`
		Instruction string   `json:"instruction"`
	} `json:"implementation_instructions"`
	PlannedChecks []struct {
		Kind           string   `json:"kind"`
		Command        string   `json:"command"`
		ExpectedResult string   `json:"expected_result"`
		SourceTestIDs  []string `json:"source_test_ids"`
	} `json:"planned_checks"`
	Acceptance   []AcceptanceCriterion `json:"acceptance"`
	Blockers     []string              `json:"blockers"`
	CodingPrompt *string               `json:"coding_prompt"`
}

type EffectiveRequirement struct {
	ID               string  `json:"id"`
	Kind             string  `json:"kind"`
	Area             string  `json:"area"`
	Statement        string  `json:"statement"`
	MeasurableTarget *string `json:"measurable_target"`
	Basis            Basis   `json:"basis"`
}

type OperationalTechnology struct {
	Stack []struct {
		ID         string        `json:"id"`
		Layer      string        `json:"layer"`
		Technology string        `json:"technology"`
		Version    *string       `json:"version"`
		Basis      Basis         `json:"basis"`
		AppliesTo  Applicability `json:"applies_to"`
	} `json:"stack"`
	Tools []struct {
		ID        string        `json:"id"`
		Category  string        `json:"category"`
		Name      string        `json:"name"`
		Purpose   string        `json:"purpose"`
		UsedIn    []string      `json:"used_in"`
		AppliesTo Applicability `json:"applies_to"`
	} `json:"tools"`
	Libraries []struct {
		ID         string        `json:"id"`
		Language   string        `json:"language"`
		Ecosystem  string        `json:"ecosystem"`
		Name       string        `json:"name"`
		Version    *string       `json:"version"`
		Capability string        `json:"capability"`
		UsedIn     []string      `json:"used_in"`
		Licence    *string       `json:"licence"`
		Concerns   []string      `json:"concerns"`
		Basis      Basis         `json:"basis"`
		AppliesTo  Applicability `json:"applies_to"`
	} `json:"libraries"`
	Services []struct {
		ID              string        `json:"id"`
		Name            string        `json:"name"`
		Direction       string        `json:"direction"`
		Protocol        string        `json:"protocol"`
		Authentication  string        `json:"authentication"`
		RateLimits      *string       `json:"rate_limits"`
		FailureBehavior string        `json:"failure_behavior"`
		OfflineStrategy string        `json:"offline_strategy"`
		ContractIDs     []string      `json:"contract_ids"`
		AppliesTo       Applicability `json:"applies_to"`
	} `json:"services"`
	Environment struct {
		Prerequisites []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"prerequisites"`
		Installation []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"installation"`
		VersionPinning []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"version_pinning"`
		CleanMachineSetup []struct {
			ID        string        `json:"id"`
			Text      string        `json:"text"`
			AppliesTo Applicability `json:"applies_to"`
		} `json:"clean_machine_setup"`
	} `json:"environment"`
	Commands struct {
		Build   *string `json:"build"`
		Test    *string `json:"test"`
		Lint    *string `json:"lint"`
		Run     *string `json:"run"`
		Release *string `json:"release"`
	} `json:"commands"`
	Security struct {
		Secrets               []string `json:"secrets"`
		DependencyUpdates     []string `json:"dependency_updates"`
		VulnerabilityScanning []string `json:"vulnerability_scanning"`
		DataHandling          []string `json:"data_handling"`
		Compliance            []string `json:"compliance"`
	} `json:"security"`
	Observability []Signal `json:"observability"`
}
