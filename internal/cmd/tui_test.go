//go:build unit

package cmd

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/hpcsc/vet/internal/verdict"
	"github.com/stretchr/testify/require"
)

func tuiReport() verdict.Report {
	return verdict.Report{
		Base: "main",
		Groups: []verdict.Group{{
			Name: "rules",
			Answers: []verdict.Row{
				{Rule: "passing-rule", Path: "change.txt", Value: 0.1, Type: questions.Noul},
				{Rule: "failing-rule", Path: "change.txt", Value: 0.9, Type: questions.Noul, Violates: true},
			},
		}},
		Violations: 1,
	}
}

func wideTUIReport() verdict.Report {
	return verdict.Report{
		Base: "a-very-long-base-branch-name",
		Groups: []verdict.Group{{
			Name: "a-very-long-group-name",
			Answers: []verdict.Row{{
				Rule:        "a-very-long-rule-name-that-never-ends",
				Description: "a very long description that also never seems to stop",
				Path:        "internal/some/deeply/nested/directory/change.txt",
				Value:       "insecure",
				Label:       "plain HTTP",
				Type:        questions.Choice,
			}},
		}},
	}
}

func manyTUIReport(count int) verdict.Report {
	rows := make([]verdict.Row, 0, count)
	for i := 1; i <= count; i++ {
		rows = append(rows, verdict.Row{
			Rule:     fmt.Sprintf("rule-%02d", i),
			Path:     "change.txt",
			Value:    0.9,
			Type:     questions.Noul,
			Violates: true,
		})
	}
	return verdict.Report{
		Base: "main",
		Groups: []verdict.Group{{
			Name:    "rules",
			Answers: rows,
		}},
		Violations: count,
	}
}

func tuiModelAt(t *testing.T, report verdict.Report, all bool, width, height int) tuiModel {
	t.Helper()
	return newTUIModel(report, all, tuiSize{width: width, height: height})
}

func tuiKeyMsg(name string) tea.KeyPressMsg {
	switch name {
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	default:
		return tea.KeyPressMsg(tea.Key{Code: rune(name[0]), Text: name})
	}
}

func tuiPress(t *testing.T, model tuiModel, keys ...string) tuiModel {
	t.Helper()
	for _, key := range keys {
		next, _ := model.Update(tuiKeyMsg(key))
		pressed, ok := next.(tuiModel)
		require.True(t, ok, "Update must keep the model type")
		model = pressed
	}
	return model
}

func tuiRun(t *testing.T, input string, report verdict.Report, all bool, size tuiSize) string {
	t.Helper()
	var out bytes.Buffer
	err := runTUI(strings.NewReader(input), &out, report, all, size)
	require.NoError(t, err)
	return out.String()
}

func tuiFrame(t *testing.T, model tuiModel) string {
	t.Helper()
	return model.View().Content
}

func TestTUI(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		t.Run("draws the report on the alternate screen and returns on quit", func(t *testing.T) {
			output := tuiRun(t, "q", tuiReport(), true, tuiSize{width: 80, height: 24})

			require.Contains(t, output, "\x1b[?1049h")
			require.Contains(t, output, "vet  ·  base main")
			require.Contains(t, output, "1 violation  ·  all rules")
			require.Contains(t, output, "  change.txt")
			require.Contains(t, output, "  ▸ ✓  noul    passing-rule  10%")
		})

		t.Run("rejects non-interactive output", func(t *testing.T) {
			var out bytes.Buffer

			err := renderTUI(&out, tuiReport(), false)

			require.EqualError(t, err, "tui output requires an interactive terminal")
		})
	})

	t.Run("move", func(t *testing.T) {
		t.Run("shows the next answer on down and on j", func(t *testing.T) {
			model := tuiModelAt(t, tuiReport(), true, 80, 24)
			model = tuiPress(t, model, "down")

			require.Equal(t, 1, model.selected)
			view := tuiFrame(t, model)
			require.Contains(t, view, "  ▸ ✗  noul    failing-rule  90%")
			require.Contains(t, view, "\n  failing-rule\n")
		})

		t.Run("wraps from the first answer to the last on up", func(t *testing.T) {
			model := tuiModelAt(t, tuiReport(), true, 80, 24)
			model = tuiPress(t, model, "up")

			require.Equal(t, 1, model.selected)
		})

		t.Run("keeps the selected row and the hints on a short terminal", func(t *testing.T) {
			model := tuiModelAt(t, manyTUIReport(30), true, 80, 12)
			model = tuiPress(t, model, slices.Repeat([]string{"j"}, 20)...)

			view := tuiFrame(t, model)
			require.LessOrEqual(t, strings.Count(view, "\n"), 12)
			require.Contains(t, view, "  ▸ ✗  noul    rule-21  90%")
			require.Contains(t, view, "\n  rule-21\n")
			require.Contains(t, view, "q quit")
			require.NotContains(t, view, "rule-01")
		})
	})

	t.Run("toggle", func(t *testing.T) {
		t.Run("shows passing answers after the toggle key without changing the violation count", func(t *testing.T) {
			hidden := tuiModelAt(t, tuiReport(), false, 80, 24)
			shown := tuiPress(t, hidden, "a")

			require.NotContains(t, tuiFrame(t, hidden), "passing-rule")
			require.Contains(t, tuiFrame(t, hidden), "1 violation  ·  violations only")
			require.Contains(t, tuiFrame(t, shown), "1 violation  ·  all rules")
			require.Contains(t, tuiFrame(t, shown), "passing-rule")
			require.Contains(t, tuiFrame(t, shown), "failing-rule")
		})

		t.Run("pulls the selection back when hiding rows drops the selected one", func(t *testing.T) {
			model := tuiModelAt(t, tuiReport(), true, 80, 24)
			model = tuiPress(t, model, "down", "a")

			require.False(t, model.showAll)
			require.Equal(t, 0, model.selected)
		})
	})

	t.Run("view", func(t *testing.T) {
		t.Run("takes the terminal size from a resize", func(t *testing.T) {
			model := tuiModelAt(t, tuiReport(), true, 80, 24)
			resized, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

			require.Equal(t, tuiSize{width: 100, height: 40}, resized.(tuiModel).size)
		})

		t.Run("says so when the report has no rules", func(t *testing.T) {
			model := tuiModelAt(t, verdict.Report{Base: "main"}, false, 80, 24)

			require.Contains(t, tuiFrame(t, model), "No rules to display.")
		})

		t.Run("keeps every line inside the terminal width", func(t *testing.T) {
			for _, size := range []tuiSize{{width: 80, height: 24}, {width: 60, height: 14}, {width: 40, height: 10}, {width: 24, height: 8}} {
				for _, report := range []verdict.Report{tuiReport(), wideTUIReport()} {
					for _, line := range strings.Split(tuiFrame(t, tuiModelAt(t, report, true, size.width, size.height)), "\n") {
						require.LessOrEqual(t, lipgloss.Width(line), size.width,
							"line wider than the terminal at %dx%d: %q", size.width, size.height, line)
					}
				}
			}
		})

		t.Run("drops detail fields to keep the hints on a short terminal", func(t *testing.T) {
			model := tuiModelAt(t, tuiReport(), true, 80, 10)

			view := tuiFrame(t, model)
			require.LessOrEqual(t, strings.Count(view, "\n"), 10)
			require.Contains(t, view, "\n  passing-rule\n")
			require.Contains(t, view, "    file          change.txt")
			require.NotContains(t, view, "group")
			require.Contains(t, view, "q quit")
		})
	})
}
