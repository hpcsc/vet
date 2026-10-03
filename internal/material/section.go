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

// maxMaterialBytes caps what repository material one prompt carries. A large
// directory or file would otherwise push the prompt past the model's context,
// and the material is advisory anyway. The cap is generous for the small facts
// a rule usually needs (file names, declarations) and only bites a directory
// whose whole text would not fit.
const maxMaterialBytes = 16 * 1024

// cap keeps the material within maxMaterialBytes. Sections stay in prompt
// order; the first section that would cross the cap is cut short and the rest
// are dropped, so the small facts that lead the prompt survive and a rule never
// fails a run over evidence it could not hold anyway.
func cap(sections []Section) []Section {
	if Bytes(sections) <= maxMaterialBytes {
		return sections
	}
	budget := maxMaterialBytes
	kept := make([]Section, 0, len(sections))
	for _, s := range sections {
		switch {
		case len(s.Content) <= budget:
			kept = append(kept, s)
			budget -= len(s.Content)
		case budget > 3:
			s.Content = s.Content[:budget-3] + "..."
			kept = append(kept, s)
			return kept
		default:
			return kept
		}
	}
	return kept
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
