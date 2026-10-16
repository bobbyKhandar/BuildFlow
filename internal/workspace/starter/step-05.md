---
id: step-05
stage: step-05
title: Step 5 - One coding prompt for one package
description: Builds the single coding prompt for the selected package using the reusable system context.
assembles: [package_content, system_context, package_input]
---

Implement exactly one selected work package:
{{package_id}} - {{package_title}}.

Your authority is limited to this package's approved scope and file ownership.
Do not redesign the system, implement other packages, or expand the feature set.

## Required preparation

1. Read the full Reusable System Context, the complete selected package, and the
   session notes before changing code.
2. Read applicable repository instructions, including any AGENTS.md files, and
   inspect the relevant code, manifests, scripts, and tests. Follow the repository's
   existing instructions and conventions.
3. Confirm the package's prerequisites against the actual repository. Saved planning
   documents or completion labels are not evidence that code exists or tests passed.
4. If the reusable context or selected package is missing, empty, or materially
   incomplete, report the missing information before implementation.
5. Identify conflicts among the context, package, notes, repository instructions,
   and actual code. Record explicit user corrections. Do not silently resolve an
   important conflict by choosing a source or inventing a decision.
6. Ask only questions that materially affect scope, architecture, safety, cost,
   or acceptance. Continue unaffected work with labeled assumptions where safe.
   Do not guess through a blocking conflict.

## Project context supplied from Step 2.5

{{system_context}}

## Work package {{package_id}} - {{package_title}}

{{package_content}}

## My notes for this session (verbatim)

{{package_input}}

## Missing prerequisites

{{missing_sources}}

## Implementation rules

- Implement only {{package_id}}. Treat any cross-package summary or document-wide
  handoff attached to the package as context, not additional scope.
- Preserve settled decisions, interfaces, constraints, and invariants.
  If the package requires changing them, report the necessary change and its
  consequences before making the affected edit.
- Stay within the package's owned files. Report required changes to another owner's
  files rather than making unplanned edits.
- Use the established stack and dependency policy. Do not add or change dependencies
  outside the package's authorized scope.
- Follow existing code style, naming, error handling, configuration, and logging.
  Keep changes focused; avoid unrelated formatting, moves, or refactoring.
- Distinguish supplied claims, repository observations, externally verified facts,
  inferences, assumptions, and decisions made during implementation.
  Do not invent files, APIs, sources, applied changes, or verification results.
- Add or update meaningful tests for changed behavior and material failure paths,
  using the repository's supported tooling and the package's acceptance criteria.
  Do not impose a test for every branch or add an unrelated testing framework.
- Run checks that exist, apply to the change, and can run in the available environment.
  Verify proposed commands before using them. For unavailable checks, state the
  prerequisite and verification gap; do not fabricate a substitute success.
- Do not leave placeholder implementations for required package behavior.
  Report incomplete work explicitly.

## How to respond

Use these phase headings exactly. Keep explanations concise and do not repeat
the full context or package.

### Phase 1 - Confirm understanding
Summarize the outcome, owned files, consumed and produced interfaces, prerequisites,
and acceptance criteria. Identify material conflicts and assumptions.
If implementation is blocked, explain the blocker and the specific answer needed.

### Phase 2 - Plan
Give the ordered changes and affected files. Identify the main risk and the checks
that will provide evidence for acceptance.

### Phase 3 - Implement
Make the authorized repository changes. Explain non-obvious choices briefly.
If you cannot edit the repository, clearly distinguish proposed changes from
applied changes; do not claim implementation is complete.

### Phase 4 - Test
Add or update the relevant tests. State what behavior they cover and any test
setup required by the package. Do not add unrelated infrastructure.

### Phase 5 - Verify
Run the applicable supported checks and inspect their results.
Report each command or manual check, its actual outcome, and relevant evidence.
Distinguish passed, failed, and not run, with reasons. Do not present expected
output as observed output or claim success without execution evidence.
Avoid repeating checks already run unless subsequent changes require it.

### Phase 6 - Report
- Files created or modified and their purpose.
- Interfaces delivered or changed.
- Acceptance criteria satisfied, with evidence, and criteria still unverified.
- Tests and checks run, failures, and checks not run.
- Deviations, assumptions requiring confirmation, and remaining issues.
- Relevant out-of-scope findings, reported without implementing them.

End with **Handoff**: the verified interfaces and artifacts subsequent work may
use, any required upstream document corrections, and unresolved issues or
verification gaps. Do not start another package.
