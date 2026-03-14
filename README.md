# skill-loop

An agentic skill orchestrator for coding-agent CLIs.

Chain multiple coding-agent skills together in a loop-based workflow. Define skills, routing criteria, and iteration limits in a simple YAML config. skill-loop executes each skill, runs a separate LLM router on the skill's stdout, and advances the workflow from that routing decision.

## How it works

```
          +--------+         +----------+
          | 1-impl | ------> | 2-review |
          +--------+         +----------+
              ^                 |
              |                 v
              +----------- router -----------+
                         rework / approve
```

1. skill-loop reads a YAML config that defines skills and routing rules
2. Starting from `--entrypoint` (if provided) or `default_entrypoint`, it invokes the configured agent (`claude`, `codex`, or `opencode`) with each skill
3. Each skill produces stdout; a separate router agent evaluates that stdout against the configured route `criteria`
4. The loop continues until the selected route has `done: true` or `max_iterations` is reached

When `schedule` is set, the detached tmux session stays resident and waits for the next cron match instead of running immediately. Each cron tick starts a fresh workflow from the configured entrypoint.

## Installation

### Homebrew (macOS / Linux)

```bash
brew install takumiyoshikawa/tap/skill-loop
```

### Go install

```bash
go install github.com/takumiyoshikawa/skill-loop/cmd/skill-loop@latest
```

Requires at least one supported agent CLI to be available on your PATH:

- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude`)
- [Codex CLI](https://github.com/openai/codex) (`codex`)
- [OpenCode CLI](https://github.com/sst/opencode) (`opencode`)

And `tmux` must be installed (skill execution is tmux-backed).

## Quick start

**1. Install skill-loop** (see [Installation](#installation) below)

**2. Add the `skill-loop` skill to your coding agent:**

```bash
npx skills add takumiyoshikawa/skill-loop
```

This installs the `skill-loop` slash command into your coding agent (Claude Code, Codex, OpenCode, etc.), which helps you create and edit `skill-loop.yml` and starter skill files.

**3. Ask your agent to scaffold your workflow:**

Open your coding agent in the project and run:

```
/skill-loop
```

The agent will help you write a `skill-loop.yml` and the corresponding skill files under `.agents/skills/`.

**4. Run:**

```bash
skill-loop run
```

`skill-loop run` starts in background by default and prints a `run_id`.
Use `skill-loop run --attach` to start detached and immediately attach to its tmux session.

---

A typical `skill-loop.yml` looks like this:

```yaml
name: feature-review
default_entrypoint: 1-impl
max_iterations: 10
router:
  runtime: claude
  model: claude-sonnet-4-6

skills:
  1-impl:
    agent:
      runtime: claude
      model: claude-sonnet-4-6
      args:
        - "--dangerously-skip-permissions"
    next:
      - id: send-review
        skill: 2-review

  2-review:
    agent:
      runtime: codex
      model: gpt-5.4
      args:
        - "--full-auto"
    next:
      - id: approve
        criteria: "If the review says implementation quality is sufficient."
        done: true
      - id: ask-human
        criteria: "If the review needs a human decision before implementation can continue."
        blocked: true
        skill: 1-impl
      - id: rework
        criteria: "If review found issues that require more implementation work."
        skill: 1-impl
```

Skills are coding-agent slash commands under `.agents/skills`:

```
.agents/skills/
  1-impl/SKILL.md
  2-review/SKILL.md
```

For periodic execution, add `schedule` with standard 5-field cron syntax:

```yaml
name: daily-check
schedule: "0 9 * * *"
default_entrypoint: staleness-check
max_iterations: 10

skills:
  staleness-check:
    agent:
      runtime: claude
      model: claude-sonnet-4.6
      args:
        - "--dangerously-skip-permissions"
    next:
      - id: finish
        done: true
```

## Usage

```bash
skill-loop run [config.yml] [flags]
```

| Argument / Flag    | Description                                                         |
| ------------------ | ------------------------------------------------------------------- |
| `[config.yml]`     | Path to config file (default: `skill-loop.yml`)                     |
| `--prompt`         | Initial prompt passed to the first skill                            |
| `--max-iterations` | Override the config's `max_iterations` value                        |
| `--entrypoint`     | Start from a specific skill (overrides config `default_entrypoint`) |
| `--attach`         | Attach to the detached run session immediately                      |

### Scheduled runs

If `schedule` is present, `skill-loop run` starts a resident scheduler inside tmux. The process waits until the next cron match, runs the workflow once, then waits again.

```bash
skill-loop run skill-loop.yml
skill-loop sessions ls
```

Example:

```text
ID                    STATUS     DETAILS                    CONFIG          STARTED
20260307T090000Z-ab   scheduled  next: 2026-03-08 09:00:00 skill-loop.yml  2026-03-07T09:00:00Z
20260307T091000Z-cd   running    iter: 3/10                ci-watch.yml    2026-03-07T09:10:00Z
```

### Examples

```bash
# Use default config (skill-loop.yml)
skill-loop run

# Start detached and attach immediately
skill-loop run --attach

# Specify a config file
skill-loop run my-workflow.yml

# Pass an initial prompt
skill-loop run --prompt "Add a /logout endpoint"

# Limit iterations
skill-loop run --max-iterations 5

