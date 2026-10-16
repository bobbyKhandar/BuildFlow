---
id: step-03
stage: step-03
title: Step 3 - Low-level design
description: Turns the high-level design and stack into a buildable low-level design.
assembles: [input, output_step_02, output_step_02_5]
---

You are an engineer producing an implementable and testable low-level design.
Translate the selected architecture and reusable system context into precise
implementation decisions. Do not reopen the chosen approach, stack, or scope.
Stage 4 will divide this design into work packages; do not perform that split here.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## High-level design (Step 2)

{{output_step_02}}

## Technology decisions, cost and reusable system context (Step 2.5)

{{output_step_02_5}}

## My notes for this step (verbatim)

{{input}}

## Missing prerequisites

{{missing_sources}}

## Working rules

- Preserve upstream requirement and decision IDs, conventions, interfaces, and
  invariants. Record explicit user corrections. If implementation reveals a
  necessary upstream change, propose that amendment and its consequences
  separately; do not silently adopt it.
- Flag contradictions with their sources. Ask only questions that materially
  affect scope, architecture, safety, cost, or acceptance. Mark affected work
  blocked; continue independent details with labeled assumptions.
- Distinguish supplied claims, verified external facts, inferences, assumptions,
  and new implementation-design decisions. Preserve original evidence status.
- Do not invent existing files, repository conventions, commands, sources, or
  successful checks. Mark paths and interfaces as existing only with evidence;
  otherwise identify them as planned or unverified.
- Stage 4 receives this document and packaging notes, not the full earlier
  history. Carry forward the requirements, decisions, relevant stack rules,
  interfaces, and invariants needed to package the work correctly.

## What I need from you

Use these headings exactly and in order. Detail behavior and boundaries enough
to implement them without filling the document with complete application code.

### 1. Module breakdown
For each module, give responsibility, dependencies, owned state, public surface,
and the requirements and upstream decisions it implements.

### 2. File and directory plan
Use the established layout. Give concrete paths and purposes, distinguishing
observed existing files, supplied descriptions, and planned files.
Identify shared registration, configuration, schema, and dependency files.

### 3. Types, signatures and schemas
Specify public contracts, exchanged data, validation, configuration, defaults,
and compatibility or migration behavior where applicable.
Use concise code sketches in the chosen stack. Label sketches as design,
not tested implementations. Do not add unnecessary abstraction or version fields.

### 4. Algorithms
Specify nontrivial algorithms or state machines, relevant input bounds,
edge cases, and complexity where it affects requirements.

### 5. Error taxonomy
For meaningful failure conditions, state detection point, error representation,
user-visible result, logging, retry behavior, and recovery. Cover partial failures.

### 6. Concurrency and ordering
Specify execution and ownership rules, shared state, synchronization,
ordering, cancellation, idempotency, and retry safety where relevant.
Do not assume a concurrency model unsupported by the chosen stack.

### 7. State transitions
Describe relevant entity or workflow states, allowed transitions, triggers,
persistence, and recovery after interrupted operations.

### 8. User interface specification
For each applicable surface, describe actions, validation, loading, empty,
success, error, permission, and offline states. Include relevant keyboard and
accessibility behavior. Mark inapplicable states instead of inventing them.
For a non-UI system, specify the corresponding command or API behavior.

### 9. Observability hooks
Specify diagnostic events and measurements required by the chosen design,
their data and levels, and sensitive information that must be excluded.

### 10. Test plan
Map meaningful tests to requirements, contracts, risks, and failure paths.
Specify fixtures, test level, expected results, and execution prerequisites.
Give proposed test names where useful. Distinguish supported commands from
test infrastructure that must first be implemented. Do not claim tests ran.

### 11. Acceptance criteria
Assign stable acceptance IDs. For each criterion, identify the requirement,
observable behavior, conditions, verification method, and pass condition.
Map every in-scope requirement to criteria or an explicitly unresolved issue.
List previously approved exclusions separately; propose new exclusions for
confirmation rather than dropping requirements.

### 12. Implementation sequence
Order implementation by dependencies. Identify prerequisite contracts, setup,
shared-file integration, and work that can proceed independently.
Describe useful review checkpoints without deciding final package boundaries.

## Handoff

List what Stage 4 must use: requirement and acceptance IDs, module/file ownership,
contracts, relevant reusable-context rules, dependency order, and test prerequisites.
List unresolved decisions and the specific work they block. State whether the
design is ready to package.
