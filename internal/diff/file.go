package diff

import "strings"

type File struct {
	Path string
	From string
	Diff string
}

func (f File) AddedLines() []string {
	return f.lines('+')
}

func (f File) RemovedLines() []string {
	return f.lines('-')
}

func (f File) lines(sign byte) []string {
	var found []string
	inHunk := false
	for line := range strings.Lines(f.Diff) {
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && len(line) > 0 && line[0] == sign:
			found = append(found, line[1:])
		}
	}
	return found
}
