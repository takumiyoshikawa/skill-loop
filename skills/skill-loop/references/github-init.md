# GitHub bootstrap

Use this reference when the user wants a setup for github issues.

## Goal

Create the smallest working GitHub issue loop:

- `tracker.repo` in `skill-loop.yml`
- `default_entrypoint: issue-check`
- starter skills for `issue-check`, `plan`, `implement`, and `review`

## Workflow

1. Inspect the repository for an existing `skill-loop.yml`.
2. Inspect the git remote and infer `tracker.repo` when it is safe to do so.
3. If there is no config, start from `assets/skill-loop.github.yml.tmpl`.
4. If there are no starter skills, create them from the templates in `assets/`.
5. Keep the first version conservative and short. Avoid advanced dispatcher settings unless the user asks.

## Defaults

- Use `issue-check` as the entrypoint.
- Keep issue state decisions in the skill graph, not in a large dispatcher config.
- Prefer the inner loop `issue-check -> plan -> implement -> review`.
- Make `plan` attach its implementation plan to the issue.
- Make `review` attach only important findings back to the issue.
- Prefer `.agents/skills/` as the output location for generated starter skills.

## Repo detection

If a remote like `git@github.com:owner/repo.git` or `https://github.com/owner/repo.git` exists, normalize it to `owner/repo`.

If repo detection is ambiguous, ask the user for the repository slug instead of guessing.

## Minimal bootstrap result

Create:

- `skill-loop.yml`
- `.agents/skills/issue-check/SKILL.md`
- `.agents/skills/plan/SKILL.md`
- `.agents/skills/implement/SKILL.md`
- `.agents/skills/review/SKILL.md`

Use the assets as templates and then tailor the wording to the repository.
