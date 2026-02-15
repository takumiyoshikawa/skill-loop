# skill-loop

An agentic skill orchestrator for coding-agent CLIs.

Chain multiple coding-agent skills together in a loop-based workflow. Define skills, routing conditions, and iteration limits in a simple YAML config — skill-loop handles the rest.

## How it works

```
          +--------+         +----------+
          | 1-impl | ------> | 2-review |
          +--------+         +----------+
              ^                 |     |
              |  (needs fix)    |     |  (REVIEW_OK)
              +-----------------+     |
                                      v
                                   <DONE>
```

1. skill-loop reads a YAML config that defines skills and routing rules
2. Starting from `--entrypoint` (if provided) or `default_entrypoint`, it invokes the configured agent (`claude`, `codex`, or `opencode`) with each skill
3. Each skill produces a summary; routing rules match substrings in the summary to decide the next skill
4. The loop continues until a route resolves to `<DONE>` or `max_iterations` is reached

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

Create a `skill-loop.yml` in your project:

```yaml
default_entrypoint: 1-impl
max_iterations: 10

skills:
  1-impl:
    agent:
      runtime: claude
      model: claude-sonnet-4-5-20250929
      args:
        - "--dangerously-skip-permissions"
    next:
      - skill: 2-review

  2-review:
    agent:
      runtime: codex
      model: gpt-5.3-codex
      args:
        - "--full-auto"
    next:
      - when: "<REVIEW_OK>"
        criteria: "If the review says implementation quality is sufficient."
        skill: "<DONE>"
      - skill: 1-impl
```

Define skills as Claude Code custom slash commands under `.claude/skills/`:

```
.claude/skills/
  1-impl/SKILL.md
  2-review/SKILL.md
```

Run:

```bash
skill-loop run
```

`skill-loop run` starts in background by default and prints a `run_id`.
Use `skill-loop run --attach` to start detached and immediately attach to its tmux session.

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
| `--attach`         | Attach to the detached run session immediately                       |

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

### Agent fields

| Field     | Type   | Required | Description                                                                              |
| --------- | ------ | -------- | ---------------------------------------------------------------------------------------- |
| `runtime` | string | No       | Agent CLI to execute (`claude`, `codex`, `opencode`). Defaults to `claude`               |
| `model`   | string | No       | Model to use for the selected agent (for example `claude-sonnet-4-5-20250929`)           |
| `args`    | list   | No       | Additional CLI arguments passed to the agent (e.g. `["--dangerously-skip-permissions"]`) |

`args` lets you pass arbitrary flags to the underlying agent CLI. This is useful for skipping permission prompts in automated workflows:

```yaml
skills:
  1-impl:
    agent:
      runtime: claude
      model: claude-sonnet-4-5-20250929
      args:
        - "--dangerously-skip-permissions"
    next:
      - skill: 2-review
```

### Route fields

| Field      | Type   | Required | Description                                                                                              |
| ---------- | ------ | -------- | -------------------------------------------------------------------------------------------------------- |
| `when`     | string | No       | Substring to match in the skill's summary. If omitted, the route always matches (acts as a default)      |
| `criteria` | string | No       | Judgment criteria describing when this route should be chosen (included in the agent prompt as guidance) |
| `skill`    | string | Yes      | Next skill to run, or `<DONE>` to terminate the loop                                                     |

Routes are evaluated top-to-bottom. The first matching route is selected. A route without `when` acts as a fallback.

## Sessions

Each detached run is recorded under:

```
<repo-root>/.skill-loop/sessions/<session-id>/
  session.json
  stdout.log
  stderr.log
```

Session root is resolved from `git rev-parse --show-toplevel` (fallback: current directory).

```bash
skill-loop sessions ls
skill-loop sessions attach <session-id>
skill-loop sessions stop <session-id>
```

## Architecture

```
cmd/skill-loop/          CLI entrypoint (Cobra)
internal/
  config/                YAML config loading & validation
  executor/              Agent CLI invocation & output parsing
  orchestrator/          Loop control, routing, iteration management
  session/               tmux session lifecycle + session metadata/log storage
```

## License

MIT
