# vet

`vet` is a command-line tool that judges a change to code against a file of written rules / questions.
It reviews the diff, answers each question with a model, and reports whether the change violates any rule.

## Why not a golangci-lint plugin?

`golangci-lint` is designed for `go/analysis` analyzers that inspect loaded Go packages. `vet` is
repository-level: it compares a Git diff, reads YAML questions, and can judge any changed file,
including non-Go files. Its results are file-level and come from an external model rather than local
static analysis.

A plugin would need to run Git itself, turn each violation into a positioned `analysis.Diagnostic`,
and manage network calls, concurrency, retries, timeouts, and caching. It would also require a custom
`golangci-lint` build and couple Jev to the plugin lifecycle without improving the core workflow.
Keeping `vet` as a separate command preserves its repository scope and report format.

## Install

The installer downloads the latest release for your platform, verifies its checksum, and puts the
binary where you can start using it. It needs `sh`, `curl`, `jq`, `tar`, and `gzip`.

```shell
sh <(curl -fsSL https://raw.githubusercontent.com/hpcsc/vet/main/scripts/install.sh)
```

It asks for the release channel and the install directory. To skip the questions on the command line:

```shell
sh <(curl ...) --channel release --dir ~/.local/bin
```

The channel is `release` (the latest stable release) or `prerelease` (the latest build of `main`).
Without `--dir`, the binary lands in `~/.local/bin` (or `$VET_INSTALL_DIR`). When more than one
version is available in the channel, the installer lists them for you to pick.

`GITHUB_TOKEN` or `GH_TOKEN` gives access to a private repository.

`vet update` also installs from a release, and it replaces this very binary, so the installer and the
update command do the same kind of job; use whichever you find convenient on a fresh machine.

## Use

```shell
vet                                # judge the diff from the base to HEAD against questions.yaml
vet --base origin/main             # compare against a specific base
vet --questions my-rules.yaml      # use a different questions file, or a directory of them
vet --output json                  # print the report as JSON
vet -o tui                          # open the interactive report
vet --all                          # show passing and failing rules
vet --exit-code                    # exit 1 when the change violates a rule
vet questions example                # print an example questions file
vet questions init                   # write the default questions file where vet looks for it
vet config example                   # print the default config file
vet config init                      # write the default config file where vet looks for it
```

`vet` runs the diff of a change against a file of written rules, answered by the System One model
`jev-latest`, and prints the failing rules by default. Use `--output`/`-o` to select `text` (the default),
`json`, or `tui`; TUI output requires an interactive terminal. Pass `--all` to show passing and failing
rules in any output mode. When `--questions` names a directory, the text report groups results by changed
file first, then by the questions file's `name`, or by the file name. JSON keeps its questions-file
groups. It exits 0 when the change violates no rule, 1 when a rule violates and `--exit-code` is on, and 2
when it cannot finish: no questions file, no API key, or a backend error.

## API key

`vet` takes the System One API key from the first of these that is set:

1. `--api-key`
2. `TYPESAFE_API_KEY`
3. `apiKeyCommand` in the config file

An `apiKeyCommand` runs through the shell and uses its stdout as the key, so it can ask a keychain or a
credential store for the key. With `fnox`, for example:

```shell
vet config init   # write the config file first, then edit it
```

```yaml
# in ~/.config/vet/config.yaml
apiKeyCommand: fnox get TYPESAFE_API_KEY
```

When none of the three is set, `vet` fails and says how to provide a key.

## Questions file

`vet` judges the diff against a YAML file of rules. The file has a `version` and a list of `rules`; an
optional `name` labels it in the report; a shared `context` above the rules is prose the model sees before
every question. Each rule has an `id`, the `instructions` it answers, a `type`, and the fields that type needs.
An optional `description` labels the rule in the report, and the `id` stands in when it is missing:

| Type | Fields | Answer | Violation |
| --- | --- | --- | --- |
| `noul` | `noulLimit` | A probability from 0 to 1 | The answer is greater than or equal to `noulLimit` |
| `choice` | `choices`, `violatesWhen` | One of the `choices` keys | The answer equals `violatesWhen` |
| `score` | `scores`, `scoreLimit` | A level index | The answer is greater than or equal to `scoreLimit` |

`vet questions example` prints a complete, runnable starting point. The example asks policy questions,
not syntax questions: whether tests observe public behavior, whether a rejected operation proves state
stayed unchanged, whether a dependency needs a mock, and whether a change breaks an existing contract.
The rules below are an excerpt; edit them to match your repository:

