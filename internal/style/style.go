package style

import (
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

const (
	PassMark = "✓"
	FailMark = "✗"
)

var (
	boldStyle  = lipgloss.NewStyle().Bold(true)
	faintStyle = lipgloss.NewStyle().Faint(true)
	fileColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	groupColor = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	typeColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	ruleColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	passColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	failColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	// lipgloss paints every string, and the text report is assembled as a
	// string, so the color profile decides here instead of at write time.
	useColor = colorprofile.Detect(os.Stdout, os.Environ()) > colorprofile.Ascii
)

func Bold(s string) string  { return colorize(boldStyle, s) }
func Faint(s string) string { return colorize(faintStyle, s) }
func File(s string) string  { return colorize(fileColor, s) }
func Group(s string) string { return colorize(groupColor, s) }
func Type(s string) string  { return colorize(typeColor, s) }
func Rule(s string) string  { return colorize(ruleColor, s) }
func Pass(s string) string  { return colorize(passColor, s) }
func Fail(s string) string  { return colorize(failColor, s) }

func colorize(paint lipgloss.Style, s string) string {
	if !useColor {
		return s
	}
	return paint.Render(s)
}
