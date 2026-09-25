//go:build unit

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

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

func runTUITest(input string, report verdict.Report, all bool, height int) (string, error) {
	var out bytes.Buffer
	err := runTUI(strings.NewReader(input), &out, report, all, func() int { return height })
	return out.String(), err
}

func tuiFrames(output string) []string {
	parts := strings.Split(output, "\x1b[2J\x1b[H")
	if len(parts) > 0 && parts[0] == "" {
		return parts[1:]
	}
	return parts
}

func TestTUI(t *testing.T) {
	t.Run("shows the selected answer after navigation", func(t *testing.T) {
		output, err := runTUITest("\x1b[Bq", tuiReport(), true, 0)

		require.NoError(t, err)
		frames := tuiFrames(output)
		require.Len(t, frames, 2)
		require.Contains(t, frames[0], "> PASS change.txt  rules  [noul] passing-rule")
		require.Contains(t, frames[0], "\nRule: passing-rule\n")
		require.Contains(t, frames[1], "> FAIL change.txt  rules  [noul] failing-rule")
		require.Contains(t, frames[1], "\nRule: failing-rule\n")
	})

	t.Run("shows passing answers after the toggle key without changing the violation count", func(t *testing.T) {
		output, err := runTUITest("aq", tuiReport(), false, 0)

		require.NoError(t, err)
		frames := tuiFrames(output)
		require.Len(t, frames, 2)
		require.Contains(t, frames[0], "violations: 1")
		require.NotContains(t, frames[0], "passing-rule")
		require.Contains(t, frames[1], "violations: 1")
		require.Contains(t, frames[1], "passing-rule")
		require.Contains(t, frames[1], "failing-rule")
	})

	t.Run("keeps the selected row inside the terminal height", func(t *testing.T) {
		output, err := runTUITest(strings.Repeat("j", 20)+"q", manyTUIReport(30), true, 12)

		require.NoError(t, err)
		frames := tuiFrames(output)
		require.Len(t, frames, 21)
		last := frames[len(frames)-1]
		require.LessOrEqual(t, strings.Count(last, "\n"), 12)
		require.Contains(t, last, "> FAIL change.txt  rules  [noul] rule-21")
		require.Contains(t, last, "\nRule: rule-21\n")
		require.NotContains(t, last, "rule-01")
	})

	t.Run("shows an empty report without a panic", func(t *testing.T) {
		output, err := runTUITest("\x1b[Bq", verdict.Report{Base: "main"}, false, 12)

		require.NoError(t, err)
		require.Contains(t, output, "No rules to display.")
	})

	t.Run("uses carriage returns for raw terminal output", func(t *testing.T) {
		var out bytes.Buffer
		input := []byte("one\ntwo\n")

		n, err := rawTerminalWriter{out: &out}.Write(input)

		require.NoError(t, err)
		require.Equal(t, len(input), n)
		require.Equal(t, "one\r\ntwo\r\n", out.String())
	})

	t.Run("rejects an unknown key", func(t *testing.T) {
		_, err := runTUITest("z", tuiReport(), false, 0)

		require.EqualError(t, err, `unknown TUI input "z"`)
	})

	t.Run("reports input that ends before quit", func(t *testing.T) {
		_, err := runTUITest("", tuiReport(), false, 0)

		require.EqualError(t, err, "TUI input ended before quit")
	})

	t.Run("rejects non-interactive output", func(t *testing.T) {
		var out bytes.Buffer

		err := renderTUI(&out, tuiReport(), false)

		require.EqualError(t, err, "tui output requires an interactive terminal")
	})
}
