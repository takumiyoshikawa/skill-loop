# YAML authoring

Use this reference when writing or revising `skill-loop.yml`.

This reference is a router, not the final pattern guide. First choose the workflow shape, then read only the matching reference:

1. Habitual scheduled task with no meaningful inner loop
   - Read `references/yaml-pattern-scheduled.md`
2. GitHub issue throughput or issue triage loop
   - Do not hand-author that YAML here
   - Use `references/github-init.md`
3. Normal agent loop without GitHub issue management
   - Read `references/yaml-pattern-loop.md`

## Selection rules

- If the user mainly wants a cron-like recurring check, report, cleanup, or sync job, choose the scheduled pattern.
- If the user wants to pick up GitHub issues, assign work, branch from issues, or attach plans and review findings to issues, choose the GitHub bootstrap path and use `github-init`.
- If the user wants repeated implementation, analysis, or review passes but does not need `gh` issue operations, choose the plain loop pattern.
- If multiple patterns seem possible, prefer the smallest one that satisfies the request.

## Core rules for any pattern

- Keep the top level short.
- Use a small number of skills with clear router criteria.
- Prefer simple routes over large dispatcher logic.
- Preserve existing names when updating an existing workflow unless the user wants a rename.
- Keep `default_entrypoint` stable unless the user asks for a new entrypoint.
- Simplify before adding new top-level config.

## Output expectation

- State which pattern was selected and why.
- Only generate the YAML and starter skills required for that pattern.
