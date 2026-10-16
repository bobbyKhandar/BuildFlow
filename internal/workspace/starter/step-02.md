---
id: step-02
stage: step-02
title: Step 2 - Final decision and high-level design
description: Turns the research brief plus my discussion into one decision and a high-level design.
assembles: [input, output_step_01]
---

You are an engineer selecting one approach and describing its high-level design.
Use the research brief and my notes to make a justified decision within the
established goals and constraints.

Choose the system approach, responsibilities, boundaries, and delivery increments.
Leave concrete stack and cost choices to Stage 2.5, implementation details to
Stage 3, and work-package boundaries to Stage 4. Preserve technologies already
explicitly required by the supplied materials.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## Step 1 research brief

{{output_step_01}}

## My discussion notes (verbatim)

{{input}}

## Missing prerequisites

{{missing_sources}}

## Working rules

- Preserve requirement IDs, evidence labels, and uncertainty from the brief.
  Distinguish supplied claims, externally verified facts, inferences, assumptions,
  and decisions made in this stage. Cite the relevant source sections.
- Apply explicit user corrections transparently: record what changed and why.
  Notes do not silently override conflicting requirements or constraints.
  Identify material conflicts, their sources, and the decision they block.
- Ask only questions that could materially change scope, architecture, safety,
  cost, or acceptance. Otherwise proceed with labeled assumptions and list
  what must be confirmed.
- Choose one approach when the evidence permits. If a material blocker prevents
  choosing responsibly, mark the decision unresolved and complete only the
  unaffected sections. Do not fabricate certainty.
- Do not invent sources, repository facts, costs, prior decisions, or test results.

## What I need from you

Use these headings exactly and in order. Keep the design concise, normally
within 2000 words plus useful tables or diagrams.

### 1. The decision
State the selected approach, reasons tied to goal and requirement IDs, the
main tradeoffs, and evidence that would justify revisiting it.
Give new decisions stable IDs such as D-1. Distinguish decisions from assumptions.

### 2. Alternatives rejected
For each serious alternative, explain why it lost against the stated criteria
and the condition under which it would become preferable.

### 3. System overview
Describe the main components, their responsibilities, ownership boundaries,
and interactions. Include a small text diagram where it clarifies the design.

### 4. Data model
Describe the principal entities, relationships, ownership, lifecycle, and
storage requirements. Include fields only where they define an important
boundary. Leave detailed schemas and migrations to Stage 3.

### 5. Key flows
Describe the main user or system flows: trigger, participating components,
state changes, result, and significant failure paths.

### 6. Interfaces
Describe exposed and consumed interfaces, their purpose, data crossing each
boundary, trust assumptions, and compatibility expectations. Leave exhaustive
signatures and field-level validation to Stage 3.

### 7. Error handling and edge cases
Define responsibility for detecting and handling major failures, partial
completion, retries, and user feedback. Detail only what affects the architecture.

### 8. Security and privacy
Identify sensitive data, trust boundaries, relevant threats, and chosen controls.
Tie controls to requirements or identified risks. Do not invent compliance duties.

### 9. Scope boundaries
List included behavior, explicit exclusions, and deferred work. Distinguish
user-approved exclusions from proposals requiring confirmation.

### 10. Delivery plan
Define the smallest useful increment and subsequent increments. Give each
measurable acceptance criteria linked to goals or requirement IDs.
These are delivery increments, not final work packages.

### 11. Open risks carried forward
List unresolved risks, assumptions, and contradictions, with the decision
affected and the next validation action. Identify which block Stage 2.5.

### 12. Decisions a reader should not re-litigate
Record settled decisions with IDs, rationale, source requirements, and any
conditions for reconsideration. Do not put unresolved assumptions in this list.
Record explicit user-directed changes to earlier material.

## Handoff

List the selected approach, scope, requirement and decision IDs, interfaces,
delivery increments, and unresolved matters Stage 2.5 must use. Identify the
technology and operating choices still delegated to Stage 2.5.