```yaml
version: 1
name: Go service policy
context: |
  This repository treats public behavior and stored data as contracts.
  Tests should prove what callers observe, not how the code is arranged internally.
rules:
  - id: tests-through-public-api
    description: The change's tests assert on implementation details.
    instructions: |
      The change's tests assert private fields, internal call counts, call
      order, or intermediate state instead of the result a caller observes.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*_test.go"

  - id: rejected-operation-inert
    description: A rejected operation's test does not check the state stayed unchanged.
    instructions: |
      A test of a rejected operation checks the error but not that the
      observable state stayed unchanged where the public interface can show it.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*_test.go"

  - id: test-double
    description: Which test double the change uses for a dependency.
    instructions: Which test double does the change use for a dependency?
    type: choice
    choices:
      real: The real implementation or an in-memory double for the happy path.
      broken: A broken double that always fails, for error paths.
      recording: A recording double that captures call details.
      mock: A mock that verifies call sequences, the last resort.
    violatesWhen: mock
    files:
      - "**/*_test.go"

  - id: public-contract-change
    description: The change preserves its public contract.
    instructions: |
      Which option best describes the compatibility of this change for
      existing callers, stored data, and integrations?
    type: choice
    choices:
      compatible: No existing caller or stored record needs to change.
      additive: The change adds behavior without changing existing behavior.
      breaking: The change removes or changes behavior that existing callers or stored data rely on.
    violatesWhen: breaking
    files:
      - "**/*.go"
    exclude:
      - "**/*_test.go"

  - id: testing-quality
    description: How well the change's tests follow the testing policy.
    instructions: |
      Rate how well the change's tests prove observable success and failure
      behavior and remain independent of production implementation details.
    type: score
    scores:
      - Tests prove observable behavior and cover the relevant success and failure paths.
      - The tests follow the policy with one minor gap.
      - A test locks in implementation details or cannot fail on a real defect.
    scoreLimit: 2
    files:
      - "**/*_test.go"
```

These are decisions that a compiler cannot make: a test can compile and still assert the wrong thing, and
an apparently additive change can still break a stored contract. `vet` applies the written policy to the
actual diff, so a team can make its expectations explicit and get a consistent check on every change.

### Scoping rules to files

A rule can limit itself to some files and away from others. `files` holds globs of the
file paths the rule applies to, `exclude` removes the matched paths again, and `**`
crosses directories. A rule without `files` applies to every file, and an `exclude`
alone means every file but the matched ones.

The scope can sit once at the top of the questions file, above `rules`, instead of on
every rule. Each rule without its own `files` inherits the file's, a rule with its own
`files` replaces it, and the excludes of the file and the rule both apply:

```yaml
version: 1
name: Go change policy
files:
  - "**/*.go"
rules:
  - id: comment-adds-guidance
    description: A comment repeats the code instead of explaining a constraint.
    instructions: |
      The change adds a comment that repeats what the code says or explains no
      constraint a reader needs.
    type: noul
    noulLimit: 0.5
  - id: testing-quality
    description: How well the change's tests follow the testing policy.
    instructions: Rate how well the change's tests prove observable behavior.
    type: score
    scores: [proves behavior, follows with a gap, cannot fail on a defect]
    scoreLimit: 2
    files:
      - "**/*_test.go"
```

A changed file that no rule applies to is skipped: `vet` does not ask the model about it,
so a README-only change answers none of the Go rules.

### Referencing other files

A `context` or an `instructions` that starts with `@` names a file whose content is read instead, so a rule
can point at the guideline it measures instead of copying it. The path resolves against the directory of the
questions file, a leading `~` expands to the home directory, and a missing file is an error.

```yaml
version: 1
context: "@guidelines/testing.md"
rules:
  - id: testing-quality
    instructions: "@guidelines/testing.md"
    type: score
    scores: [proves behavior, follows with a gap, cannot fail on a defect]
    scoreLimit: 2
```

The reference works in both places: a `@` `context` and a `@` `instructions` each read their own file, and
so does every questions file when you pass a directory of them.

`docs/proposal.md` specifies the file format in full. The base is
`--base` when given, else `origin/HEAD`, then `origin/main`, `origin/master`, `main`, `master`, and `HEAD~1`.

## Version and update

```shell
vet version                # the tag of a release or a prerelease, or the commit of any other build
vet update                 # install the latest release
vet update --prerelease    # install the latest prerelease, a build of main
vet update --check         # only tell you whether this build is the latest
```

`internal/version` reports the version. A release build gets its tag from goreleaser, through
`-ldflags -X .../internal/version.releaseTag`. Any other build reports the short commit sha that Go
keeps in the build information, with `-dirty` after it when the working tree had changes.

`vet update` downloads the archive for your platform from a GitHub release, checks it
against the `checksums.txt` of that release, and then replaces the binary. It names each step on
stderr, and in a terminal it shows how much of the download has arrived. `GITHUB_TOKEN` or `GH_TOKEN`
gives access to a private repository.

Releases and prereleases are two channels. Each command installs the latest build of its channel when
this build is a different one, so `vet update` on a prerelease goes back to the latest
release. A build from a commit is not a release or a prerelease, so `vet update` does not
replace it unless you add `--force`.
