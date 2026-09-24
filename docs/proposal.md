# Proposal: vet, a CLI That Judges a Diff Against Written Rules

## Summary

`vet` is a command-line tool that judges the changes in a git branch against a file of written rules. It sends each changed file to an AI backend, gets structured answers, and reports whether the change violates the rules.

A maintainer writes the rules once. The tool applies them to every change, so the review does not depend on who reviews it, or on how tired they are.

## Problem

A code review checks a diff against guidelines that live in the reviewer's memory. This fails in three ways:

- The reviewer forgets a rule, or reads a long diff too fast to apply it.
- Two reviewers apply the same guideline differently.
- The guideline never becomes a written artifact, so it never gets reviewed or improved.

`vet` fixes all three. The rules live in a file in the repository. A model reads every changed file and answers the rules. The output is deterministic enough to gate a push or a merge.

## Design Decisions

### The command

| Command | Work |
| --- | --- |
| `vet` | Judge the diff against the questions file |
| `vet questions example` | Print an example questions file |
| `vet questions init` | Write the default questions file where vet looks for it |
| `vet config example` | Print the default config file |
| `vet config init` | Write the default config file where vet looks for it |
| `vet version` | Print the version |
| `vet update` | Replace `vet` with the latest release |

### The flags

| Flag | Default | Work |
| --- | --- | --- |
| `--base` | Detected | The git ref to compare against |
| `--questions` | The config, then `questions.yaml` in the working directory | The path of the questions file, or a directory of them |
| `--config` | `$XDG_CONFIG_HOME/vet/config.yaml` (or `~/.config/vet/config.yaml`) | The path of the config file |
| `--json` | Off | Print the report as JSON |
| `--exit-code` | Off | Exit 1 when the change violates a rule |
| `--api-key` | `TYPESAFE_API_KEY`, then the config `api-key-command` | The API key |
| `--api-url` | `TYPESAFE_API_URL`, then the config, then `https://api.typesafe.ai/v1/systemone` | The backend endpoint |
| `--model` | The config, then `jev-latest` | The model name |

The base detection tries these refs in order, and uses the first one that `git rev-parse --verify --quiet` accepts:

1. `--base`
2. `origin/HEAD`
3. `origin/main`
4. `origin/master`
5. `main`
6. `master`
7. `HEAD~1`

The tool reads the API key from `TYPESAFE_API_KEY` when the flag is empty. It fails with a clear message when no key exists.

### The questions file

The file is YAML. It has a version and a list of rules. The rules share an optional `context`, prose that the model sees above every question. A rule has one of three types. The example below uses all three.

```yaml
version: 1
context: |
  The change is in a Go codebase. General guidelines:
  - Log with slog, never to stdout.
rules:
  - id: no-flag-field
    description: The change adds a flag or knob that toggles behaviour.
    instructions: |
      The change adds a flag field to the request struct.
    type: noul
    noulLimit: 0.5

  - id: database-migration
    description: How the change touches the database.
    instructions: |
      Which option describes the change best?
    type: choice
    choices:
      no-db: The change does not touch the database.
      uses-db: The change reads or writes the database.
      migrates: The change alters the schema.
    violatesWhen: migrates

  - id: log-guideline
    description: How well the change follows the logging guideline.
    instructions: |
      Rate how well the change follows the logging guideline.
    type: score
    scores:
      - Logs with slog
      - Logs directly to stdout
      - Adds or keeps prohibited logging
    scoreLimit: 2
```

Each rule has an `id`, the `instructions` it answers, a `type`, and the fields that type needs. An optional `description` labels the rule in the report, and the `id` stands in when it is missing.

`context` and `instructions` also accept a file reference: when the value begins with `@`, the tool reads the file and uses its content instead. The path is relative to the questions file, and a leading `~` expands to the home directory. This lets a rule point at the guideline it measures instead of copying it, and lets one questions file reuse the same guideline files as the repository's other tooling.

```yaml
version: 1
context: "@~/.config/ai/guidelines/go/logging.md"
rules:
  - id: log-guideline
    instructions: "@~/.config/ai/guidelines/go/logging.md"
    type: score
    ...
```

The three types:

| Type | Field | Answer | Violation |
| --- | --- | --- | --- |
| `noul` | `noulLimit` | A probability from 0 to 1 | The answer is greater than or equal to `noulLimit` |
| `choice` | `choices`, `violatesWhen` | One of the `choices` keys | The answer equals `violatesWhen` |
| `score` | `scores`, `scoreLimit` | A level index | The answer is greater than or equal to `scoreLimit` |

The tool validates the file before it asks the backend. It rejects a file when:

- A rule has no id, or empty instructions.
- A `@` reference names a file the tool cannot read.
- `noulLimit` is outside 0 to 1.
- `violatesWhen` names a key that is not in `choices`.
- `scoreLimit` is outside the `scores` range.

### The backend

`internal/backend` defines an interface. A backend answers the rules for one file.

```go
type Backend interface {
    Ask(ctx context.Context, state State, questions questions.File) ([]Answer, error)
}
```

