# output-mode-flag: select text, JSON, or interactive output

Replace the boolean `--json` flag with one output-mode selector while preserving the existing report model and JSON contract.

## Task 1 — CLI output mode selection

Replace `judge.json` with an output-mode value wired from a `--output`/`-o` string flag. The default is `text`; accepted values are `text`, `json`, and `tui`; unknown values fail before judging. Keep `--all` independent and preserve the existing JSON `groups` schema and text report behavior.

Acceptance criteria:

- [x] `vet` and `vet -o text` render the existing text report.
- [x] `vet --output json` renders the existing filtered JSON report.
- [x] `vet -o tui` selects the TUI mode; interactive behavior is covered by Task 2.
- [x] `vet --output unknown` fails with a clear invalid-output error.
- [x] The command no longer defines or accepts `--json`.
- [x] Unit tests cover the output mode through `judge.run` and its validation.

Patterns: `internal/cmd/root.go:36-96`, `internal/cmd/judge.go:24-115`, `internal/cmd/judge_test.go:168-229`, `internal/verdict/report.go:122-269`.

## Task 2 — interactive report renderer

Add a terminal renderer for the report in TUI mode. It should show a report overview, file/group/rule rows, and useful answer details; support selecting rows, toggling passing rows, and quitting. The renderer must restore terminal state on exit and report a clear error when standard input/output is not an interactive terminal. Keep the renderer isolated from backend judging and test its state transitions and output with injected streams.

Acceptance criteria:

- [x] TUI mode can render a report and return cleanly on quit.
- [x] Navigation changes the selected row.
- [x] The visibility toggle can include or omit passing rows without changing the total violation count.
- [x] Interrupted or invalid input returns a useful error and restores terminal state.
- [x] Non-interactive execution fails clearly instead of blocking or silently falling back.
- [x] Renderer tests do not require a real terminal.

Patterns: `internal/verdict/report.go:13-36`, `internal/verdict/report.go:122-269`, `internal/cmd/judge.go:104-115`, `go.mod`.

## Task 3 — e2e and documentation

Update the public CLI examples and proposal to describe `--output text|json|tui`, `-o`, and the independent `--all` flag. Replace e2e JSON invocations and add coverage for the new default/JSON forms and invalid values. Do not reintroduce the removed pseudo-terminal harness unless the TUI test requires it.

Acceptance criteria:

- [x] No user-facing documentation recommends `--json`.
- [x] README and proposal document the output selector and its default.
- [x] E2E JSON tests use `--output json` and retain the current `groups` assertions.
- [x] E2E tests verify the default text form and invalid output handling.
- [x] TypeScript tests remain type-safe without changing unrelated dependencies.

Patterns: `README.md:30-50`, `docs/proposal.md:33-45`, `e2e/tests/judge.test.ts:100-207`, `e2e/package.json`.
