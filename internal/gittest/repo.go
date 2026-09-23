package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type Repo struct {
	t      testing.TB
	Dir    string
	origin string
}

// NewWithRemote makes a clone of a bare origin on main, with one commit on the
// origin and its HEAD set so that origin/HEAD resolves.
func NewWithRemote(t testing.TB) *Repo {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	dir := filepath.Join(root, "repo")
	run(t, root, "init", "-q", "--bare", "-b", "main", origin)
	run(t, root, "clone", "-q", origin, dir)
	r := &Repo{t: t, Dir: dir, origin: origin}
	r.configure()
	r.Commit("README.md", "readme\n", "Start the repository")
	r.Commit("code.go", "package main\n", "Add code.go")
	r.Git("push", "-q", "origin", "main")
	r.Git("remote", "set-head", "origin", "-a")
	return r
}

// NewLocal makes a repository with one commit on main and no remote.
func NewLocal(t testing.TB) *Repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	run(t, filepath.Dir(dir), "init", "-q", "-b", "main", dir)
	r := &Repo{t: t, Dir: dir}
	r.configure()
	r.Commit("README.md", "readme\n", "Start the repository")
	r.Commit("code.go", "package main\n", "Add code.go")
	return r
}

// Empty makes a repository with no commits and no remote.
func Empty(t testing.TB) *Repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	run(t, filepath.Dir(dir), "init", "-q", "-b", "main", dir)
	r := &Repo{t: t, Dir: dir}
	r.configure()
	return r
}

func (r *Repo) configure() {
	r.Git("config", "user.name", "vet")
	r.Git("config", "user.email", "vet@example.com")
	r.Git("config", "commit.gpgsign", "false")
}

func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return run(r.t, r.Dir, args...)
}

func (r *Repo) Commit(path, content, message string) {
	r.t.Helper()
	r.Write(path, content)
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", message)
}

func (r *Repo) Write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, path)
	require.NoError(r.t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(r.t, os.WriteFile(full, []byte(content), 0o644))
}

func run(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := command(dir, args...).CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
	return string(out)
}

func command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=vet", "GIT_AUTHOR_EMAIL=vet@example.com",
		"GIT_COMMITTER_NAME=vet", "GIT_COMMITTER_EMAIL=vet@example.com",
		"GIT_CONFIG_COUNT=3",
		"GIT_CONFIG_KEY_0=commit.gpgsign", "GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=core.hooksPath", "GIT_CONFIG_VALUE_1=/dev/null",
		"GIT_CONFIG_KEY_2=init.defaultBranch", "GIT_CONFIG_VALUE_2=main",
	)
	return cmd
}
