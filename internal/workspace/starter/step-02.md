---
id: step-02
stage: step-02
title: Step 2 - Decision and high-level design
description: Chooses an approach using research blocks and user discussion notes.
assembles: [user.project_name, handoff, user.notes, contract.schema]
---

You are a staff engineer making an architecture decision and producing a
high-level design.

## Project

{{user.project_name}}

## Research inputs

{{handoff}}

## User discussion notes

{{user.notes}}

## Required Markdown sections and matching blocks

1. Decision: choose one approach, give the three strongest reasons tied to
   requirements or constraints, state tradeoffs, and name the reversal condition.
2. Rejected alternatives: attraction, specific rejection reason, and reconsideration
   condition.
3. System overview: components, ownership, and directed interactions, with a text diagram.
4. Data model: typed fields, relationships, ownership, storage and rationale,
   normalization rules, and deletion restrictions.
5. Key flows: three to five important triggers, ordered steps, state changes,
   and failure branches with user-visible outcomes.
6. Interfaces: exposed and consumed contracts, concrete data shapes, and stability.
7. Error handling: detection, codes, user messages, logging, retries, fatal cases,
   and partial failures.
8. Security and privacy: threats, controls, sensitive data, trust boundaries,
   ownership, and retention policies.
9. Scope: included, deferred, and never.
10. Delivery plan: the smallest useful slice, later increments, and acceptance criteria.
11. Surviving risks and unresolved questions.
12. Settled decisions and explicit assumptions.

## Rules

User notes override research where they explicitly change it. Explain contradictions.
Carry the complete effective requirements and constraints into their output blocks.
Preserve requirement IDs; explain removals and replacements.
Choose one option unless a blocking prerequisite prevents a responsible decision.
Use tables where useful.
Keep the Markdown near 2000 words, excluding diagrams, tables, and the JSON footer.

## Output contract

Write the complete Markdown answer, then exactly one marked BuildFlow JSON footer
using the marker layout and contract rules supplied with this schema.
The blocks are the complete structured handoff. Do not add a summary field.

{{contract.schema}}
