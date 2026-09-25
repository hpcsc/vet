package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/hpcsc/vet/internal/verdict"
	"golang.org/x/term"
)

type tuiModel struct {
	report   verdict.Report
	showAll  bool
	selected int
}

type tuiRow struct {
	path   string
	group  string
	answer verdict.Row
}

type tuiKey byte

const (
	tuiKeyQuit tuiKey = iota
	tuiKeyUp
	tuiKeyDown
	tuiKeyToggle
)

func newTUIModel(report verdict.Report, all bool) tuiModel {
	return tuiModel{report: report, showAll: all}
}

func (m *tuiModel) visibleReport() verdict.Report {
	if m.showAll {
		return m.report
	}
	return m.report.ViolationsOnly()
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

func (m *tuiModel) frame(height int) string {
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
		return tuiFrameText(lines, height)
	}

	details := tuiDetailLines(rows[m.selected])
	rowLimit := len(rows)
	if height > 0 {
		rowLimit = height - len(lines) - len(details)
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
	return tuiFrameText(lines, height)
}

func tuiFrameText(lines []string, height int) string {
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
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

func readTUIKey(reader *bufio.Reader) (tuiKey, error) {
	value, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}
	switch value {
	case 'q', 3:
		return tuiKeyQuit, nil
	case 'k':
		return tuiKeyUp, nil
	case 'j':
		return tuiKeyDown, nil
	case 'a':
		return tuiKeyToggle, nil
	case 27:
		sequence, err := reader.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("read TUI key sequence: %w", err)
		}
		if sequence != '[' {
			return 0, fmt.Errorf("unknown TUI input %q", string(value))
		}
		direction, err := reader.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("read TUI key sequence: %w", err)
		}
		switch direction {
		case 'A':
			return tuiKeyUp, nil
		case 'B':
			return tuiKeyDown, nil
		default:
			return 0, fmt.Errorf("unknown TUI key sequence %q", direction)
		}
	default:
		return 0, fmt.Errorf("unknown TUI input %q", string(value))
	}
}

func runTUI(in io.Reader, out io.Writer, report verdict.Report, all bool, height func() int) error {
	model := newTUIModel(report, all)
	reader := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprintf(out, "\x1b[2J\x1b[H%s", model.frame(height())); err != nil {
			return err
		}
		key, err := readTUIKey(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("TUI input ended before quit")
			}
			return err
		}
		switch key {
		case tuiKeyQuit:
			return nil
		case tuiKeyUp:
			model.move(-1)
		case tuiKeyDown:
			model.move(1)
		case tuiKeyToggle:
			model.toggle()
		}
	}
}

func tuiTerminalAvailable(out io.Writer) bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && terminalWriter(out)
}

func renderTUI(out io.Writer, report verdict.Report, all bool) error {
	if !tuiTerminalAvailable(out) {
		return errors.New("tui output requires an interactive terminal")
	}
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("enable tui input: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), state)
	if _, err := fmt.Fprint(out, "\x1b[?1049h\x1b[?25l"); err != nil {
		return err
	}
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?1049l")
	return runTUI(os.Stdin, rawTerminalWriter{out: out}, report, all, terminalHeight)
}

type rawTerminalWriter struct {
	out io.Writer
}

func (w rawTerminalWriter) Write(p []byte) (int, error) {
	replaced := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
	n, err := w.out.Write(replaced)
	if err != nil {
		return 0, err
	}
	if n != len(replaced) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}

func terminalHeight() int {
	_, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return 0
	}
	return height
}

func terminalWriter(out io.Writer) bool {
	writer, ok := out.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(writer.Fd()))
}
