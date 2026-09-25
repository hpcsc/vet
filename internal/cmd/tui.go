package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/hpcsc/vet/internal/verdict"
	"golang.org/x/term"
)

type tuiModel struct {
	report   verdict.Report
	showAll  bool
	selected int
	size     tuiSize
}

type tuiSize struct {
	width  int
	height int
}

type tuiRow struct {
	path   string
	group  string
	answer verdict.Row
}

func newTUIModel(report verdict.Report, all bool, size tuiSize) tuiModel {
	return tuiModel{report: report, showAll: all, size: size}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.size = tuiSize{width: msg.Width, height: msg.Height}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "a":
			m.toggle()
		}
	}
	return m, nil
}

func (m *tuiModel) move(delta int) {
	rows := m.rows()
	if len(rows) == 0 {
		m.selected = 0
		return
	}
	m.selected = (m.selected + delta) % len(rows)
	if m.selected < 0 {
		m.selected += len(rows)
	}
}

func (m *tuiModel) toggle() {
	m.showAll = !m.showAll
	rows := m.rows()
	if len(rows) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(rows) {
		m.selected = len(rows) - 1
	}
}

func (m *tuiModel) rows() []tuiRow {
	report := m.visibleReport()
	rows := make([]tuiRow, 0)
	for _, group := range report.Groups {
		for _, answer := range group.Answers {
			rows = append(rows, tuiRow{path: answer.Path, group: group.Name, answer: answer})
		}
	}
	return rows
}

func (m *tuiModel) visibleReport() verdict.Report {
	if m.showAll {
		return m.report
	}
	return m.report.ViolationsOnly()
}

func (m tuiModel) View() tea.View {
	view := tea.NewView(m.frame())
	view.AltScreen = true
	return view
}

func (m *tuiModel) frame() string {
	rows := m.rows()
	view := "violations only"
	if m.showAll {
		view = "all rules"
	}
	lines := []string{
		fmt.Sprintf("vet report  base: %s  violations: %d  view: %s", m.report.Base, m.report.Violations, view),
		"↑/↓ or j/k move  a toggle passing  q quit",
		"",
	}
	if len(rows) == 0 {
		lines = append(lines, "No rules to display.")
		return strings.Join(lines, "\n") + "\n"
	}

	details := tuiDetailLines(rows[m.selected])
	rowLimit := len(rows)
	if m.size.height > 0 {
		rowLimit = m.size.height - len(lines) - len(details)
		if rowLimit < 1 {
			rowLimit = 1
		}
		if rowLimit > len(rows) {
			rowLimit = len(rows)
		}
	}
	start := 0
	if len(rows) > rowLimit {
		start = m.selected - rowLimit/2
		if start < 0 {
			start = 0
		}
		if start+rowLimit > len(rows) {
			start = len(rows) - rowLimit
		}
	}
	for index, row := range rows[start : start+rowLimit] {
		lines = append(lines, tuiRowLine(row, start+index == m.selected))
	}
	lines = append(lines, details...)
	return strings.Join(lines, "\n") + "\n"
}

func tuiRowLine(row tuiRow, selected bool) string {
	marker := " "
	if selected {
		marker = ">"
	}
	status := "PASS"
	if row.answer.Violates {
		status = "FAIL"
	}
	line := fmt.Sprintf("%s %-4s %s  %s  [%s] %s: %v", marker, status, row.path, row.group, row.answer.Type, row.answer.Rule, row.answer.Value)
	if row.answer.Label != "" {
		line += fmt.Sprintf(" (%s)", row.answer.Label)
	}
	return line
}

func tuiDetailLines(row tuiRow) []string {
	lines := []string{
		"",
		"Details",
		fmt.Sprintf("Rule: %s", row.answer.Rule),
		fmt.Sprintf("File: %s", row.path),
	}
	if row.group != "" {
		lines = append(lines, fmt.Sprintf("Group: %s", row.group))
	}
	if row.answer.Description != "" {
		lines = append(lines, fmt.Sprintf("Description: %s", row.answer.Description))
	}
	lines = append(lines, fmt.Sprintf("Value: %v", row.answer.Value))
	if row.answer.Confidence != nil {
		lines = append(lines, fmt.Sprintf("Confidence: %v", *row.answer.Confidence))
	}
	if len(row.answer.Probabilities) > 0 {
		lines = append(lines, fmt.Sprintf("Probabilities: %s", probabilityText(row.answer.Probabilities)))
	}
	if len(row.answer.Legend) > 0 {
		lines = append(lines, fmt.Sprintf("Legend: %s", legendText(row.answer.Legend)))
	}
	return lines
}

func probabilityText(values map[string]float64) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

func legendText(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

func runTUI(in io.Reader, out io.Writer, report verdict.Report, all bool, size tuiSize) error {
	program := tea.NewProgram(
		newTUIModel(report, all, size),
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithWindowSize(size.width, size.height),
	)
	if _, err := program.Run(); err != nil {
		if errors.Is(err, tea.ErrProgramKilled) {
			return nil
		}
		return err
	}
	return nil
}

func tuiTerminalAvailable(out io.Writer) bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && terminalWriter(out)
}

func renderTUI(out io.Writer, report verdict.Report, all bool) error {
	if !tuiTerminalAvailable(out) {
		return errors.New("tui output requires an interactive terminal")
	}
	return runTUI(os.Stdin, out, report, all, terminalSize())
}

func terminalSize() tuiSize {
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return tuiSize{}
	}
	return tuiSize{width: width, height: height}
}

func terminalWriter(out io.Writer) bool {
	writer, ok := out.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(writer.Fd()))
}
