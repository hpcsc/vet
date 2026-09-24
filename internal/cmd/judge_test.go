//go:build unit

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/gittest"
	"github.com/stretchr/testify/require"
)

func noul(value float64) *float64 { return &value }

func choice(value string) *string { return &value }

func score(value int) *int { return &value }

func confidence(value float64) *float64 { return &value }

func TestJudge(t *testing.T) {
	ctx := context.Background()

	questionsFile := `version: 1
name: the rules
rules:
  - id: no-flag-field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
  - id: database-migration
    instructions: describes the change
    type: choice
    choices:
      no-db: nothing
      uses-db: reads or writes
      migrates: alters the schema
    violatesWhen: migrates
  - id: log-guideline
    instructions: rates the change
    type: score
    scores:
      - first
      - second
      - third
    scoreLimit: 2
`

	cleanAnswers := []backend.Answer{
		{Rule: "no-flag-field", Noul: noul(0.2)},
		{Rule: "database-migration", Choice: choice("uses-db"), Confidence: confidence(0.9)},
		{Rule: "log-guideline", Score: score(1)},
	}

	violatingAnswers := []backend.Answer{
		{Rule: "no-flag-field", Noul: noul(0.9)},
		{Rule: "database-migration", Choice: choice("migrates")},
		{Rule: "log-guideline", Score: score(2)},
	}

	setup := func(t *testing.T, answers []backend.Answer) (judge, *gittest.Repo) {
		t.Helper()
		repo := gittest.NewWithRemote(t)
		repo.Commit("change.txt", "one\n", "Change a file")
		questionsPath := filepath.Join(t.TempDir(), "questions.yaml")
		require.NoError(t, os.WriteFile(questionsPath, []byte(questionsFile), 0o644))
		return judge{
			repo:      git.New(repo.Dir),
			backend:   backend.NewFake().WithAnswers(answers...),
			out:       &bytes.Buffer{},
			questions: questionsPath,
			base:      "origin/main",
		}, repo
	}

	t.Run("clean change", func(t *testing.T) {
		t.Run("prints checks and a no-violation summary", func(t *testing.T) {
			j, _ := setup(t, cleanAnswers)

			err := j.run(ctx)

			require.NoError(t, err)
			text := j.out.(*bytes.Buffer).String()
			require.Contains(t, text, "the rules")
			require.Contains(t, text, "✓ [noul] no-flag-field: 20%")
			require.Contains(t, text, "The change violates no rule.")
		})

		t.Run("stays on exit code 0 with --exit-code", func(t *testing.T) {
			j, _ := setup(t, cleanAnswers)
			j.exit = true

			err := j.run(ctx)

			require.NoError(t, err)
		})
	})

	t.Run("violating change", func(t *testing.T) {
		t.Run("prints crosses, the violation count, and the violated rules", func(t *testing.T) {
			j, _ := setup(t, violatingAnswers)

			err := j.run(ctx)

			require.NoError(t, err)
			text := j.out.(*bytes.Buffer).String()
			require.Contains(t, text, "✗ [noul] no-flag-field: 90%")
			require.Contains(t, text, "The change violates 3 rules.")
			require.Contains(t, text, "  - no-flag-field in change.txt (the rules)")
			require.Contains(t, text, "  - database-migration in change.txt (the rules)")
			require.Contains(t, text, "  - log-guideline in change.txt (the rules)")
		})

		t.Run("exits 0 when --exit-code is off", func(t *testing.T) {
			j, _ := setup(t, violatingAnswers)

			err := j.run(ctx)

			require.NoError(t, err)
		})

		t.Run("exits 1 when --exit-code is on", func(t *testing.T) {
			j, _ := setup(t, violatingAnswers)
			j.exit = true

			err := j.run(ctx)

			require.Error(t, err)
			var code exitCode
			require.ErrorAs(t, err, &code)
			require.Equal(t, 1, int(code))
		})
	})

	t.Run("json", func(t *testing.T) {
		t.Run("prints the report as JSON with the base and the violations", func(t *testing.T) {
			j, _ := setup(t, violatingAnswers)
			j.json = true

			err := j.run(ctx)

			require.NoError(t, err)
			var report map[string]any
			require.NoError(t, json.Unmarshal(j.out.(*bytes.Buffer).Bytes(), &report))
			require.Equal(t, "origin/main", report["base"])
			require.Equal(t, float64(3), report["violations"])
		})
	})

	t.Run("base detection", func(t *testing.T) {
		t.Run("detects the base when none is given", func(t *testing.T) {
			j, _ := setup(t, cleanAnswers)
			j.base = ""
			j.json = true

			err := j.run(ctx)

			require.NoError(t, err)
			var report map[string]any
			require.NoError(t, json.Unmarshal(j.out.(*bytes.Buffer).Bytes(), &report))
			require.Equal(t, "origin/HEAD", report["base"])
		})
	})

	t.Run("tool failures", func(t *testing.T) {
		t.Run("a missing questions file is a failure", func(t *testing.T) {
			j, _ := setup(t, cleanAnswers)
			j.questions = "no-such-questions.yaml"

			err := j.run(ctx)

			require.Error(t, err)
		})

		t.Run("a backend error is a failure", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("change.txt", "one\n", "Change a file")
			questionsPath := filepath.Join(t.TempDir(), "questions.yaml")
			require.NoError(t, os.WriteFile(questionsPath, []byte(questionsFile), 0o644))
			j := judge{
				repo:      git.New(repo.Dir),
				backend:   backend.NewFake().WithError(errors.New("the backend is down")),
				out:       &bytes.Buffer{},
				questions: questionsPath,
				base:      "origin/main",
			}

			err := j.run(ctx)

			require.ErrorContains(t, err, "the backend is down")
		})

		t.Run("no changed files is a clean run", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			questionsPath := filepath.Join(t.TempDir(), "questions.yaml")
			require.NoError(t, os.WriteFile(questionsPath, []byte(questionsFile), 0o644))
			j := judge{
				repo:      git.New(repo.Dir),
				backend:   backend.NewFake(),
				out:       &bytes.Buffer{},
				questions: questionsPath,
				base:      "origin/main",
			}

			err := j.run(ctx)

			require.NoError(t, err)
			require.Contains(t, j.out.(*bytes.Buffer).String(), "The change violates no rule.")
		})
	})

	t.Run("the files the backend sees", func(t *testing.T) {
		t.Run("one file per changed file, with its path and diff", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("change.txt", "one\n", "Change a file")
			fake := backend.NewFake().WithAnswers(cleanAnswers...)
			questionsPath := filepath.Join(t.TempDir(), "questions.yaml")
			require.NoError(t, os.WriteFile(questionsPath, []byte(questionsFile), 0o644))
			j := judge{
				repo:      git.New(repo.Dir),
				backend:   fake,
				out:       &bytes.Buffer{},
				questions: questionsPath,
				base:      "origin/main",
			}

			err := j.run(ctx)

			require.NoError(t, err)
			files := fake.Files()
			require.Len(t, files, 1)
			require.Equal(t, "change.txt", files[0].Path)
			require.Contains(t, files[0].Diff, "@@")
		})
	})
}
