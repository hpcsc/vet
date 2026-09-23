# implement-vet: judge a diff against a file of written rules

Build `vet` as `docs/proposal.md` specifies. The scaffold already provides `cmd/vet`,
`internal/cmd` (root, update, adapter), `internal/version`, `internal/release`,
`internal/progress`, goreleaser/Taskfile config and the e2e harness. The proposal's new
packages are `internal/git`, `internal/gittest`, `internal/diff`, `internal/questions`,
`internal/backend`, `internal/backend/jev`, `internal/verdict`. The root command becomes
the judge; `vet questions` and `vet version` become subcommands; the TUI helpers in the
e2e harness (`openCli`, `node-pty`, `tuistory`) are removed.

## Task 1 — questions package: model and validation

`internal/questions/` holds the questions-file model and its validation. `questions.yaml`
is version 1 with a list of rules; each rule is one of three types (`noul`, `choice`,
`score`) with type-specific fields. Validation rejects: a rule with no id or empty
instructions; `noulLimit` outside 0..1; `violatesWhen` naming a key absent from `choices`;
`scoreLimit` outside the `scores` range. Unit tests next to the code carry `//go:build unit`.

## Task 2 — git, gittest and diff: base, changed files, per-file diff

`internal/git/` runs the diff plumbing and detects the base ref. Base detection tries
`--base`, then `origin/HEAD`, `origin/main`, `origin/master`, `main`, `master`, using
`git rev-parse --verify --quiet`. Changed files come from `git diff --raw -z --no-abbrev -M base HEAD`; each file's unified diff from `git diff -M -U3 base HEAD -- path`.
`internal/gittest/` builds a scratch git repository for tests. `internal/diff/` exposes the
per-file diff concept. Unit tests in all three.

## Task 3 — backend package: the interface, State and Answer

`internal/backend/` defines the backend contract: `Backend.Ask(ctx, state, questions.File)
([]Answer, error)`. `State` carries the file path and the unified diff; `Answer` has the
rule id, a value (noul float, choice string or score int) and a confidence. The package
owns the request/answer contract.

## Task 4 — verdict package: judge answers and report

`internal/verdict/` checks each answer against its rule and renders a report. Violation
rules: noul answer >= `noulLimit`; choice answer equals `violatesWhen`; rounded score >=
`scoreLimit`. The report is JSON with base, per-file answers (omitting noul confidence)
and a violation count, plus a text form with check and cross marks and a summary line.

## Task 5 — jev backend: the System One client

`internal/backend/jev/` posts one request per changed file to the API URL with model and
API key, mapping each rule kind to a question, and maps responses back to answers. Retry
429/529 with exponential backoff (honouring `Retry-After`, at most 6 attempts); fail at
once on 401/422 with the server's reason. Unit tests against a fake server cover mapping
and both retry paths.

## Task 6 — cmd: flags, subcommands, pipeline, exit codes

`internal/cmd/` becomes the judge plus the `questions` and `version` subcommands. Flags:
`--base`, `--questions` (default `questions.yaml`), `--json`, `--exit-code`, `--api-url`
(default `https://api.typesafe.ai/v1/systemone` or `TYPESAFE_API_URL`), `--model`
(jev-latest), `--api-key` (`TYPESAFE_API_KEY`; empty key with no env is a clear failure).
Pipeline: resolve config, detect base, list changed files, build per-file diffs, load and
validate questions, ask the backend with a semaphore of 4, judge, print, exit code
(0 clean, 1 violation with `--exit-code` returned as `exitCode` not error, 2 tool failure).
`vet questions` prints the example file. The scaffold's TUI action and `adapter.go` go.

## Task 7 — e2e: fake System One, runCli-only tests, remove TUI

`e2e/` tests the real binary against a fake System One server in a scratch repo: an
unclean change, a violating change, JSON output, and the exit codes. `openCli`,
`node-pty` and `tuistory` are removed from the harness, package.json, Dockerfile and
docs — the tests use `runCli` only, and the image no longer installs `python3`, `make`,
`g++`.