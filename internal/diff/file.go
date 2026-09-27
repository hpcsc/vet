package diff

import "strings"

type File struct {
	Path string
	From string
	Diff string
}

func (f File) AddedLines() []string {
	var added []string
	inHunk := false
	for line := range strings.Lines(f.Diff) {
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && strings.HasPrefix(line, "+"):
			added = append(added, strings.TrimPrefix(line, "+"))
		}
	}
	return added
}
