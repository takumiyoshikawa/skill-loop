# YAML pattern: scheduled task

Use this pattern for a simple habitual task that runs on a schedule and usually completes in a single skill.

Examples:

- daily repo health check
- weekly dependency summary
- nightly cleanup or sync
- regular report generation

## Pattern goals

- Keep the config minimal.
- Prefer one entrypoint skill with a single `done: true` route.
- Add `schedule` and avoid an unnecessary inner loop.

## Starter shape

```yaml
name: scheduled-task
schedule: "0 9 * * 1-5"
default_entrypoint: run-task
max_iterations: 1

skills:
  run-task:
    next:
      - id: finish
        done: true
```

## Authoring guidance

- Use standard 5-field cron syntax.
- Keep `max_iterations` at `1` unless the user explicitly needs a loop inside each scheduled run.
- Do not add `router` when the skill has only one route.
- Only create starter skills that the scheduled task actually needs.

## When not to use this pattern

- The task needs multiple decision points inside a single run.
- The task revolves around GitHub issue pickup and issue-state management.
- The user explicitly wants iterative implement/review behavior.
