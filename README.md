# vet

`vet` is a command-line tool that judges a change to code against a file of written rules / questions.
It reviews the diff, answers each question with a model, and reports whether the change violates any rule.

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
vet --json                         # print the report as JSON
vet --exit-code                    # exit 1 when the change violates a rule
vet questions example                # print an example questions file
vet questions init                   # write the default questions file where vet looks for it
vet config example                   # print the default config file
vet config init                      # write the default config file where vet looks for it
```

`vet` runs the diff of a change against a file of written rules, answered by the System One model
`jev-latest`, and prints a report with a check or a cross per rule. When `--questions` names a directory,
every questions file in it is asked, and the report groups the results under each file by its `name`, or
by the file name. It exits 0 when the change violates
no rule, 1 when a rule violates and `--exit-code` is on, and 2 when it cannot finish: no questions file,
no API key, or a backend error.

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

```yaml
version: 1
context: |
  The change is in a Go codebase. General guidelines:
  - Log with slog, never to stdout.
rules:
  - id: no-flag-field
    description: The change adds a flag or knob that toggles behaviour.
    instructions: The change adds a flag field to the request struct.
    type: noul
    noulLimit: 0.5
```

### Scoping a rule to files

A rule can limit itself to some files and away from others. `files` holds globs of the
file paths the rule applies to, `exclude` removes the matched paths again, and `**`
crosses directories. A rule without `files` applies to every file, and an `exclude`
alone means every file but the matched ones:

```yaml
version: 1
rules:
  - id: go-naming
    description: The change names the identifiers well.
    instructions: Rate how well the change names the identifiers.
    type: score
    scores: [well, poorly]
    scoreLimit: 1
    files:
      - "**/*.go"
    exclude:
      - "**/*_test.go"
```

A changed file that no rule applies to is skipped: `vet` does not ask the model about it,
so a README-only change answers none of the Go naming rules.

### Referencing other files

A `context` or an `instructions` that starts with `@` names a file whose content is read instead, so a rule
can point at the guideline it measures instead of copying it. The path resolves against the directory of the
questions file, a leading `~` expands to the home directory, and a missing file is an error.

```yaml
version: 1
context: "@~/.config/ai/guidelines/go/logging.md"
rules:
  - id: log-guideline
    instructions: "@~/.config/ai/guidelines/go/logging.md"
    type: score
    scores: [follows, adds noise, prohibited]
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
