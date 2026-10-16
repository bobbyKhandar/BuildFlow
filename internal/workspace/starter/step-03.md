---
id: step-03
stage: step-03
title: Step 3 - Low-level design
description: Produces complete implementation details and effective implementation context.
assembles: [user.project_name, handoff, user.notes, contract.schema]
---

You are a senior engineer producing a low-level design that independent
implementers can follow without unstated decisions.

## Project

{{user.project_name}}

## Approved design, technology, operational context, and budgets

{{handoff}}

## User design notes

{{user.notes}}

## Required Markdown sections and matching blocks

1. Effective implementation context: produce the complete effective operational
   context, including technology, commands, conventions, invariants, and budgets.
   Apply justified changes and preserve unchanged facts. Include effective linked
   records with stable IDs, applicability, ownership, and canonical references.
2. Modules: responsibility, separation rationale, dependencies, public contracts,
   and exact owned files, with existing/new status.
3. Types, signatures, schemas, and configuration: exact declarations, field names,
   types, versions, defaults, and validation. Use short idiomatic design sketches.
4. Algorithms: steps or pseudocode, complexity, input size, edge cases, and handling.
5. Error taxonomy: every fallible contract, detection point, error code,
   user message, logging, retries, and fatal behavior.
6. Concurrency and ordering: execution context, shared state, synchronization,
   race prevention, idempotency, and safe reruns.
7. State transitions: states, triggers, persistence, terminal states, and crash recovery.
8. UI or command surfaces: regions, loading/empty/populated/error/permission/offline
   states where applicable, exact copy, keyboard behavior, and destructive actions.
9. Observability: exact logs, metric names, health checks, and diagnostic use.
10. Tests: actual names, files, cases, fixtures, failure paths, commands,
    and required acceptance checks.
11. Acceptance criteria: an unambiguous checklist per module.
12. Requirement coverage and deliberate exclusions.
13. Implementation sequence: dependency order, parallel batches, and blocked work.
14. Design changes, risks, assumptions, and unresolved questions.

## Rules

Follow the pinned versions, layout, and conventions unless an explicit replacement
is necessary. Record its source, previous decision, replacement, and rationale.
Apply accepted replacements to implementation_context itself.

Every effective Stage 2 requirement must be assigned to modules or explicitly
excluded with a reason. Preserve identifiers.

Record every contract needed by Stage 4 here first; Stage 4 must not invent
contracts. Retain complete effective linked records, not just their IDs.

Do not omit detailed contracts from JSON on the assumption that later stages
will receive the Markdown answer.
Do not write placeholder implementations. Unknown decisions become questions
with recommended defaults.

## Output contract

Write the complete Markdown answer, then exactly one marked BuildFlow JSON footer
using the marker layout and contract rules supplied with this schema.
The blocks are the complete structured handoff. Do not add a summary field.

{{contract.schema}}
