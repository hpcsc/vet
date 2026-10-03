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
vet origin/main                    # compare against a specific base
vet --base origin/main             # the same, spelled with a flag
vet --questions my-rules.yaml      # use a different questions file, or a directory of them
vet --output json                  # print the report as JSON
vet -o tui                          # open the interactive report
vet --all                          # show passing and failing rules
vet --exit-code                    # exit 1 when the change violates a rule
vet questions example                # print an example questions file
vet questions init                   # write the default questions file where vet looks for it
vet config example                   # print the default config file
vet config init                      # write the default config file where vet looks for it
vet replay --questions my-rules.yaml . /tmp/runs <sha>...   # judge commits and save the reports
vet profile /tmp/runs                # report how near each rule's answers sit to its limit
```

`vet` runs the diff of a change against a file of written rules, answered by the System One model
`jev-latest`, and prints the failing rules by default. Use `--output`/`-o` to select `text` (the default),
`json`, or `tui`; TUI output requires an interactive terminal. Pass `--all` to show passing and failing
rules in any output mode. When `--questions` names a directory, the text report groups results by changed
file first, then by the questions file's `name`, or by the file name. JSON keeps its questions-file
groups. It exits 0 when the change violates no rule, 1 when a rule violates and `--exit-code` is on, and 2
when it cannot finish: no questions file, no API key, or a backend error.

A `noul` answer within 0.1 of its `noulLimit` prints as `unsure` rather than a percentage, and the JSON row
gains `"unsure": true`. An unsure answer counts as neither a pass nor a violation: its mark is `?` and it
does not gate the exit code. The number stays in the JSON `value` for a tool that reads it. A model
asked the same question twice answers `noul` somewhere near its limit often enough that a number that
close says less than the verdict does.

`vet` can cache each answer by the exact prompt that produced it, so the same change against the same
questions reports the same answers across runs instead of the model's run-to-run churn. Caching is off
unless you ask for it: pass `--cache-dir <dir>`, or set `VET_CACHE_DIR`, to store and reuse the answers
there.

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

`vet questions example` prints a complete, runnable starting point. A rule is only worth asking when its
answer is decidable from the file path, the diff, and the repository material the file asks for with
`include`. A rule whose deciding fact is not in the prompt makes the model guess at the threshold instead
of answer, so the example asks for facts a compiler cannot see but the prompt does carry: whether an
interface repeats its package, whether a change adds a doc comment, and how a test names and stubs its
dependencies. The rules below are an excerpt; edit them to match your repository:

```yaml
version: 1
name: Go change policy
context: |
  This repository keeps a comment only where the code cannot carry the fact,
  and tests observable behavior through the public interface.
include: [siblingFilePaths]
rules:
  - id: interface-repeats-package
    description: An interface repeats its package name.
    instructions: |
      Answer 1 only when the change adds an interface whose name repeats its
      package name. Answer 0 when every interface the change adds does not
      repeat its package name, and when the change adds no interface.
    type: noul
    noulLimit: 0.5
    requiresAddedLine: '^\s*type\s+\w+\s+interface\b'

  - id: package-doc-comment
    description: The change adds a package or file doc comment.
    instructions: |
      Answer 1 only when the change adds a package or file doc comment. Answer
      0 when the change adds no such comment.
    type: noul
    noulLimit: 0.5
    requiresAddedLine: '^\s*//'

  - id: test-file-name
    description: A test file is not named for the file it tests.
    instructions: |
      Answer 1 only when the change renames or adds a test file whose name is
      not the file it tests plus _test, or names a test support file for its
      role instead of the type it declares. Answer 0 when every test file in the
      change is named for the file it tests, and when the change leaves the name
      alone.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*_test.go"

  - id: test-double
    description: Which test double the change uses for a dependency.
    instructions: |
      Which test double does the change use for a dependency? Answer stub
      when the double returns a fixed answer, and no-double when the change
      stubs no dependency out.
    type: choice
    choices:
      real: The real implementation or an in-memory double with real behavior.
      stub: A stub that returns a fixed answer, for the happy path.
      broken: A broken double that always fails, for error paths.
      recording: A recording double that captures call details.
      mock: A mock that verifies call sequences, the last resort.
      no-double: The change stubs no dependency out.
    violatesWhen: mock
    files:
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

These are decisions that a compiler cannot make: a file can compile and still be named wrong, an
interface can compile and still repeat its package, and a doc comment can compile and still belong
somewhere else. `vet` applies the written policy to the actual diff, so a team can make its
expectations explicit and get a consistent check on every change.

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

