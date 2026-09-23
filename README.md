# vet

## Build

```shell
task build            # build ./bin/vet
task run -- --help    # run the CLI from the source
```

## Judge a change

```shell
vet                                # judge the diff from the base to HEAD against questions.yaml
vet --base origin/main             # compare against a specific base
vet --questions my-rules.yaml      # use a different questions file, or a directory of them
vet --json                         # print the report as JSON
vet --exit-code                    # exit 1 when the change violates a rule
vet questions                      # print an example questions file
vet questions init                 # write the default questions file where vet looks for it
vet config                         # print the default config file
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

## Goreleaser

- Run goreleaser in local: `task release:local`. This will generate a snapshot build under `./dist`
- Create a release:

```shell
git tag vX.X.X
git push origin vX.X.X
```

This will trigger the release workflow, which runs the CI checks and then creates a Github Release with
binaries for MacOS and Linux.

Each push to `main` starts the prerelease workflow. It tags the commit with the next patch after the
latest release, the run number and the commit, for example `v0.2.1-42.g4829f92`, publishes that tag as a
prerelease, and then keeps only the 5 newest prereleases.

`On Demand Build` builds a snapshot of any ref from Github Actions and uploads the archives as artifacts.

## E2E Test

The end-to-end tests run the built binary against a fake System One server and throwaway git
repositories. They are in `e2e/`, and [docs/e2e-tests.md](docs/e2e-tests.md) tells how they work.

```shell
task test:e2e          # in Docker, as CI does
task test:e2e:local    # on this machine, needs node
```