# Start from a specific skill (skip earlier skills)
skill-loop run --entrypoint 2-review
```

## Configuration

### Top-level fields

| Field                  | Type   | Required | Description                                                              |
| ---------------------- | ------ | -------- | ------------------------------------------------------------------------ |
| `name`                 | string | No       | Workflow name used for session storage under `~/.local/share/skill-loop/<name>/` |
| `schedule`             | string | No       | Optional cron schedule in standard 5-field crontab syntax for periodic execution |
| `router`               | object | Sometimes | Shared router agent settings. Required when any skill has multiple `next` routes. |
| `default_entrypoint`   | string | Yes      | Default skill name to start with (unless overridden via `--entrypoint`)  |
| `max_iterations`       | int    | No       | Maximum loop iterations (default: 100)                                   |
| `idle_timeout_seconds` | int    | No       | Idle timeout for each skill execution before auto-restart (default: 900) |
| `max_restarts`         | int    | No       | Max auto-restarts per skill execution on idle timeout (default: 2, set `0` to disable) |
| `skills`               | map    | Yes      | Skill definitions                                                        |

### Skill fields

| Field   | Type   | Required | Description                      |
| ------- | ------ | -------- | -------------------------------- |
| `agent` | object | No       | Agent settings for the skill     |
| `next`  | list   | Yes      | Routing rules evaluated in order |

Each skill runs freely and writes normal stdout. skill-loop sends that stdout to the shared router agent, then passes the previous skill's stdout plus the router's decision reason into the next skill as handoff context.

### Agent fields

| Field     | Type   | Required | Description                                                                              |
| --------- | ------ | -------- | ---------------------------------------------------------------------------------------- |
| `runtime` | string | No       | Agent CLI to execute (`claude`, `codex`, `opencode`). Defaults to `claude`               |
| `model`   | string | No       | Model to use for the selected agent (for example `claude-sonnet-4.6`)                     |
| `args`    | list   | No       | Additional CLI arguments passed to the agent (e.g. `["--dangerously-skip-permissions"]`) |

`args` lets you pass arbitrary flags to the underlying agent CLI. This is useful for skipping permission prompts in automated workflows:

```yaml
skills:
  1-impl:
    agent:
      runtime: claude
      model: claude-sonnet-4.6
      args:
        - "--dangerously-skip-permissions"
    next:
      - skill: 2-review
```

`router` uses the same `runtime` / `model` / `args` shape as a skill agent, so you can also pass router-specific CLI flags:

```yaml
router:
  runtime: codex
  model: gpt-5.4
  args:
    - "--full-auto"
```

### Route fields

| Field      | Type   | Required | Description                                                                                              |
| ---------- | ------ | -------- | -------------------------------------------------------------------------------------------------------- |
| `id`       | string | Yes      | Stable route identifier that the router agent returns                                                    |
| `criteria` | string | Usually  | Judgment criteria used by the router when choosing this route. Required when a skill has multiple routes |
| `skill`    | string | Conditional | Next skill to run. Required unless `done: true`                                                        |
| `done`     | bool   | Conditional | Ends the workflow when selected. Mutually exclusive with `skill` and `blocked`                         |
| `blocked`  | bool   | No       | Pause the workflow and mark the session `blocked` awaiting human input. Requires `skill`                |

When a skill has exactly one route, skill-loop skips the router and selects that route automatically.

### Human in the loop

Use `blocked: true` when the workflow should stop and wait for a person:

```yaml
skills:
  review:
    next:
      - id: approve
        criteria: "Ready to ship"
        done: true
      - id: ask-human
        criteria: "A human needs to choose the direction before continuing"
        blocked: true
        skill: implement
```

When that route is selected, the session moves to `blocked` and stores the next skill plus the handoff context. Resume it later with:

```bash
skill-loop sessions resume <session-id> --prompt "Use option 2 and keep the existing API shape."
```

The resume prompt is appended to the saved handoff before the workflow continues from the blocked route's `skill`.

## Sessions

Each detached run is recorded under:

```
~/.local/share/skill-loop/<name>/<random-name>/
  session.json
  stdout.log
  stderr.log
```

Session files are stored under `~/.local/share/skill-loop/<name>/<random-name>`.
Commands like `skill-loop sessions ls` still scope results to the current repository by matching the recorded repo root.

```bash
skill-loop sessions ls
skill-loop sessions show
skill-loop sessions inspect <session-id>
skill-loop sessions logs <session-id>
skill-loop sessions logs <session-id> --stderr
skill-loop sessions logs <session-id> --tail 200
skill-loop sessions attach <session-id>
skill-loop sessions stop <session-id>
skill-loop sessions resume <session-id> --prompt "Reviewed. Continue with option B."
skill-loop sessions prune
skill-loop sessions prune --dry-run
skill-loop sessions prune --all
```

`skill-loop run` also prints the session directory plus the captured `stdout.log` and `stderr.log` paths when a detached run starts.
Scheduled sessions appear in `skill-loop sessions ls` with `scheduled` status and a `next:` timestamp. When a scheduled workflow is actively executing, the session switches to `running` and reports `iter: current/max`.
If a route selects `blocked: true`, the run stops in `blocked` status until a human resumes it.
Use `skill-loop sessions show` to launch the embedded React dashboard for the current repository and manage sessions from your browser.

## Architecture

```
cmd/skill-loop/          CLI entrypoint (Cobra)
internal/
  config/                YAML config loading & validation
  executor/              Agent CLI invocation & output parsing
  orchestrator/          Loop control, routing, iteration management
  scheduler/             Cron-backed resident execution loop
  session/               tmux session lifecycle + session metadata/log storage
```

## License

MIT