### Scoping rules to what a change does

A rule can also ask for something the change has to contain. `requiresAddedLine` and
`requiresRemovedLine` hold a regular expression, and `vet` skips the rule unless the
change adds or removes at least one line that matches.

This is for a rule that has nothing to judge without a particular line. A rule about
comments that the change deleted cannot fire on a change that deletes no comment, and
asking the model about it anyway only buys a wrong answer:

```yaml
version: 1
name: Comment policy
rules:
  - id: comment-prevents-no-edit
    description: The change removes a comment that explained the code.
    instructions: |
      The change deletes a comment that carried information a reader needs.
    type: noul
    noulLimit: 0.5
    requiresRemovedLine: '^\s*//'
  - id: comment-not-one-sentence
    description: The change adds a comment of more than one sentence.
    instructions: |
      The change adds a comment that spans more than one sentence.
    type: noul
    noulLimit: 0.5
    requiresAddedLine: '^\s*//'
```

The check runs before the model is asked, so a skipped rule costs nothing and produces no
answer. Both patterns are checked when the questions file loads, and one that does not
compile is an error that names the rule.

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

### Adding repository material

A rule can be undecidable not because it is badly worded but because the fact it turns on is not in
the prompt. `test-file-name` asks whether `loader_test.go` is named for the file it tests, and the
model is never told that file exists, so it has to guess. An `include` names what to put in the
prompt alongside the changed file's own path and diff.

```yaml
version: 1
include: [siblingFilePaths, containingDirDeclarations]
rules:
  - id: test-file-name
    instructions: Is the test file named for the file it tests?
    type: noul
    noulLimit: 0.5
    requiresAddedLine: '^\s*(t\.Run\(|func Test)'
```

| `include` | What it adds |
| --- | --- |
| `siblingFilePaths` | The other files in the same directory |
| `containingDirDeclarations` | The declarations in the same directory, without their bodies |
| `containingDirContent` | The full text of the same directory, the changed file excepted |
| `repoDeclarations` | The declarations from every file in the repository |
| `fileContent` | The whole changed file, for a rule turning on something outside the changed lines |
| `previousFileContent` | The file as it was before the change |

Declarations are read with Go's parser, so they come from Go files. Anything else in the directory
is listed by path but contributes no declarations, and a file that does not parse is skipped rather
than failing the run.

`include` is a property of the questions file, not of a rule, because one call judges every rule
that applies to a file, so the prompt has to hold what all of them need. Passing a directory of
questions files gives each path the union of what all of them asked for, and the same material
appears once. It does not exclude test files by name: the only file it leaves out is the one being
judged.

`context` is a guideline the rule is measured against. `include` is the repository material the rule
needs to decide. A run reports what the material cost on standard error, since that is a fact about
the run rather than about the code being judged.

`docs/proposal.md` specifies the file format in full. The base is the bare argument when you give one,
else `--base`, else `origin/HEAD`, then `origin/main`, `origin/master`, `main`, `master`, and `HEAD~1`.
Seven words name a subcommand, so `vet` reads them as that subcommand rather than as a base: `config`,
`questions`, `version`, `update`, `replay`, `profile`, and `help`. A branch with one of those names needs
`--base`.

## Profiling a rule

A rule that answers near its `noulLimit` is guessing: the deciding fact is not in the prompt, and the
model returns a number near the threshold rather than a verdict. `vet replay` and `vet profile` measure
that per rule, so an author can see which rules to fix without keeping hand labels.

```shell
vet replay --questions my-rules.yaml . /tmp/runs $(git rev-list -n 9 HEAD)
vet profile /tmp/runs
vet profile /tmp/runs --max-near-limit 0.2   # exit non-zero when a rule guesses too often
```

`replay` judges each commit against its own parent and saves the report under
`<dir>/<sha>/1.json`, which is the JSON `vet --output json --all` already writes. `profile` reads those
reports and prints one row per rule, worst first:

```
rule                            type    asked  near limit  reports  median
comment-carried-by-code         noul    9      3 (33%)     1        0.28
test-file-name                  noul    15     2 (13%)     1        0.27
exposed-for-tests               noul    12     0 (0%)      0        0.04
struct-naming                   choice  3      -           1        0.60
```

The columns are the number of answers, the share of them that sat near the limit, the number that
reported, and the middle answer (or confidence for a `choice` rule). A rule whose answers cluster near
the limit is undecidable and reports noise at the rate of a coin flip; narrow it, give the model the
missing facts with `include`, or drop it.

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
