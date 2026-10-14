---
id: step-05
stage: step-05
title: Step 5 - One coding prompt for one package
description: Builds the single coding prompt for the selected package using the reusable system context.
assembles: [package_content, system_context, package_input]
---

You are implementing **one work package, {{package_id}} - {{package_title}}**, inside an
existing project. You have the complete system context below. Follow it exactly. You are
not designing the system; you are extending it according to a design that is already
settled.

## Non-negotiable rules

1. Read the **Reusable System Context** in full before you write anything. It defines
   the stack, the repository layout, the conventions, the interfaces, the invariants and
   the testing bar. Where this prompt and the system context disagree, the system
   context wins, and you must say so in your report.
2. Implement **only {{package_id}}**. Do not refactor adjacent code, do not add
   dependencies that are not already pinned in the system context, and do not start the
   next package. Out-of-scope improvements go in a clearly labelled final list, nothing
   more.
3. Respect every listed invariant. If an invariant appears to block the package,
   stop and report it instead of breaking it.
4. Match the existing code style, naming, error handling and logging conventions
   exactly. Consistency with this codebase beats your own preferences.
5. Use the pinned language version, libraries and commands from the system context.
6. Every non-trivial branch needs a test. Every package needs its failure-path test.
7. Do not leave `TODO`, `FIXME`, or placeholder implementations. If something genuinely
   cannot be finished, leave it out and report it.
8. Keep changes reviewable: no drive-by reformatting, no unrelated file moves.

## Reusable System Context

{{system_context}}

{{missing_sources}}

## Work package {{package_id}} - {{package_title}}

{{package_content}}

## My notes for this session (verbatim)

{{package_input}}

## The reusable system context is not present

If the section above is missing, say so immediately and stop. Do not attempt the
implementation: the system context is what keeps this package consistent with the rest
of the project, and guessing it is how systems rot.

## How to respond

Work through these phases and label them in your answer.

### Phase 1 - Confirm understanding
In at most 200 words: what {{package_id}} must deliver, which files it owns, which
existing interfaces it consumes and produces, and which invariants constrain it. List
any assumption you had to make, or any contradiction you found in the package
description. If you found a blocking contradiction, stop here and report it.

### Phase 2 - Plan
The ordered implementation steps, each with the file it touches. Call out the risky
step. Keep it short; this is a plan, not an essay.

### Phase 3 - Implement
The actual code. Show complete, runnable implementations, not sketches. Include the
imports, the config or schema changes, and the migration if any. Explain the
non-obvious choices in one line each.

### Phase 4 - Test
The test code, with the command to run it. State clearly which tests you ran and
which you could not run, and why.

### Phase 5 - Verify
The exact commands to prove the package works: build, test, lint, and any manual check
worth doing. Quote the expected output.

### Phase 6 - Report
- Files created and files modified, with a one-line purpose each.
- Interfaces produced, so the next package knows what it may call.
- Test names added and what they cover.
- Deviations from the package description, each with the reason.
- Anything you deliberately left undone, and what is needed to finish it.
- Anything in an adjacent package that looks wrong. Do not fix it; report it.

Do not summarise the whole project, do not repeat the system context back to me, and do
not add features that were not asked for.

## Previous step output

{{previous_output}}
