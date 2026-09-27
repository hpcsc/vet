// Package material resolves the repository material a questions file asks for
// into the text that goes into the prompt.
package material

import (
	"fmt"
	"strings"

	"github.com/hpcsc/vet/internal/questions"
)

// Section is one piece of repository material, kept apart from the file's own
// diff so a reader of the prompt — and a report of the run — can tell evidence
// from subject.
type Section struct {
	Include questions.Include
	Content string
}

// Bytes is what the material adds to the prompt, so a run can say what the
// evidence cost rather than leaving the prompt size to be guessed at.
func Bytes(sections []Section) int {
	total := 0
	for _, s := range sections {
		total += len(s.Content)
	}
	return total
}

// Render is the material as it goes into the prompt: one labeled section per
// piece, in the fixed order the questions file defined, so the same rules
// produce the same prompt every run.
func Render(sections []Section, filePath string) string {
	var b strings.Builder
	for _, s := range sections {
		if s.Content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "Repository material for %s (%s):\n\n%s", filePath, s.Include, s.Content)
	}
	return b.String()
}
