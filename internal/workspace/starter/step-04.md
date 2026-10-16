---
id: step-04
stage: step-04
title: Step 4 - Work packages
description: Splits the low-level design into independently implementable, testable work packages.
assembles: [input, output_step_03]
---

You are a technical lead dividing the low-level design into a small number of
implementable, reviewable, and testable work packages.

Choose package boundaries, ownership, and sequencing. Do not redesign the system,
change the stack, introduce features, or discard acceptance criteria.
A future coding session receives the reusable system context and one selected
package, not this entire planning conversation.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## Low-level design (Step 3)

{{output_step_03}}

## My notes on packaging (verbatim)

{{input}}

## Missing prerequisites

{{missing_sources}}

## Rules for the split

1. Preserve requirement, decision, and acceptance IDs. Every in-scope acceptance
   criterion must have a responsible package.
2. Use the smallest coherent set of packages, often three to eight and normally
   fewer than twelve. Do not split merely to meet a count. Explain a larger set.
3. Each package delivers one reviewable outcome and is testable once its declared
   prerequisites are complete. Independent does not mean dependency-free.
4. Give each changed file one owner across the plan. Assign shared manifests,
   schemas, configuration, registration, and integration wiring explicitly.
   Use an owning prerequisite or integration package to avoid overlapping edits.
5. Dependencies must be explicit and acyclic. Name the prerequisite interfaces
   and verification evidence needed, not just another package's ID.
6. Include the relevant contracts, requirements, constraints, and design details
   inside each package. Do not rely on a document-wide preamble reaching Stage 5.
   The reusable context is not directly supplied to this stage; do not claim to
   have inspected it or invent rules missing from the low-level design.
7. Preserve evidence labels. Distinguish supplied facts, external facts actually
   verified, inferences, assumptions, and packaging decisions. Label effort estimates.
8. Flag contradictions and ask only questions that materially affect scope,
   architecture, safety, cost, or acceptance. Continue unaffected packages.
   Mark any package with a material unresolved dependency as not ready.
9. Do not invent repository files, runnable test targets, sources, or test results.
   Use existing checks or explicitly declared test-setup prerequisites.

## Required heading format

Start each package with exactly this form, using unique sequential IDs:

## P01 - <short imperative title>

Use no other P-number headings or standalone P-number/title lines.
Do not wrap the complete answer in a code fence.

## For each package, provide these subsections

Use these bold labels in this order:

- **Goal** — the observable outcome and the requirement IDs it serves.
- **Why now** — why its position in the dependency order is appropriate.
- **Prerequisites** — package IDs or "none", required interfaces, setup, and
  evidence that must exist before implementation.
- **Scope in** — a checklist of concrete deliverables.
- **Scope out** — explicit exclusions, including adjacent work.
- **Files** — exact repository-relative paths, create/modify status, and ownership.
  Label planned or unverified paths; avoid broad globs that conceal ownership.
- **Interfaces consumed** — required signatures, schemas, or commands, their
  producer, and relevant behavior.
- **Interfaces produced** — contracts this package establishes, including errors
  and compatibility requirements.
- **Implementation notes** — relevant algorithms, rules, invariants, edge cases,
  and approved decisions from the low-level design.
- **Tests** — meaningful cases, fixtures, expected results, and supported execution
  method or explicit setup prerequisite. Include relevant failure paths.
- **Done when** — acceptance IDs and measurable criteria, including integration
  checks where needed. Distinguish implementation complete from verification blocked.
- **Effort** — an estimate or range with its basis and largest uncertainty;
  state when there is insufficient information.
- **Risk** — the main implementation risk and an early detection or validation step.
- **Handoff** — the context and prerequisite evidence Stage 5 must read, what this
  package makes available afterward, and unresolved decisions affecting readiness.

A package's scope ends with its Handoff field. The closing cross-package summary
below is planning context, not additional implementation scope for the last package.

## Close the document with

Use the heading `## Cross-package summary`, followed by:

- **Dependency graph** — a table of package ID, depends on, and blocks.
- **Recommended build order** — including work that can run in parallel.
- **Integration points to verify first** — contracts and behaviors to check when
  packages meet.
- **Packages I deliberately did not create** — why that work belongs elsewhere
  or is outside the approved scope.

## Handoff

Tell Stage 5 to use exactly one selected package, its session notes, and the
Stage 2.5 Reusable System Context. List unresolved cross-package decisions and
affected package IDs. Do not select or implement a package on the user's behalf.

This final Handoff and the cross-package summary may accompany the last package
after import. They do not expand that package's scope.
