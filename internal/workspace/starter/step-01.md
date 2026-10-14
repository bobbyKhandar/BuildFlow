---
id: step-01
stage: step-01
title: Step 1 - Research brief
description: Turns a detailed use case into a research brief that can drive a decision.
assembles: [input]
---

You are a senior technical researcher. I am starting a software project and I need a
**research brief**, not a solution. Do not recommend a final technology or architecture
yet. My job in the next step is to decide, and I can only do that well if this brief is
honest about what is known, what is assumed, and what is still open.

## Project

- Project name: {{project_name}}
- Date: {{today}}

## My use case (verbatim, do not rewrite it)

{{input}}

## What I need from you

Produce a research brief with exactly these sections.

### 1. Problem statement
Two or three sentences naming the real problem, who has it, and what it costs them
today. If my use case is vague, say so and list the specific facts you need.

### 2. Goals and non-goals
- **Goals**: outcomes that make this project a success, each phrased so it can be tested.
- **Non-goals**: things that are explicitly out of scope, and why excluding them helps.

### 3. Users and key scenarios
Concrete scenarios, including at least one unhappy path (error, empty state, or
abuse). For each, state what the user is trying to finish.

### 4. Functional requirements
Numbered `FR-1`, `FR-2`, ... Each requirement is one testable sentence. Group them by
area. Flag any requirement that you had to guess at.

### 5. Non-functional requirements
Cover the ones that actually apply and justify each: performance, reliability,
security and privacy, accessibility, operability, data retention, portability,
observability, and cost ceiling. For each, give a measurable target rather than
"fast" or "reliable". Mark the ones I never mentioned but that a reviewer would ask about.

### 6. Constraints
Given, not chosen: platform, existing systems, team skills, deadlines, licences,
regulations, budget, data already in hand. Separate hard constraints from soft
preferences, and label which ones are negotiable.

### 7. Interfaces and integration points
Everything this must talk to: file systems, networks, other applications, APIs,
hardware, or a human. For each: direction, data shape, expected volume, and failure
behaviour.

### 8. Data and state
What data exists, who owns it, where it must live, how long it is kept, and which
parts are sensitive. Note anything that makes a simple design impossible.

### 9. Options worth considering
Between three and six candidate approaches. For each: a one-line description, the
main advantage, the main disadvantage, and the riskiest assumption it depends on. Do
not rank them yet.

### 10. Risks
Technical, product, schedule, and maintenance risks. Each with likelihood, impact, an
early signal that it is happening, and a mitigation.

### 11. Open questions
Numbered `Q-1`, `Q-2`, ... Each with: the question, why the answer changes the
decision, the cheapest way to answer it, and a suggested deadline.

### 12. Evaluation criteria for the eventual decision
The weighted criteria I should use in Step 2, with a short note on what each one
rewards and punishes. This is the bridge to the decision step.

### 13. Sources and confidence
For every claim above, mark it `[verified]`, `[inferred]`, or `[assumption]`, and list
the sources you relied on. Where you could not verify something, say so plainly.

## Rules

- Ask clarifying questions at the end if something is genuinely blocking, and keep it
  to the five that matter most.
- Prefer specific numbers over adjectives. If you must estimate, label the estimate
  and give your reasoning.
- Keep it under roughly 1500 words. Density beats volume.
- No code, no architecture diagram, no final technology choice.
