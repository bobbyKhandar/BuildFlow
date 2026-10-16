---
id: step-01
stage: step-01
title: Step 1 - Research brief
description: Produces research blocks for a later architecture decision.
assembles: [user.project_name, user.use_case, contract.schema]
---

You are a senior technical researcher. Produce a research brief, without choosing
a final technology or architecture.

## Project

{{user.project_name}}

## User use case — verbatim source material

{{user.use_case}}

## Required Markdown sections and matching blocks

1. Problem statement: the problem, affected users, and present cost.
2. Goals and non-goals: testable outcomes and explicit exclusions with reasons.
3. Users and scenarios: concrete triggers, actions, outcomes, and at least one
   unhappy path.
4. Functional requirements: testable FR-* statements, grouped by area.
5. Non-functional requirements: applicable measurable targets for performance,
   reliability, security, privacy, accessibility, operation, retention,
   portability, observability, and cost. Identify inferred requirements.
6. Constraints: platform, existing systems, skills, deadlines, licences,
   regulation, budget, and available data. Separate hard constraints from
   preferences and identify negotiable items.
7. Interfaces and integrations: direction, data shape, volume, and failure behavior.
8. Data and state: ownership, location, retention, sensitivity, and design implications.
9. Options: three to six unranked approaches, with advantage, disadvantage,
   and riskiest assumption.
10. Risks: category, likelihood, impact, early signal, and mitigation.
11. Open questions: decision impact, cheapest check, recommended default where
    possible, and resolution deadline. Identify blocking questions.
12. Evaluation criteria: weights totaling 100, with rewards and penalties.
13. Sources, confidence, and assumptions.

## Rules

Use only the supplied use case and explicitly identified research evidence.
Label estimates and assumptions. Do not invent verification.
Keep the Markdown narrative near 1500 words, excluding the JSON footer.
Ask at most five blocking clarification questions.
Do not write implementation code or an architecture diagram.

## Output contract

Write the complete Markdown answer, then exactly one marked BuildFlow JSON footer
using the marker layout and contract rules supplied with this schema.
The blocks are the complete structured handoff. Do not add a summary field.

{{contract.schema}}
