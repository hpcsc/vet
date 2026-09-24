//go:build unit

package verdict

import (
	"encoding/json"
	"testing"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, text string) questions.File {
	t.Helper()

	file, err := questions.Parse([]byte(text))
	require.NoError(t, err)
	return file
}

func noul(value float64) *float64 { return &value }

func choice(value string) *string { return &value }

func score(value int) *int { return &value }

func confidence(value float64) *float64 { return &value }

func TestJudge(t *testing.T) {
	file := mustParse(t, `version: 1
rules:
  - id: no-flag-field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
  - id: database-migration
    instructions: describes the change
    type: choice
    choices:
      no-db: no database
      uses-db: reads or writes the database
      migrates: alters the schema
    violatesWhen: migrates
  - id: log-guideline
    instructions: rates the change
    type: score
    scores:
      - logs with slog
      - logs to stdout
      - prohibited logging
    scoreLimit: 2
`)

	judge := func(t *testing.T, answers []backend.Answer) Report {
		t.Helper()

		withPath := make([]backend.Answer, len(answers))
		copy(withPath, answers)
		for i := range withPath {
			withPath[i].Path = "internal/repo.go"
		}
		report, err := Judge("origin/main", file, withPath)
		require.NoError(t, err)
		return report
	}

	row := func(t *testing.T, report Report) Row {
		t.Helper()

		require.Len(t, report.Files, 1)
		require.Len(t, report.Files[0].Answers, 1)
		return report.Files[0].Answers[0]
	}

	t.Run("noul", func(t *testing.T) {
		t.Run("a value below the limit violates nothing", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.2)}})

			require.Equal(t, Row{Rule: "no-flag-field", Value: 0.2}, row(t, report))
			require.Equal(t, 0, report.Violations)
		})

		t.Run("a value at the limit violates the rule", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.5)}})

			require.Equal(t, Row{Rule: "no-flag-field", Value: 0.5, Violates: true}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("a value above the limit violates the rule", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.8)}})

			require.Equal(t, Row{Rule: "no-flag-field", Value: 0.8, Violates: true}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("the row omits any confidence", func(t *testing.T) {
			raw, err := json.Marshal(row(t, judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.2)}})))

			require.NoError(t, err)
			require.JSONEq(t, `{"rule":"no-flag-field","value":0.2}`, string(raw))
		})
	})

	t.Run("choice", func(t *testing.T) {
		t.Run("the violatesWhen answer violates the rule", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "database-migration", Choice: choice("migrates"), Confidence: confidence(0.92)}})

			require.Equal(t, Row{Rule: "database-migration", Value: "migrates", Violates: true, Confidence: confidence(0.92)}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("another answer violates nothing", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "database-migration", Choice: choice("uses-db"), Confidence: confidence(0.95)}})

			require.Equal(t, Row{Rule: "database-migration", Value: "uses-db", Confidence: confidence(0.95)}, row(t, report))
			require.Equal(t, 0, report.Violations)
		})
	})

	t.Run("score", func(t *testing.T) {
		t.Run("a level at the limit violates the rule", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "log-guideline", Score: score(2)}})

			require.Equal(t, Row{Rule: "log-guideline", Value: 2, Violates: true}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("a level below the limit violates nothing", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "log-guideline", Score: score(1)}})

			require.Equal(t, Row{Rule: "log-guideline", Value: 1}, row(t, report))
			require.Equal(t, 0, report.Violations)
		})
	})

	t.Run("report", func(t *testing.T) {
		t.Run("groups the rows under each file", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.1)},
				{Path: "b.go", Rule: "database-migration", Choice: choice("no-db")},
			})

			require.NoError(t, err)
			require.Equal(t, "origin/main", report.Base)
			require.Len(t, report.Files, 2)
			require.Equal(t, "a.go", report.Files[0].Path)
			require.Equal(t, "b.go", report.Files[1].Path)
		})

		t.Run("keeps the files in the order their answers arrive", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "b.go", Rule: "database-migration", Choice: choice("no-db")},
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.1)},
			})

			require.NoError(t, err)
			require.Equal(t, []string{"b.go", "a.go"}, []string{report.Files[0].Path, report.Files[1].Path})
		})

		t.Run("counts every violation across the files", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.9)},
				{Path: "b.go", Rule: "database-migration", Choice: choice("migrates")},
				{Path: "b.go", Rule: "log-guideline", Score: score(2)},
			})

			require.NoError(t, err)
			require.Equal(t, 3, report.Violations)
		})

		t.Run("rejects an answer for an unknown rule", func(t *testing.T) {
			_, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-such-rule", Noul: noul(0.1)},
			})

			require.ErrorContains(t, err, "no-such-rule")
		})
	})

	t.Run("text", func(t *testing.T) {
		t.Run("renders each file with a check or a cross per rule", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.2)},
				{Path: "a.go", Rule: "database-migration", Choice: choice("migrates"), Confidence: confidence(0.9)},
				{Path: "b.go", Rule: "log-guideline", Score: score(2)},
			})

			require.NoError(t, err)
			text := report.Text()

			require.Contains(t, text, "a.go")
			require.Contains(t, text, "✓ no-flag-field: 0.2")
			require.Contains(t, text, "✗ database-migration: migrates (confidence 0.9)")
			require.Contains(t, text, "b.go")
			require.Contains(t, text, "✗ log-guideline: 2")
			require.Contains(t, text, "The change violates 2 rules.")
		})

		t.Run("says so when the change violates no rule", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.1)},
			})

			require.NoError(t, err)
			text := report.Text()

			require.Contains(t, text, "✓ no-flag-field: 0.1")
			require.Contains(t, text, "The change violates no rule.")
		})
	})
}