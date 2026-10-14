---
id: step-02-5
stage: step-02-5
title: Step 2.5 - Tech stack, packages, cost and reusable system context
description: Locks the technology choices and produces the reusable system context injected into every package prompt.
assembles: [input, output_step_02]
---

You are a technology lead converting a high-level design into a concrete, costed
technology decision. The most important output is the **Reusable System Context**
section at the end: it is the durable knowledge that will be injected into every
future coding session for this project, so it must be self-contained.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## High-level design from Step 2

{{output_step_02}}

## My technology notes and constraints (verbatim)

{{input}}

## What I need from you

### 1. Technology decisions
One table per layer of the system: layer, chosen technology, exact version or version
range, why this one over the runner-up, and what it costs. Layers to consider where
applicable: language and runtime, build tooling, UI or front end, HTTP or transport,
persistence, search, validation, testing, packaging and distribution, logging and
metrics, authentication, and deployment target.

### 2. Dependency list
A table of every third-party package you expect to use: name, version pin, what it is
for, licence, last release date, maintainer health, and whether it is critical or
convenience. Mark anything abandoned, single-maintainer, or licence-encumbered. If a
capability is better written in-house than pulled in, say so and give the estimate.

### 3. APIs and services
Every external service or API the system will call, plus every one it exposes. For
each: protocol, authentication, rate limits, cost model, and the failure behaviour we
must implement. Include a stubbing or offline development strategy.

### 4. Cost estimates
Three tables, with arithmetic shown:

- **Build cost**: person-days per work area, with a total, plus a 30% contingency.
- **Running cost per month at low / medium / high usage**: each line item as
  `units x unit price = amount`. Include hosting, storage, third-party APIs,
  licences, observability, and any egress.
- **Scaling triggers**: the metric, the threshold, the cost of the next step, and what
  to do at that threshold.

State your assumptions about volume explicitly, and show what breaks at 10x.

### 5. Development environment
The exact toolchain, how it is installed and pinned, the commands for build, test,
lint, run, and release, and how a new developer gets from a clean machine to a running
app. This list is authoritative: Step 5 coding prompts will assume it.

### 6. Security and compliance
Secrets handling, dependency update policy, vulnerability scanning, data handling rules,
and anything a security reviewer would demand for this design.

### 7. Observability
The logs, metrics, and health signals to emit, with names and the level each is used
at. Include how a failure will be diagnosed in production.

### 8. Risks and mitigations
Technical risk register for the chosen stack, each with an early detection signal.

### 9. Rejected technologies
What you deliberately did not choose and the one condition that would reverse it.

## Reusable System Context

End your answer with a section titled exactly `## Reusable System Context`. This is the
most important part of your response: it will be extracted and re-injected into every
future coding prompt for this project, including prompts written by people who have
never seen this conversation.

Write it for a competent developer who has never seen this project, and include:

1. **Mission** - three sentences on what the system does and what it must never do.
2. **Stack** - language, runtime, key libraries with pinned versions, and the build,
   test, lint and run commands.
3. **Repository layout** - the directories that matter and what belongs in each, with
   real example paths.
4. **Interfaces and contracts** - the public functions, types, endpoints or commands
   other code calls, with signatures. This is what lets independent work packages
   agree.
5. **Conventions** - naming, file layout, error handling, logging, comment style,
   time and path handling, configuration.
6. **Patterns to follow** - the two or three patterns that appear repeatedly, shown as
   short correct examples.
7. **Anti-patterns** - what must never be done, and why, each stated as a rule.
8. **Invariants and constraints** - the things that must always be true: thread safety,
   ordering, atomicity, no network on the UI thread, and so on.
9. **Testing standards** - the framework, what level tests what, naming, and the bar for
   "done".
10. **Glossary** - ten to twenty domain terms used consistently across packages.
11. **Performance and resource budgets** - the numbers a change must respect.
12. **Extension recipe** - the exact steps to add a new feature, module, or package so
    every work package extends the system the same way.

Rules for this section: no placeholders, no "TBD", no references to this conversation,
and no more than 1200 words. Write it as if it will outlive the current design.

## Missing prerequisites

{{missing_sources}}

## Previous step output

{{previous_output}}
