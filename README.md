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
vet config                           # print the default config file
```

`vet` runs the diff of a change against a file of written rules, answered by the System One model
`jev-latest`, and prints a report with a check or a cross per rule. It exits 0 when the change violates
no rule, 1 when a rule violates and `--exit-code` is on, and 2 when it cannot finish: no questions file,
no API key, or a backend error.

```shell
TYPESAFE_API_KEY=... vet  # the API key, from the TYPESAFE_API_KEY env var or --api-key
```

`docs/proposal.md` specifies the questions file format and how the answers are judged. The base is
`--base` when given, else `origin/HEAD`, then `origin/main`, `origin/master`, `main`, and `master`.

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
