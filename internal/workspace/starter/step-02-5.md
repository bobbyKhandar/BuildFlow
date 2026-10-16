---
id: step-02-5
stage: step-02-5
title: Step 2.5 - Tech stack, packages, cost and reusable system context
description: Locks the technology choices and produces the reusable system context injected into every package prompt.
assembles: [input, output_step_02]
---

You are a technology lead turning the chosen high-level design into concrete
technology decisions, cost estimates, and a reusable context for future sessions.

Choose the stack and durable project rules supported by the supplied design,
constraints, and available repository evidence. Do not redesign the product,
expand scope, or invent implementation details reserved for Stage 3.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## High-level design from Step 2

{{output_step_02}}

## My technology notes and constraints (verbatim)

{{input}}

## Missing prerequisites

{{missing_sources}}

## Working rules

- Preserve upstream decisions and requirement IDs. Record explicit user corrections.
  Flag material contradictions with their source sections; do not silently choose
  a winner or replace an architectural decision.
- Distinguish [supplied], [verified external], [inference], and [assumption].
  Mark new technology choices as decisions and explain their evidence and rationale.
- Verify changing external information when tools permit. Cite the actual official
  source and access date for versions, prices, licences, limits, or support claims.
  If verification is unavailable, say so; do not invent exact values or sources.
- Distinguish observed repository facts, supplied repository descriptions, and
  planned structure. A proposed command is not an existing runnable command.
- Ask only questions that materially affect scope, architecture, safety, cost,
  or acceptance. Continue unaffected work with labeled assumptions.
  Do not mark a material unresolved choice as pinned.
- Include only applicable layers and services. Do not add infrastructure, tools,
  policies, or dependencies merely to fill a section.

## What I need from you

Use the following headings exactly and in order.

### 1. Technology decisions
Give a concise table: layer, choice, version or supported range, evidence/status,
requirement or decision served, tradeoff, and cost implications.
Keep supplied pins unless an explicit change is recorded. Identify unresolved
pins instead of inventing versions.

### 2. Dependency list
List direct dependencies needed by the design: purpose, version status, licence,
criticality, and material maintenance risks. Include release or maintainer facts
only when verified. Distinguish required dependencies from optional proposals.

### 3. APIs and services
Identify external services and public interfaces. State known protocols,
authentication, limits, cost model, failure behavior, and local testing or
stubbing approach. Keep unknown contracts explicit.

### 4. Cost estimates
Provide:
- Build cost: effort ranges, work areas, estimation basis, and uncertainty.
- Monthly running cost: low, medium, and high usage scenarios where meaningful,
  with units × unit price, currency, region, pricing date/source, and exclusions.
- Scaling triggers: relevant metrics, justified thresholds, and consequences.

Label workload and effort assumptions. Show contingency separately and justify it;
a percentage is a planning assumption, not a verified fact.
For unknown prices, show the formula and missing input instead of a fabricated
total. Describe sensitivity to the largest uncertain inputs, including 10× usage
where meaningful. Separate monetary cost from engineering effort.

### 5. Development environment
Describe the toolchain, pinning method, configuration, and setup.
List build, test, lint, run, and release commands only where supported by repository
evidence or an explicit planned setup decision. Label existing versus planned.
Do not imply a command has run successfully.

### 6. Security and compliance
Specify supported rules for secrets, data handling, dependencies, and relevant
security checks. Separate established obligations from recommendations or unknowns.

### 7. Observability
Define signals needed to diagnose the design's important failures. Prefer existing
conventions and capabilities; do not add an observability platform without a reason.

### 8. Risks and mitigations
Record stack and operating risks, assumptions, material conflicts, early signals,
and validation actions. Identify unresolved choices that block implementation.

### 9. Rejected technologies
Record meaningful alternatives rejected, the reason, and what would justify
reconsideration. Do not invent a comparison history.

## Reusable System Context

Write this exact level-two heading once. Everything following it must belong to
the reusable context. Use bold labels or level-three-or-deeper headings inside
it, including the final Handoff. Do not introduce another level-one or level-two
heading afterward.

Keep this section self-contained and normally within 1200 words. Include durable
facts and rules supported by the supplied materials or decisions explicitly made
above. Preserve provenance and unresolved status where it matters.

Use these labels in order:

1. **Mission** — purpose, users, scope, and explicit non-goals.
2. **Stack** — chosen technologies, supported pins, and commands with their
   existing or planned status.
3. **Repository layout** — supported paths and responsibilities. Clearly identify
   planned paths; never describe an uninspected path as existing.
4. **Interfaces and contracts** — established boundaries and contracts that
   packages must share. Leave unestablished signatures to Stage 3.
5. **Conventions** — supported naming, errors, configuration, logging, data,
   time, and path rules relevant to this project.
6. **Patterns to follow** — established patterns and source examples when available.
   Do not invent examples and describe them as existing repository practice.
7. **Anti-patterns** — specific prohibited behavior with its project-specific reason.
8. **Invariants and constraints** — rules that must remain true, tied to requirements
   or decisions.
9. **Testing standards** — supported tooling, relevant test levels, execution
   prerequisites, and evidence required before claiming completion.
10. **Glossary** — only domain terms needed to interpret interfaces and requirements.
11. **Performance and resource budgets** — supplied or explicitly adopted limits,
    measurement conditions, and their status.
12. **Extension recipe** — supported steps for extending the system consistently,
    including interface, registration, and testing rules where established.

If a category lacks supported information, say "Not established in supplied
materials" and identify any necessary follow-up in the handoff. Do not fill gaps
with generic rules, invented paths, or speculative facts. Keep volatile pricing
details and rejected alternatives outside this reusable section.

### Handoff

State what Stage 3 should use: chosen stack, durable rules, interfaces, invariants,
and requirement/decision IDs. List unresolved choices, verification needs, and
whether the context is ready for implementation planning. This handoff remains
inside the extracted Reusable System Context.
