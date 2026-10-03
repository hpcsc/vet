package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v3"
)

// newReplayCommand judges each commit against its own parent and saves the
// report, so a profile has something to read. It runs this same binary in a
// worktree per commit, so the tool measures the binary a user runs rather than
// a second path through the judge that only the tool exercises.
func newReplayCommand() *cli.Command {
	return &cli.Command{
		Name:  "replay",
		Usage: "judge each commit against its parent and save the reports",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "questions", Value: "", Usage: "the questions file or directory to judge with"},
		},
		Action: replayAction,
	}
}

func replayAction(ctx context.Context, cmd *cli.Command) error {
	questions := cmd.String("questions")
	if questions == "" {
		return fmt.Errorf("replay needs --questions")
	}
	args := cmd.Args().Slice()
	if len(args) < 3 {
		return fmt.Errorf("replay needs a repository, an output directory, and at least one commit")
	}
	repo, dir, shas := args[0], args[1], args[2:]

	bin, err := os.Executable()
	if err != nil {
		return err
	}
	for _, sha := range shas {
		if err := replayCommit(ctx, bin, questions, repo, dir, sha); err != nil {
			return err
		}
	}
	return nil
}

func replayCommit(ctx context.Context, bin, questions, repo, dir, sha string) error {
	started := time.Now()
	work := filepath.Join(dir, sha, "work")
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	if err := gitRun(ctx, repo, "worktree", "add", "--detach", "--force", work, sha); err != nil {
		return err
	}
	defer gitRun(ctx, repo, "worktree", "remove", "--force", work)

	// --all is what puts the passing answers in the report. Without it vet
	// saves only the violations, and profile would have no denominator: every
	// rule would look asked exactly as often as it reported.
	judge := exec.CommandContext(ctx, bin, "--base", sha+"~1", "--questions", questions, "--output", "json", "--all")
	judge.Dir = work
	raw, err := judge.Output()
	if err != nil {
		return fmt.Errorf("judge %s: %w: %s", shortSha(sha), err, raw)
	}
	out := filepath.Join(dir, sha, "1.json")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		return err
	}
	var report struct {
		Violations int
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	fmt.Printf("%s: %d violations in %s\n", shortSha(sha), report.Violations, time.Since(started).Round(time.Millisecond))
	return nil
}

func gitRun(ctx context.Context, repo string, args ...string) error {
	run := exec.CommandContext(ctx, "git", args...)
	run.Dir = repo
	if out, err := run.CombinedOutput(); err != nil {
		return fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return nil
}

func shortSha(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
