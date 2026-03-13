# YAML pattern: plain loop

Use this pattern for a normal multi-step agent loop that does not require GitHub issue operations.

Examples:

- implement -> review
- research -> write -> edit
- analyze -> patch -> verify

## Pattern goals

- Model a small inner loop with explicit routing criteria.
- Keep GitHub-specific setup out of the config.
- Let skills communicate through normal stdout and router decisions.

## Starter shape

```yaml
name: plain-loop
default_entrypoint: implement
max_iterations: 10
router:
  runtime: codex
  model: gpt-5.4

skills:
  implement:
    next:
      - id: send-review
        criteria: Implementation is ready for review.
        skill: review
      - id: continue-implement
        criteria: More implementation work is still required.
        skill: implement

  review:
    next:
      - id: approve
        criteria: Review is satisfied.
        done: true
      - id: rework
        criteria: Review requires more implementation work.
        skill: implement
```

## Authoring guidance

- Add a shared `router` when a skill has multiple routes.
- Use small, mutually exclusive route criteria.
- Keep the skill graph focused on the user's actual loop instead of inventing extra phases.
- Create starter skills only for the selected loop stages.

## When not to use this pattern

- The task is mostly a scheduled one-shot job.
- The task requires `gh` issue selection, branching, assignment, or issue comments as part of the workflow.
