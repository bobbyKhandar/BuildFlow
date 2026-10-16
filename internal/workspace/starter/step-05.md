---
id: step-05
stage: step-05
title: Step 5 - Generate one package coding prompt
description: Writes a standalone coding prompt without implementing the package.
assembles: [user.project_name, user.target, handoff, user.package_notes, user.notes, contract.schema]
---

You are writing a coding prompt for a future coding session.
Your deliverable is the prompt itself. Do not implement the package, edit files,
run commands, write production implementations, or claim that checks passed.

## Project

{{user.project_name}}

## Selected target

{{user.target}}

## Selected operational context and work package

{{handoff}}

## User notes from this package's Stage 4 slot

{{user.package_notes}}

## User notes for this Stage 5 session

{{user.notes}}

## Your task

Write one standalone coding prompt containing all context the future coding
session needs. Include only the operational context selected for this package,
not the entire project context. Resolve ordinary wording into clear instructions.
Do not silently override package contracts or system invariants.

If essential context is missing, or notes contradict an invariant or package
boundary, explain the blocker, ask the necessary question, and return a blocked
artifact without a coding prompt.

## Required content inside the generated coding prompt

1. Identify the package and its single intended outcome.
2. Include the selected effective operational context, pinned stack, commands,
   layout, conventions, contracts, invariants, testing standards, and budgets.
3. Include scope in, scope out, prerequisite expectations, owned files,
   and interfaces consumed and produced.
4. Include the applicable implementation details and both sets of user notes.
5. Instruct the future coding session to confirm understanding and stop on
   missing prerequisites or blocking contradictions.
6. Give ordered implementation instructions and identify the risky step.
7. Instruct the future session to implement only this package, follow the
   project style, avoid unrelated changes, and avoid unapproved dependencies.
8. Specify the planned tests, including failure paths, verification commands,
   expected results, and acceptance criteria.
9. Require the future session to distinguish actual execution from suggested
   commands and to report deviations, unfinished work, and adjacent findings.

Do not execute any of those future-session instructions yourself.
Do not claim that the package is complete.

## Markdown answer

For a ready artifact, provide:
- Target and scope.
- Assumptions or unresolved non-blocking questions.
- A section titled "Coding prompt" containing the complete standalone prompt.

For a blocked artifact, provide the blockers and questions instead.

The coding_prompt JSON string must contain exactly the reusable prompt text,
without the surrounding BuildFlow explanation or JSON footer.

## Output contract

Write the complete Markdown answer, then exactly one marked BuildFlow JSON footer
using the marker layout and contract rules supplied with this schema.
Use the selected package target. Do not add a summary field.

{{contract.schema}}