`State` carries the file path and the unified diff. `Answer` has the rule id, the value, and a confidence. The value is one of a `noul` float, a `choice` string, or a `score` int.

The first backend is Jev, TypeSafe's System One model. The Jev client posts one request per changed file on `POST /api/v1/systemone`:

```json
{
  "state": "File: internal/repo.go\n\n@@ ... @@",
  "model": "jev-latest",
  "questions": {
    "no-flag-field": {
      "type": "noul",
      "instructions": "The change adds a flag field to the request struct."
    },
    "database-migration": {
      "type": "choice",
      "instructions": "Which option describes the change best?",
      "criteria": {
        "no-db": "The change does not touch the database.",
        "uses-db": "The change reads or writes the database.",
        "migrates": "The change alters the schema."
      }
    }
  }
}
```

The response has one answer per question:

```json
{
  "answers": {
    "no-flag-field": { "type": "noul", "noul": 0.2 },
    "database-migration": {
      "type": "choice",
      "choice": "uses-db",
      "probabilities": { "no-db": 0.0, "uses-db": 0.95, "migrates": 0.05 },
      "confidence": 0.92
    }
  }
}
```

The Jev client maps a `noul` rule to a `noul` question, a `choice` rule to a `choice` question with `criteria`, and a `score` rule to a `score` question with an ordered `criteria` list.

Retry policy:

| Status | Work |
| --- | --- |
| 429, 529 | Retry with exponential backoff, and honor `Retry-After` when the server sends it. At most 6 attempts. |
| 401, 422 | Fail at once, with the server's reason. |

### The verdict

`internal/verdict` turns answers into a report grouped by questions file. Each rule becomes one row, and a row carries the file it judged.

```json
{
  "base": "origin/main",
  "groups": [
    {
      "name": "naming-patterns.yaml",
      "answers": [
        { "path": "internal/repo.go", "rule": "no-flag-field", "description": "The change adds a flag field to the request struct.", "value": 0.2 },
        {
          "path": "internal/repo.go",
          "rule": "database-migration",
          "description": "How the change touches the database.",
          "value": "migrates",
          "violates": true,
          "confidence": 0.92
        }
      ]
    }
  ],
  "violations": 1
}
```

A group holds the rows of one questions file. When you pass a directory of questions files, the report groups by file; each group is labeled by the questions file's `name`, or its file name. The verdict checks the violation rule for the question type:

- `noul`: the answer is greater than or equal to `noulLimit`.
- `choice`: the answer equals `violatesWhen`.
- `score`: the rounded answer is greater than or equal to `scoreLimit`.

A `noul` answer has no confidence in the Jev response, so its row omits it. The text report shows `check` and `cross` marks per rule with the rule `type` in brackets, and a summary that names every violated rule with the file and the questions file it comes from. A rule with a `description` renders it in place of the `id`; a rule without one renders the `id`. A `noul` row renders its value as a percentage.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | The tool ran, and the change violates no rule |
| 1 | The tool ran, a rule violated, and the caller passed `--exit-code` |
| 2 | The tool could not finish: no questions file, no API key, or a backend error |

The command returns an `exitCode` value instead of an error for the violation case, so `main` does not print a `vet:` message for it.

## How It Works

1. Resolve the API URL, the model, and the API key.
2. Detect the base ref.
3. List the changed files between the base and `HEAD`.
4. Build one unified diff per changed file.
5. Load and validate the questions file.
6. Ask the backend for every changed file. Files run in parallel, limited by a semaphore of about 4.
7. Judge the answers against the rules.
8. Print the report, and apply the exit code.

The diff commands:

```shell
git diff --raw -z --no-abbrev -M base HEAD
git diff -M -U3 base HEAD -- path
```

## Package Layout

```
cmd/vet/                 the entry point
internal/cmd/            the CLI, the flags, the subcommands
internal/git/            diff commands, base detection
internal/gittest/        a fake git repository for tests
internal/diff/           the per-file diff
internal/questions/      the questions file model and validation
internal/backend/        the Backend interface
internal/backend/jev/    the Jev client
internal/verdict/        the rule check and the report
internal/version/        the version of vet itself
internal/release/        the self-update
e2e/                     end-to-end tests
```

## Testing

Unit tests live next to the code and carry the `//go:build unit` tag. They cover:

- Questions file validation.
- The verdict for all three types, including each failure mode.
- The Jev client against a fake server, including the 429 and 529 retries.
- The base detection against a fake git repository.

End-to-end tests live in `e2e/`. They run the real binary in a scratch git repository, and answer it with a fake System One server. They cover the full flow: a change that passes, a change that violates, JSON output, and the exit codes.

The scaffold's TUI helpers are removed. `vet` is not interactive, so the tests use `runCli` only, and `e2e/package.json` drops `node-pty` and `tuistory`. The Docker image no longer needs `python3`, `make`, and `g++`.

## Future Backends

The `Backend` interface lets a second backend replace Jev without touching `cmd`, `git`, or `verdict`. The request and answer types are the only contracts the backend package owns.