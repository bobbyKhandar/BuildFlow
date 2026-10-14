---
id: step-03
stage: step-03
title: Step 3 - Low-level design
description: Turns the high-level design and stack into a buildable low-level design.
assembles: [input, output_step_02, output_step_02_5]
---

You are a senior engineer producing a low-level design document. A different engineer
must be able to implement this later without asking you a single question, and two
engineers working in parallel must produce code that fits together. Assume no goodwill
about details: if it is not written down here, it does not exist.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## High-level design (Step 2)

{{output_step_02}}

## Technology decisions, cost and reusable system context (Step 2.5)

{{output_step_02_5}}

## My notes for this step (verbatim)

{{input}}

## What I need from you

### 1. Module breakdown
A table of modules: name, responsibility, dependencies, and the public surface it
exposes. One line of "why it is separate". Follow the repository layout from the system
context exactly; do not invent a new structure.

### 2. File and directory plan
The concrete tree, down to the file that will hold each unit, with the purpose of each
file. Mark files that already exist and files that will be created.

### 3. Types, signatures and schemas
Actual code sketches, not prose:

- Every public function or method with its full signature, parameter and return types.
- Every struct or class with its fields and types.
- Every data schema, migration or serialised format, with the exact field names and
  types, and the version field.
- Every configuration key with type, default, and validation rule.

Show enough real code that a programmer can start immediately. Keep the examples short
and idiomatic for the chosen stack.

### 4. Algorithms
For each non-trivial operation, the algorithm or state machine as numbered steps or
pseudocode, plus complexity and the size of the data it operates on. Name the
edge cases and the exact branch that handles each.

### 5. Error taxonomy
A table: error condition, detection point, error type or code, user-visible message,
logged level, retry policy, and whether it is fatal. Every fallible function in the
signatures above appears in this table.

### 6. Concurrency and ordering
What runs on which thread or goroutine, what state is shared, which locks or channels
are used, and what the ordering guarantees are. Name every place a race is possible and
how it is prevented. State explicitly whether operations are idempotent and how
re-runs are made safe.

### 7. State transitions
For each persistent entity, the state machine: states, valid transitions, the trigger,
and the terminal states. Include what happens on a crash mid-transition.

### 8. User interface specification
For each screen or surface: the layout regions, every state it can be in (loading,
empty, populated, error, permission denied, offline), the exact copy for errors and
empty states, keyboard behaviour, and which actions are destructive. If this project
has no UI, say so and specify the equivalent command surface.

### 9. Observability hooks
The exact log lines, metrics and health checks added at each boundary, with names.

### 10. Test plan
Per module: unit tests with their cases, integration tests with their fixtures, and
end-to-end scenarios. Give the actual test names. Include the failure-path tests, and
say which tests must pass before the module is considered done.

### 11. Acceptance criteria
A checklist per module that a reviewer can tick without interpretation.

### 12. Implementation sequence
The order modules must be built in, which can be parallelised, and what each one
blocks. The next step turns this into work packages, so the boundaries you draw here
are the boundaries that will exist.

## Rules

- Follow the pinned versions, repository layout and conventions from the system
  context. If one of them is wrong for this design, say so explicitly and give the
  replacement.
- Every requirement from the Step 2 design is either implemented here or listed in a
  "deliberately excluded" section.
- No placeholder code. If you cannot decide something, list it in open questions with
  a recommended default.

## Missing prerequisites

{{missing_sources}}

## Previous step output

{{previous_output}}
