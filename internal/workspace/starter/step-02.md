---
id: step-02
stage: step-02
title: Step 2 - Final decision and high-level design
description: Turns the research brief plus my discussion into one decision and a high-level design.
assembles: [input, output_step_01]
---

You are a staff engineer making and defending an architecture decision. I need a
**decision** and a **high-level design** that a team could build. I will not challenge
you on this until Step 3, so commit to a position.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## Step 1 research brief

{{output_step_01}}

## My discussion notes (verbatim, they override the brief)

{{input}}

## What I need from you

### 1. The decision
State the decision in one sentence, in plain language. Then:

- **Chosen approach**, in two or three sentences.
- **Why this one**: the three strongest reasons, each tied to a specific finding or
  constraint above.
- **What I give up**, stated honestly.
- **What would change this decision**: the concrete finding that would flip it.

### 2. Alternatives rejected
For each serious alternative, one short paragraph: why it was tempting, the specific
reason it lost, and the condition under which it would become the better choice. This
section is what I will read in six months, so be precise.

### 3. System overview
Components, their responsibilities, and the direction of every interaction. Use a text
diagram, and keep the boundaries honest: name exactly what each component owns.

### 4. Data model
The entities, their fields with types, the relationships, and the ownership rules. Give
the storage medium and the reasoning. Call out anything that must not be normalised or
anything that must never be deleted.

### 5. Key flows
The three to five flows that matter most, each as an ordered list: trigger, steps,
state changes, and every failure branch with its user-visible result.

### 6. Interfaces
Every interface the system exposes or consumes, with the shape of the data crossing it.
Prefer concrete field lists over prose. Note the version-stability expectation of each.

### 7. Error handling and edge cases
The error taxonomy: which layer detects what, what the user sees, what is logged, what
is retried, and what is fatal. Cover the partial-failure cases explicitly.

### 8. Security and privacy
Threats you considered, the controls chosen, the data that is sensitive, and what
"trust the local machine" or "trust the user" means for this design.

### 9. Scope boundaries
In scope for this version, explicitly deferred, and never. The deferred list should be
specific enough to stop scope creep in review.

### 10. Delivery plan
The smallest slice that is genuinely useful, then the increments after it, each with its
own acceptance criteria.

### 11. Open risks carried forward
Only the risks that survive the decision, with the check that will confirm or kill each
one early. These become the input to Step 2.5.

### 12. Decisions a reader should not re-litigate
List the settled questions with their one-line rationale, so future work does not reopen
them.

## Rules

- Pick one option. No hedging menus.
- Where the brief is silent, state the assumption you made and keep a list of them.
- Use tables where they compress better than sentences.
- Under roughly 2000 words plus the diagram and tables.

## Previous step output

{{previous_output}}
