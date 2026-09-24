package progress

import (
	"fmt"
	"io"
)

type Line struct {
	w        io.Writer
	terminal bool
	label    string
	shown    string
}

func Start(w io.Writer, terminal bool, label string) *Line {
	l := &Line{w: w, terminal: terminal, label: label}
	if terminal {
		l.draw(label + "…")
	} else {
		fmt.Fprintln(w, label+"…")
	}
	return l
}

// total -1 means the server sends no size.
func (l *Line) Bytes(done, total int64) {
	if !l.terminal {
		return
	}
	text := l.label + ": " + size(done)
	if total > 0 {
		text += fmt.Sprintf(" of %s (%d%%)", size(total), done*100/total)
	}
	l.draw(text)
}

func (l *Line) End() {
	if l.terminal {
		fmt.Fprintln(l.w)
	}
}

func (l *Line) draw(text string) {
	if text == l.shown {
		return
	}
	// \r goes back to the start of the line, and \x1b[K clears what a longer
	// text left after it.
	fmt.Fprint(l.w, "\r"+text+"\x1b[K")
	l.shown = text
}

func size(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	case n < 1000*1000:
		return fmt.Sprintf("%.1f KB", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/1000/1000)
	}
}
