package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Repo struct {
	dir string
}

func New(dir string) *Repo {
	return &Repo{dir: dir}
}

// Toplevel returns the root directory of the working tree.
func (r *Repo) Toplevel(ctx context.Context) (string, error) {
	out, err := r.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// DetectBase returns the first ref that git accepts, from base first and then
// the usual defaults. base is empty when the caller gave no --base.
func (r *Repo) DetectBase(ctx context.Context, base string) (string, error) {
	refs := []string{
		base,
		"origin/HEAD",
		"origin/main",
		"origin/master",
		"main",
		"master",
	}
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		ok, err := r.ok(ctx, "rev-parse", "--verify", "--quiet", ref)
		if err != nil {
			return "", err
		}
		if ok {
			return ref, nil
		}
	}
	used := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref != "" {
			used = append(used, ref)
		}
	}
	return "", fmt.Errorf("no base ref resolves (tried %s)", strings.Join(used, ", "))
}

// ChangedFiles lists the paths the head changes against base, that is the
// destination path of a rename.
func (r *Repo) ChangedFiles(ctx context.Context, base string) ([]string, error) {
	out, err := r.run(ctx, "diff", "--raw", "-z", "--no-abbrev", "-M", base, "HEAD")
	if err != nil {
		return nil, err
	}
	return changedPaths(out), nil
}

func (r *Repo) UnifiedDiff(ctx context.Context, base, path string) (string, error) {
	return r.run(ctx, "diff", "-M", "-U3", base, "HEAD", "--", path)
}

func changedPaths(out string) []string {
	tokens := strings.Split(out, "\x00")
	var paths []string
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if !strings.HasPrefix(token, ":") {
			continue
		}
		fields := strings.Fields(token)
		if len(fields) < 5 {
			continue
		}
		status := fields[4][0]
		next := i + 1
		if status == 'R' || status == 'C' {
			next++
		}
		if next < len(tokens) {
			paths = append(paths, tokens[next])
		}
	}
	return paths
}

// ok runs git for a yes or no answer: true for exit 0, false for exit 1.
func (r *Repo) ok(ctx context.Context, args ...string) (bool, error) {
	err := r.runExit(ctx, args...)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func (r *Repo) run(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := r.command(ctx, args)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (r *Repo) runExit(ctx context.Context, args ...string) error {
	cmd := r.command(ctx, args)
	return cmd.Run()
}

func (r *Repo) command(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat")
	return cmd
}
