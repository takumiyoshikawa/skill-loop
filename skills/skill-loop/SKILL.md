---
name: skill-loop
description: Use when bootstrapping skill-loop in a repository, creating a GitHub issue loop starter, or writing and updating skill-loop.yml and starter skills.
---

# skill-loop

Use this skill for two cases:

- bootstrap skill-loop into a repository with a GitHub issue workflow
- write, simplify, or migrate `skill-loop.yml`

## Modes

- GitHub bootstrap: read `references/github-init.md`
- YAML authoring or migration: read `references/yaml-authoring.md`

## Working style

- Keep the generated setup minimal.
- Prefer `tracker.repo` plus a small `skills:` graph.
- Prefer an `issue-check` entrypoint that uses `gh` to choose the next ready issue and complete the minimum setup before planning.
- Reuse templates from `assets/` instead of writing large starter files from scratch.
- Default to `.claude/skills/` for generated starter skills unless the repository already uses another skill directory.

## Expected outputs

- create or update `skill-loop.yml`
- create starter skills such as `issue-check`, `plan`, `implement`, and `review`
- explain the execution flow in terms of `gh` issue discovery, issue setup, planning, implementation, and review
