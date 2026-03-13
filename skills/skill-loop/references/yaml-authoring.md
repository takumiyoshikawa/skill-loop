# YAML authoring

Use this reference when writing or revising `skill-loop.yml`.

## Core rules

- Keep the top level short.
- Prefer `tracker.repo` over a large block of ticket metadata.
- Use a small number of skills with clear router criteria.
- Treat the `skills` graph as the per-ticket inner loop.

## Preferred starter shape

```yaml
name: github-issue-loop
default_entrypoint: issue-check
router:
  runtime: codex
  model: gpt-5.4

tracker:
  repo: owner/repo

skills:
  issue-check:
    next:
      - id: start-plan
        criteria: A ready GitHub issue was selected and the repo is prepared for planning.
        skill: plan
      - id: stop
        criteria: No issue is ready or setup cannot be completed safely.
        done: true

  plan:
    next:
      - id: start-implement
        criteria: A concrete plan was prepared and attached to the issue.
        skill: implement
      - id: stop
        criteria: The issue is not actionable after planning.
        done: true

  implement:
    next:
      - id: send-review
        criteria: Implementation is ready for an explicit review pass.
        skill: review
      - id: revise-plan
        criteria: New information means the plan must be updated before implementation can continue safely.
        skill: plan
      - id: continue-implement
        criteria: More implementation work is still required before review.
        skill: implement

  review:
    next:
      - id: approve
        criteria: Review is satisfied.
        done: true
      - id: rework
        criteria: Review requires more implementation work.
        skill: implement
      - id: replan
        criteria: Review found a scope or approach problem that requires updating the plan first.
        skill: plan
```

## Authoring guidelines

- Use route `id`s plus `criteria`; let the router choose instead of embedding status markers in skill output.
- Prefer one small set of mutually exclusive routes per skill.
- Avoid over-configuring polling, labels, or workspace settings unless the repository needs them.
- If the user wants a `ghpm`-style flow, push issue selection and minimum work setup into `issue-check`, make `plan` post the plan to the issue, and make `review` post only important findings back to the issue.

## When updating existing YAML

- preserve the existing skill names when possible
- keep `default_entrypoint` stable unless the user wants a new entrypoint
- simplify before adding new top-level config
