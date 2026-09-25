//go:build unit

package verdict

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, text string) questions.File {
	t.Helper()

	file, err := questions.Parse([]byte(text), "")
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
    description: the change adds a flag field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
  - id: database-migration
    description: how the change touches the database
    instructions: describes the change
    type: choice
    choices:
      no-db: no database
      uses-db: reads or writes the database
      migrates: alters the schema
    violatesWhen: migrates
  - id: log-guideline
    description: how well the change follows the logging guideline
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

		require.Len(t, report.Groups, 1)
		require.Len(t, report.Groups[0].Answers, 1)
		return report.Groups[0].Answers[0]
	}

	t.Run("noul", func(t *testing.T) {
		t.Run("a value below the limit violates nothing", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.2)}})

			require.Equal(t, Row{Rule: "no-flag-field", Description: "the change adds a flag field", Type: questions.Noul, Path: "internal/repo.go", Value: 0.2}, row(t, report))
			require.Equal(t, 0, report.Violations)
		})

		t.Run("a value at the limit violates the rule", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.5)}})

			require.Equal(t, Row{Rule: "no-flag-field", Description: "the change adds a flag field", Type: questions.Noul, Path: "internal/repo.go", Value: 0.5, Violates: true}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("a value above the limit violates the rule", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.8)}})

			require.Equal(t, Row{Rule: "no-flag-field", Description: "the change adds a flag field", Type: questions.Noul, Path: "internal/repo.go", Value: 0.8, Violates: true}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("the row omits any confidence", func(t *testing.T) {
			raw, err := json.Marshal(row(t, judge(t, []backend.Answer{{Rule: "no-flag-field", Noul: noul(0.2)}})))

			require.NoError(t, err)
			require.JSONEq(t, `{"rule":"no-flag-field","description":"the change adds a flag field","path":"internal/repo.go","value":0.2}`, string(raw))
		})
	})

	t.Run("choice", func(t *testing.T) {
		t.Run("the violatesWhen answer violates the rule and carries the choice label", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "database-migration", Choice: choice("migrates"), Confidence: confidence(0.92)}})

			require.Equal(t, Row{Rule: "database-migration", Description: "how the change touches the database", Type: questions.Choice, Path: "internal/repo.go", Value: "migrates", Label: "alters the schema", Violates: true, Confidence: confidence(0.92)}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("another answer violates nothing", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "database-migration", Choice: choice("uses-db"), Confidence: confidence(0.95)}})

			require.Equal(t, Row{Rule: "database-migration", Description: "how the change touches the database", Type: questions.Choice, Path: "internal/repo.go", Value: "uses-db", Label: "reads or writes the database", Confidence: confidence(0.95)}, row(t, report))
			require.Equal(t, 0, report.Violations)
		})

		t.Run("the row carries the probabilities the backend gave", func(t *testing.T) {
			probabilities := map[string]float64{"no-db": 0.0, "uses-db": 0.05, "migrates": 0.95}
			report := judge(t, []backend.Answer{{Rule: "database-migration", Choice: choice("migrates"), Probabilities: probabilities}})

			require.Equal(t, probabilities, row(t, report).Probabilities)
		})
	})

	t.Run("score", func(t *testing.T) {
		t.Run("a level at the limit violates the rule and carries the score label", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "log-guideline", Score: score(2)}})

			require.Equal(t, Row{Rule: "log-guideline", Description: "how well the change follows the logging guideline", Type: questions.Score, Path: "internal/repo.go", Value: 2, Label: "prohibited logging", Violates: true}, row(t, report))
			require.Equal(t, 1, report.Violations)
		})

		t.Run("a level below the limit violates nothing", func(t *testing.T) {
			report := judge(t, []backend.Answer{{Rule: "log-guideline", Score: score(1)}})

			require.Equal(t, Row{Rule: "log-guideline", Description: "how well the change follows the logging guideline", Type: questions.Score, Path: "internal/repo.go", Value: 1, Label: "logs to stdout"}, row(t, report))
			require.Equal(t, 0, report.Violations)
		})

		t.Run("prefers the backend legend over the file labels", func(t *testing.T) {
			legend := map[string]string{"0": "first", "1": "second", "2": "third"}
			report := judge(t, []backend.Answer{{Rule: "log-guideline", Score: score(2), Legend: legend}})

			require.Equal(t, "third", row(t, report).Label)
		})
	})

	groupedFile := questions.File{
		Version: 1,
		Rules: []questions.Rule{
			{ID: "no-flag-field", Source: "the flags", Instructions: "adds a flag field", Type: questions.Noul, NoulLimit: noul(0.5)},
			{ID: "database-migration", Source: "the db", Instructions: "describes the change", Type: questions.Choice, Choices: map[string]string{"migrates": "alters the schema"}, ViolatesWhen: "migrates"},
		},
	}

	t.Run("report", func(t *testing.T) {
		t.Run("groups the rows under the questions file that declared their rule", func(t *testing.T) {
			report, err := Judge("origin/main", groupedFile, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.9)},
				{Path: "b.go", Rule: "database-migration", Choice: choice("migrates")},
			})

			require.NoError(t, err)
			require.Equal(t, "origin/main", report.Base)
			require.Len(t, report.Groups, 2)
			require.Equal(t, "the flags", report.Groups[0].Name)
			require.Equal(t, "a.go", report.Groups[0].Answers[0].Path)
			require.Equal(t, "the db", report.Groups[1].Name)
			require.Equal(t, "b.go", report.Groups[1].Answers[0].Path)
		})

		t.Run("rules without a questions file group under an empty name", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.1)},
				{Path: "b.go", Rule: "database-migration", Choice: choice("no-db")},
			})

			require.NoError(t, err)
			require.Len(t, report.Groups, 1)
			require.Equal(t, "", report.Groups[0].Name)
			require.Len(t, report.Groups[0].Answers, 2)
		})

		t.Run("keeps the groups in the order their rules first answer", func(t *testing.T) {
			report, err := Judge("origin/main", groupedFile, []backend.Answer{
				{Path: "b.go", Rule: "database-migration", Choice: choice("no-db")},
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.1)},
			})

			require.NoError(t, err)
			require.Equal(t, []string{"the db", "the flags"}, []string{report.Groups[0].Name, report.Groups[1].Name})
		})

		t.Run("keeps only violating rows in a violations-only view", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.1)},
				{Path: "b.go", Rule: "database-migration", Choice: choice("migrates")},
			})
			require.NoError(t, err)

			filtered := report.ViolationsOnly()

			require.Len(t, filtered.Groups, 1)
			require.Equal(t, "database-migration", filtered.Groups[0].Answers[0].Rule)
			require.Equal(t, 1, filtered.Violations)
		})

		t.Run("counts every violation across the groups", func(t *testing.T) {
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
		t.Run("renders each file with a check or a cross per rule and its label", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.2)},
				{Path: "a.go", Rule: "database-migration", Choice: choice("migrates"), Confidence: confidence(0.9)},
				{Path: "b.go", Rule: "log-guideline", Score: score(2)},
			})
			require.NoError(t, err)
			text := report.TextWithPassing()

			require.Contains(t, text, "a.go\n  ✓ [noul] the change adds a flag field: 20%")
			require.Contains(t, text, "  ✗ [choice] how the change touches the database: migrates (alters the schema) (confidence 0.9)")
			require.Contains(t, text, "\nb.go\n")
			require.Contains(t, text, "  ✗ [score] how well the change follows the logging guideline: 2 (prohibited logging)")
			require.Contains(t, text, "The change violates 2 rules.")
		})

		t.Run("shows the backend legend label when the row carries one", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{
					Path: "a.go", Rule: "log-guideline", Score: score(2),
					Legend: map[string]string{"0": "first", "1": "second", "2": "third"},
				},
			})
			require.NoError(t, err)
			text := report.Text()

			require.Contains(t, text, "  ✗ [score] how well the change follows the logging guideline: 2 (third)")
			require.NotContains(t, text, "second")
		})

		t.Run("carries the backend probabilities in the JSON row", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{
					Path: "a.go", Rule: "log-guideline", Score: score(2),
					Probabilities: map[string]float64{"0": 0.05, "1": 0.3, "2": 0.65},
					Legend:        map[string]string{"0": "first", "1": "second", "2": "third"},
				},
			})
			require.NoError(t, err)
			row := row(t, report)
			raw, err := json.Marshal(row)
			require.NoError(t, err)
			require.JSONEq(t, `{"rule":"log-guideline","description":"how well the change follows the logging guideline","path":"a.go","value":2,"label":"third","violates":true,"probabilities":{"0":0.05,"1":0.3,"2":0.65},"legend":{"0":"first","1":"second","2":"third"}}`, string(raw))
		})

		t.Run("lists every violated rule with its file in the summary", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.2)},
				{Path: "a.go", Rule: "database-migration", Choice: choice("migrates")},
				{Path: "b.go", Rule: "log-guideline", Score: score(2)},
			})
			require.NoError(t, err)
			text := report.Text()

			require.Contains(t, text, "  - how the change touches the database in a.go")
			require.Contains(t, text, "  - how well the change follows the logging guideline in b.go")
		})

		t.Run("groups the rows under the changed file and questions file name", func(t *testing.T) {
			report, err := Judge("origin/main", groupedFile, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.9)},
				{Path: "b.go", Rule: "database-migration", Choice: choice("migrates")},
			})
			require.NoError(t, err)
			text := report.Text()

			require.Contains(t, text, "a.go\n  the flags\n    ✗ [noul] no-flag-field: 90%")
			require.Contains(t, text, "b.go\n  the db\n    ✗ [choice] database-migration: migrates")
			require.Contains(t, text, "  - database-migration in b.go (the db)")
			require.Contains(t, text, "  - no-flag-field in a.go (the flags)")
		})

		t.Run("keeps changed-file order when question groups arrive out of order", func(t *testing.T) {
			report, err := Judge("origin/main", groupedFile, []backend.Answer{
				{Path: "a.go", Rule: "database-migration", Choice: choice("migrates")},
				{Path: "c.go", Rule: "no-flag-field", Noul: noul(0.9)},
				{Path: "b.go", Rule: "database-migration", Choice: choice("migrates")},
			})
			require.NoError(t, err)
			text := report.TextWithPassing()

			firstFile := strings.Index(text, "a.go\n")
			secondFile := strings.Index(text, "c.go\n")
			thirdFile := strings.Index(text, "b.go\n")
			require.NotEqual(t, -1, firstFile)
			require.NotEqual(t, -1, secondFile)
			require.NotEqual(t, -1, thirdFile)
			require.Less(t, firstFile, secondFile)
			require.Less(t, secondFile, thirdFile)
			firstSummary := strings.Index(text, "- database-migration in a.go")
			secondSummary := strings.Index(text, "- no-flag-field in c.go")
			thirdSummary := strings.Index(text, "- database-migration in b.go")
			require.Less(t, firstSummary, secondSummary)
			require.Less(t, secondSummary, thirdSummary)
		})

		t.Run("renders the rule id when a rule has no description", func(t *testing.T) {
			report, err := Judge("origin/main", groupedFile, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.9)},
			})
			require.NoError(t, err)
			text := report.Text()

			require.Contains(t, text, "    ✗ [noul] no-flag-field: 90%")
			require.Contains(t, text, "  - no-flag-field in a.go (the flags)")
		})

		t.Run("hides passing rules unless they are requested", func(t *testing.T) {
			report, err := Judge("origin/main", file, []backend.Answer{
				{Path: "a.go", Rule: "no-flag-field", Noul: noul(0.1)},
			})
			require.NoError(t, err)

			text := report.Text()

			require.NotContains(t, text, "a.go")
			require.NotContains(t, text, "✓ [noul] the change adds a flag field: 10%")
			require.Contains(t, text, "The change violates no rule.")
			require.Contains(t, report.TextWithPassing(), "  ✓ [noul] the change adds a flag field: 10%")
		})
	})
}
