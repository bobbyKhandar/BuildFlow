---
id: step-04
stage: step-04
title: Step 4 - Work packages
description: Splits the low-level design into independently implementable, testable work packages.
assembles: [input, output_step_03]
---

You are a technical lead breaking a low-level design into work packages. Each package
will be handed to a fresh AI coding session that has seen only this project, the
reusable system context, and that one package. Your job is to make that possible.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## Low-level design (Step 3)

{{output_step_03}}

## My notes on packaging (verbatim)

{{input}}

## Rules for the split

1. **One outcome per package.** If a package cannot be finished and verified on its
   own, split it again.
2. **Sequencing must be explicit.** State which packages must exist first, and why.
3. **Boundaries must be real.** A package is not a time slice. A package owns files and
   exposes interfaces; it does not "also touch" other packages' files.
4. **Every package must be testable independently**, with its own test target.
5. **Keep the count low.** Fewer, well-bounded packages beat many small ones. Fewer than
   about twelve unless the design genuinely demands more.
6. **No package may require unstated context.** If implementing it needs a decision that
   is not written here or in the system context, the package is not ready.

## Required heading format

Start every package with a heading that looks exactly like this, one per package, in
order, with no other P-number headings anywhere in the document:

```markdown
## P01 - <short imperative title>
```

## For each package, provide these subsections

- **Goal** - one sentence: the outcome this package delivers.
- **Why now** - what depends on it, or what depends on it being absent.
- **Prerequisites** - the ids of packages that must be complete first, or "none".
- **Scope in** - the concrete deliverables, as a checklist.
- **Scope out** - what this package deliberately does not do, so it cannot grow.
- **Files** - the exact paths this package creates or modifies. Full relative paths.
- **Interfaces consumed** - the existing functions, types, endpoints, or commands this
  package calls, with their signatures.
- **Interfaces produced** - what other packages may call afterwards, with signatures.
  This is the contract; be precise, because two packages may be built in parallel.
- **Implementation notes** - the algorithms, decisions, and gotchas from the low-level
  design that apply here, expanded to the point where no guessing is needed.
- **Tests** - the test files, the test names, and what each one asserts. Include at
  least one failure-path test.
- **Done when** - the acceptance checklist a reviewer can tick.
- **Effort** - person-days, with the single biggest unknown called out.
- **Risk** - the likeliest way this package goes wrong, and the early signal.

## Close the document with

- **Dependency graph**: a table of package id, depends on, and blocks.
- **Recommended build order**, with what can run in parallel.
- **Integration points to verify first**, once two packages meet.
- **Packages I deliberately did not create**, and why the work belongs elsewhere.

## Missing prerequisites

{{missing_sources}}

## Previous step output

{{previous_output}}
