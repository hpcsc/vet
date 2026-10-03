//go:build unit

package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcsc/vet/internal/questions"
	"github.com/hpcsc/vet/internal/verdict"
	"github.com/stretchr/testify/require"
)

func noulRow(rule string, value float64) verdict.Row {
	unsure := value > 0.4 && value < 0.6
	return verdict.Row{
		Rule:     rule,
		Type:     questions.Noul,
		Value:    value,
		Violates: value >= 0.5 && !unsure,
		Unsure:   unsure,
	}
}

func choiceRow(rule, pick string, confidence float64) verdict.Row {
	return verdict.Row{
		Rule:       rule,
		Type:       questions.Choice,
		Value:      pick,
		Violates:   pick == "bad",
		Confidence: &confidence,
	}
}

func scoreRow(rule string, value int) verdict.Row {
	return verdict.Row{
		Rule:     rule,
		Type:     questions.Score,
		Value:    value,
		Violates: value >= 2,
	}
}

func reportOf(rows ...verdict.Row) verdict.Report {
	return verdict.Report{Groups: []verdict.Group{{Name: "g", Answers: rows}}}
}

func TestSummarize(t *testing.T) {
	t.Run("counting", func(t *testing.T) {
		t.Run("counts what one rule did across every report", func(t *testing.T) {
			reports := []verdict.Report{
				reportOf(noulRow("a", 0.9), noulRow("a", 0.1)),
				reportOf(noulRow("a", 0.55)),
			}
			stats := Summarize(reports)
			require.Len(t, stats, 1)
			require.Equal(t, 3, stats[0].Asked)
			require.Equal(t, 1, stats[0].Reports)
			require.Equal(t, 1, stats[0].NearLimit)
		})

		t.Run("keeps each rule on its own row", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(noulRow("a", 0.9), choiceRow("b", "good", 0.99))})
			require.Len(t, stats, 2)
			byRule := map[string]Stat{}
			for _, s := range stats {
				byRule[s.Rule] = s
			}
			require.Equal(t, questions.Noul, byRule["a"].Type)
			require.Equal(t, questions.Choice, byRule["b"].Type)
		})

		t.Run("leaves out a rule no report asked", func(t *testing.T) {
			require.Empty(t, Summarize([]verdict.Report{reportOf()}))
		})

		t.Run("trusts the report's own unsure flag", func(t *testing.T) {
			row := noulRow("a", 0.99)
			row.Unsure = true
			stats := Summarize([]verdict.Report{reportOf(row)})
			require.Equal(t, 1, stats[0].NearLimit)
		})
	})

	t.Run("share", func(t *testing.T) {
		t.Run("is the part of a noul rule's answers that sat near the limit", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(
				noulRow("a", 0.5), noulRow("a", 0.52), noulRow("a", 0.1), noulRow("a", 0.9),
			)})
			share, ok := stats[0].share()
			require.True(t, ok)
			require.InDelta(t, 0.5, share, 0.001)
		})

		t.Run("has none for a choice rule, which has no limit to crowd", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(choiceRow("a", "bad", 0.5))})
			_, ok := stats[0].share()
			require.False(t, ok)
		})

		t.Run("has none for a score rule", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(scoreRow("a", 2))})
			_, ok := stats[0].share()
			require.False(t, ok)
		})
	})

	t.Run("median", func(t *testing.T) {
		t.Run("is the middle noul value", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(
				noulRow("a", 0.9), noulRow("a", 0.1), noulRow("a", 0.5),
			)})
			median, ok := stats[0].median()
			require.True(t, ok)
			require.InDelta(t, 0.5, median, 0.001)
		})

		t.Run("averages the middle two of an even count", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(noulRow("a", 0.2), noulRow("a", 0.6))})
			median, ok := stats[0].median()
			require.True(t, ok)
			require.InDelta(t, 0.4, median, 0.001)
		})

		t.Run("is the confidence for a choice rule", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(
				choiceRow("a", "good", 0.99), choiceRow("a", "bad", 0.65),
			)})
			median, ok := stats[0].median()
			require.True(t, ok)
			require.InDelta(t, 0.82, median, 0.001)
		})

		t.Run("has none for a score rule, which has no single number", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(scoreRow("a", 1))})
			_, ok := stats[0].median()
			require.False(t, ok)
		})
	})

	t.Run("order", func(t *testing.T) {
		t.Run("puts the rule answering nearest the limit first", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(
				noulRow("calm", 0.05), noulRow("noisy", 0.5),
			)})
			require.Equal(t, "noisy", stats[0].Rule)
		})

		t.Run("keeps a noul rule above a choice rule that has no share", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(
				choiceRow("picked", "good", 0.99), noulRow("measured", 0.5),
			)})
			require.Equal(t, "measured", stats[0].Rule)
		})

		t.Run("falls back to the name when two rules share a share", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(noulRow("b", 0.5), noulRow("a", 0.5))})
			require.Equal(t, "a", stats[0].Rule)
		})
	})

	t.Run("write", func(t *testing.T) {
		t.Run("names the rule, its type, and how near the limit it answered", func(t *testing.T) {
			var out bytes.Buffer
			Write(&out, Summarize([]verdict.Report{reportOf(noulRow("a", 0.5), noulRow("a", 0.5))}))
			text := out.String()
			require.Contains(t, text, "rule")
			require.Contains(t, text, "near limit")
			require.Contains(t, text, "a")
			require.Contains(t, text, "noul")
			require.Contains(t, text, "2 (100%)")
		})

		t.Run("shows a dash where a rule has no share to report", func(t *testing.T) {
			var out bytes.Buffer
			Write(&out, Summarize([]verdict.Report{reportOf(choiceRow("a", "good", 0.99))}))
			require.Contains(t, out.String(), "-")
		})
	})

	t.Run("gate", func(t *testing.T) {
		t.Run("passes when every rule answers clear of the limit", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(noulRow("a", 0.05), noulRow("a", 0.95))})
			require.NoError(t, Gate(stats, 0.2))
		})

		t.Run("fails and names the rule that answers near the limit", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(noulRow("noisy", 0.5), noulRow("noisy", 0.52))})
			err := Gate(stats, 0.2)
			require.Error(t, err)
			require.Contains(t, err.Error(), "noisy")
		})

		t.Run("does not judge a rule that has no share", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(choiceRow("a", "good", 0.5))})
			require.NoError(t, Gate(stats, 0.2))
		})

		t.Run("does nothing when no share is asked for", func(t *testing.T) {
			stats := Summarize([]verdict.Report{reportOf(noulRow("noisy", 0.5))})
			require.NoError(t, Gate(stats, 0))
		})
	})
}

func TestReports(t *testing.T) {
	t.Run("reads reports under a commit directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "abc123"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "abc123", "1.json"), mustJSON(t, reportOf(noulRow("a", 0.1))), 0o644))
		reports, err := Reports(dir)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		require.Equal(t, "a", reports[0].Groups[0].Answers[0].Rule)
	})

	t.Run("ignores a file that is not a saved report", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "abc123"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "abc123", "notes.txt"), []byte("hello"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "loose.json"), []byte("{}"), 0o644))
		_, err := Reports(dir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "run vet replay")
	})

	t.Run("says where to look when the directory holds nothing", func(t *testing.T) {
		_, err := Reports(t.TempDir())
		require.Error(t, err)
		require.Contains(t, err.Error(), "run vet replay")
	})

	t.Run("fails on a report it cannot read", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "abc123"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "abc123", "1.json"), []byte("not json"), 0o644))
		_, err := Reports(dir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "1.json")
	})
}

func mustJSON(t *testing.T, report verdict.Report) []byte {
	t.Helper()
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	return raw
}
