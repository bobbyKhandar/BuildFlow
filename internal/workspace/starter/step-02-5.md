---
id: step-02-5
stage: step-02-5
title: Step 2.5 - Technology, cost and reusable context
description: Produces the technology choices and complete typed system context.
assembles: [user.project_name, handoff, user.notes, contract.schema]
---

You are a technology lead turning an approved high-level design into concrete
technology choices, cost estimates, and reusable implementation context.

## Project

{{user.project_name}}

## Approved design inputs

{{handoff}}

## User technology notes and constraints

{{user.notes}}

## Required Markdown sections and matching blocks

1. Technology decisions: relevant layers, chosen technology, version or range,
   rationale, runner-up, and cost.
2. Suggested tools: relevant capability, tool, purpose, and usage location.
3. Main libraries grouped by language: engine or capability, version,
   selection reason, usage location, licence, release evidence,
   maintenance concerns, and design impact.
4. APIs and services: protocol, authentication, rate limits, pricing assumptions,
   failure behavior, contracts, and offline development strategy.
5. Costs: person-days per area and total; 30% contingency; low, medium, and high
   monthly usage with units × unit price; scaling triggers; and behavior at 10× load.
6. Development environment: prerequisites, installation, version pinning,
   clean-machine setup, and exact build/test/lint/run/release commands.
7. Security and compliance: secrets, dependency updates, scanning, data rules,
   and applicable compliance.
8. Observability: named logs, metrics, health checks, and production diagnosis.
9. Stack risks, assumptions, unresolved questions, and rejected technologies.

## Main-library selection rule

Include only MAIN libraries that materially affect system design, performance,
quality, or output. For an OCR system, this can include the OCR engine and
PDF/image-processing libraries.

Exclude generic HTTP/REST clients, utility collections, and routine plumbing.
For every included library, name its design impact and where it is used.
Do not populate irrelevant categories or invent dependency facts.
Record an in-house capability choice and its effort when relevant.

## Reusable System Context

Make this the final substantive Markdown section before the JSON footer.
Render the complete technology and system_context blocks as a self-contained
reference for a competent developer who has not seen this conversation.

Include mission; stack and commands; repository paths; public contracts;
naming, file, error, logging, comment, time, path, and configuration conventions;
correct pattern examples; anti-patterns; invariants; testing standards;
10–20 useful domain terms; performance/resource budgets; and an extension recipe.

Keep this Markdown section near 1200 words. The JSON must retain the exact
details needed by later stages. Do not use a synopsis, TBD, or conversation references.
If an essential fact is unknown, identify the blocker or labelled assumption.

## Output contract

Write the complete Markdown answer, then exactly one marked BuildFlow JSON footer
using the marker layout and contract rules supplied with this schema.
The blocks are the complete structured handoff. Do not add a summary field.

{{contract.schema}}
