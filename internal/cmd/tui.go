package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hpcsc/vet/internal/style"
	"github.com/hpcsc/vet/internal/verdict"
	"golang.org/x/term"
)

const (
	tuiGap        = "  "
	tuiKindWidth  = 6
	tuiLabelWidth = 13

	// tuiValueGap keeps the value clear of the longest rule.
	tuiValueGap = 2

	// tuiFrameLines are the lines the list cannot take: header, blank, blank,
	// the box around the details, the details title, blank and hints.
	tuiFrameLines = 8
)

// tuiRowHeadWidth is the marker, mark, kind and gaps that open every row.
var tuiRowHeadWidth = lipgloss.Width(tuiGap + "▸ " + style.FailMark + tuiGap + tuiPad("choice", tuiKindWidth) + tuiGap)

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

type tuiLine struct {
	text   string
	row    int
	header bool
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

func (m *tuiModel) visibleReport() verdict.Report {
	if m.showAll {
		return m.report
	}
	return m.report.ViolationsOnly()
}

func (m *tuiModel) rows() []tuiRow {
	rows := make([]tuiRow, 0)
	for _, file := range m.visibleReport().ByFile() {
		for _, group := range file.Groups {
			for _, answer := range group.Answers {
				rows = append(rows, tuiRow{path: file.Path, group: group.Name, answer: answer})
			}
		}
	}
	return rows
}

func (m tuiModel) View() tea.View {
	view := tea.NewView(m.frame())
	view.AltScreen = true
	return view
}

func (m tuiModel) frame() string {
	rows := m.rows()
	lines := []string{m.headerLine(), ""}
	if len(rows) == 0 {
		lines = append(lines, tuiGap+"No rules to display.", "", m.hintLine())
		return m.fit(lines)
	}

	fields := m.detailFields(rows[m.selected])
	room := 0
	if m.size.height > 0 {
		room = m.listRoom(len(fields) + 1)
		for room < 1 && len(fields) > 0 {
			fields = fields[:len(fields)-1]
			room = m.listRoom(len(fields) + 1)
		}
		room = max(room, 1)
	}
	lines = append(lines, m.listLines(rows, room)...)
	lines = append(lines, "", m.detailsBox(rows[m.selected], fields), "", m.hintLine())
	return m.fit(lines)
}

func (m tuiModel) fit(lines []string) string {
	if m.size.height > 0 && len(lines) > m.size.height {
		lines = lines[:m.size.height]
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m tuiModel) headerLine() string {
	status := m.statusLine()
	name := style.Bold("vet")
	base := style.Faint(tuiGap + "·  base " + m.report.Base)
	if m.overflow(name + base + status) {
		base = ""
	}
	if m.overflow(name + base + tuiGap + status) {
		name = ""
	}
	head := name + base
	gap := m.size.width - lipgloss.Width(head) - lipgloss.Width(status)
	if gap < 1 {
		return tuiFit(head+tuiGap+status, m.size.width)
	}
	return head + strings.Repeat(" ", gap) + status
}

func (m tuiModel) statusLine() string {
	view := "violations only"
	if m.showAll {
		view = "all rules"
	}
	count := fmt.Sprintf("%d violations", m.report.Violations)
	switch m.report.Violations {
	case 0:
		count = style.Pass("no violations")
	case 1:
		count = style.Fail("1 violation")
	default:
		count = style.Fail(count)
	}
	return count + style.Faint(tuiGap+"·  "+view)
}

// overflow reports whether the parts ask for more room than the terminal has.
func (m tuiModel) overflow(parts string) bool {
	return m.size.width > 0 && lipgloss.Width(parts)+1 > m.size.width
}

func (m tuiModel) hintLine() string {
	hint := tuiGap + "↑↓ or j/k move" + tuiGap + "·  a toggle passing" + tuiGap + "·  q quit"
	if m.overflow(hint) {
		if short := style.Faint(tuiGap + "q quit"); !m.overflow(short) {
			return short
		}
	}
	return style.Faint(hint)
}

// detailsBox draws the details of the selected answer in a pane of its own,
// so the part of the screen that answers the selected rule cannot be mistaken
// for another row of the list. The pane spans the terminal, so it lines up
// with the list above it.
func (m tuiModel) detailsBox(row tuiRow, fields []string) string {
	lines := append([]string{m.detailTitle(row)}, fields...)
	frame := style.Frame()
	if m.size.width > 0 {
		frame = frame.Width(m.size.width)
	}
	return frame.Render(strings.Join(lines, "\n"))
}

// detailsRoom is the room a line inside the details pane has once the box
// takes its border, its padding and the indent the line carries.
func (m tuiModel) detailsRoom() int {
	frame := style.Frame()
	return m.size.width - frame.GetHorizontalBorderSize() - frame.GetHorizontalPadding() - lipgloss.Width(tuiGap)
}

func (m tuiModel) detailTitle(row tuiRow) string {
	rule, description := row.answer.Rule, row.answer.Description
	if m.size.width > 0 {
		room := m.detailsRoom()
		rule = tuiFit(rule, room)
		if rest := room - lipgloss.Width(rule) - lipgloss.Width(tuiGap+"·  "); rest > 0 {
			description = tuiFit(description, rest)
		} else {
			description = ""
		}
	}
	title := tuiGap + style.Rule(rule)
	if description != "" {
		title += style.Faint(tuiGap + "·  " + description)
	}
	return title
}

func (m tuiModel) detailFields(row tuiRow) []string {
	fields := [][2]string{}
	if row.path != "" {
		fields = append(fields, [2]string{"file", style.File(row.path)})
	}
	if row.group != "" {
		fields = append(fields, [2]string{"group", style.Group(row.group)})
	}
	fields = append(fields, [2]string{"value", row.answer.DisplayValue()})
	if row.answer.Label != "" {
		fields = append(fields, [2]string{"label", row.answer.Label})
	}
	if row.answer.Confidence != nil {
		fields = append(fields, [2]string{"confidence", fmt.Sprintf("%v", *row.answer.Confidence)})
	}
	if len(row.answer.Probabilities) > 0 {
		fields = append(fields, [2]string{"probabilities", probabilityText(row.answer.Probabilities)})
	}
	if len(row.answer.Legend) > 0 {
		fields = append(fields, [2]string{"legend", legendText(row.answer.Legend)})
	}

	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		value := field[1]
		if m.size.width > 0 {
			value = tuiFit(value, m.detailsRoom()-tuiLabelWidth-1)
		}
		lines = append(lines, tuiGap+style.Faint(tuiPad(field[0], tuiLabelWidth+1))+value)
	}
	return lines
}

// listRoom is the room the list can take below the details, or zero when the
// frame has no height to fit into.
func (m tuiModel) listRoom(detailLines int) int {
	if m.size.height <= 0 {
		return 0
	}
	return m.size.height - detailLines - tuiFrameLines
}

func (m tuiModel) listLines(rows []tuiRow, room int) []string {
	ruleWidth, valueWidth := m.columnWidths(rows)
	lines := make([]tuiLine, 0, len(rows))
	for index, row := range rows {
		if index == 0 || row.path != rows[index-1].path {
			lines = append(lines, tuiLine{text: tuiGap + style.File(row.path), header: true, row: index})
		}
		lines = append(lines, tuiLine{text: m.rowLine(row, index == m.selected, ruleWidth, valueWidth), row: index})
	}
	return m.draw(lines, room)
}

// columnWidths fits the rule and value columns to the terminal, giving the
// value room first because it carries the answer.
func (m tuiModel) columnWidths(rows []tuiRow) (rule, value int) {
	for _, row := range rows {
		if width := lipgloss.Width(row.answer.Rule); width > rule {
			rule = width
		}
		if width := lipgloss.Width(rowValue(row)); width > value {
			value = width
		}
	}
	if m.size.width <= 0 {
		return rule, value
	}
	room := max(m.size.width-tuiRowHeadWidth-tuiValueGap, 2)
	if value > room-1 {
		value = room - 1
	}
	if rule > room-value {
		rule = room - value
	}
	return max(rule, 1), max(value, 1)
}

func rowValue(row tuiRow) string {
	value := row.answer.DisplayValue()
	if row.answer.Label != "" {
		value += " (" + row.answer.Label + ")"
	}
	return value
}

func (m tuiModel) rowLine(row tuiRow, selected bool, ruleWidth, valueWidth int) string {
	mark, markColor := style.PassMark, style.Pass
	if row.answer.Violates {
		mark, markColor = style.FailMark, style.Fail
	}
	marker := "  "
	if selected {
		marker = style.Bold("▸") + " "
	}
	rule := tuiPad(tuiFit(row.answer.Rule, ruleWidth), ruleWidth)
	if selected {
		rule = style.Bold(style.Rule(rule))
	} else {
		rule = style.Rule(rule)
	}
	value := tuiFit(rowValue(row), valueWidth)
	gap := max(valueWidth-lipgloss.Width(value), tuiValueGap)
	head := tuiGap + marker + markColor(mark) + tuiGap + style.Type(tuiPad(string(row.answer.Type), tuiKindWidth)) + tuiGap
	return head + rule + strings.Repeat(" ", gap) + value
}

func (m tuiModel) draw(lines []tuiLine, room int) []string {
	if room <= 0 || room >= len(lines) {
		room = len(lines)
	}
	start := 0
	if len(lines) > room {
		start = m.lineAt(lines) - room/2
		if start < 0 {
			start = 0
		}
		if start+room > len(lines) {
			start = len(lines) - room
		}
	}
	drawn := make([]string, 0, room)
	for _, line := range lines[start : start+room] {
		drawn = append(drawn, line.text)
	}
	return drawn
}

func (m tuiModel) lineAt(lines []tuiLine) int {
	for index, line := range lines {
		if !line.header && line.row == m.selected {
			return index
		}
	}
	return 0
}

// tuiFit cuts a string to the room, marking the cut. A room of zero or less
// leaves the string alone, which is what an unknown terminal width asks for.
func tuiFit(s string, room int) string {
	if room <= 0 || lipgloss.Width(s) <= room {
		return s
	}
	if room == 1 {
		return "…"
	}
	return lipgloss.NewStyle().MaxWidth(room-1).Render(s) + "…"
}

func tuiPad(s string, to int) string {
	if gap := to - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
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
