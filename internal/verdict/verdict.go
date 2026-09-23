package verdict

import (
	"fmt"
	"strings"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/questions"
)

type Row struct {
	Rule       string   `json:"rule"`
	Value      any      `json:"value"`
	Violates   bool     `json:"violates,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type File struct {
	Path    string `json:"path"`
	Answers []Row  `json:"answers"`
}

type Report struct {
	Base       string `json:"base"`
	Files      []File `json:"files"`
	Violations int    `json:"violations"`
}

type FileAnswers struct {
	Path    string
	Answers []backend.Answer
}

// Judge turns the answers of every file into a report, checking each one
// against its rule.
func Judge(base string, file questions.File, files []FileAnswers) (Report, error) {
	rules := make(map[string]questions.Rule, len(file.Rules))
	for _, rule := range file.Rules {
		rules[rule.ID] = rule
	}

	report := Report{Base: base, Files: make([]File, 0, len(files))}
	for _, fileAnswers := range files {
		answers := make([]Row, 0, len(fileAnswers.Answers))
		for _, answer := range fileAnswers.Answers {
			rule, ok := rules[answer.Rule]
			if !ok {
				return Report{}, fmt.Errorf("answer for unknown rule %s", answer.Rule)
			}
			row, err := judge(rule, answer)
			if err != nil {
				return Report{}, err
			}
			if row.Violates {
				report.Violations++
			}
			answers = append(answers, row)
		}
		report.Files = append(report.Files, File{Path: fileAnswers.Path, Answers: answers})
	}
	return report, nil
}

func judge(rule questions.Rule, answer backend.Answer) (Row, error) {
	row := Row{Rule: answer.Rule, Confidence: answer.Confidence}
	switch rule.Type {
	case questions.Noul:
		row.Value = *answer.Noul
		row.Violates = row.Value.(float64) >= *rule.NoulLimit
	case questions.Choice:
		row.Value = *answer.Choice
		row.Violates = row.Value.(string) == rule.ViolatesWhen
	case questions.Score:
		row.Value = *answer.Score
		row.Violates = row.Value.(int) >= *rule.ScoreLimit
	default:
		return Row{}, fmt.Errorf("rule %s has no supported type", rule.ID)
	}
	return row, nil
}

// Text renders the report as a list of files with a check or a cross per
// rule, and a summary line.
func (r Report) Text() string {
	var b strings.Builder
	for _, f := range r.Files {
		b.WriteString(f.Path)
		b.WriteString("\n")
		for _, a := range f.Answers {
			mark := "✓"
			if a.Violates {
				mark = "✗"
			}
			fmt.Fprintf(&b, "  %s %s: %v", mark, a.Rule, a.Value)
			if a.Confidence != nil {
				fmt.Fprintf(&b, " (confidence %v)", *a.Confidence)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if r.Violations == 0 {
		b.WriteString("The change violates no rule.")
		return b.String()
	}
	if r.Violations == 1 {
		b.WriteString("The change violates 1 rule.")
		return b.String()
	}
	fmt.Fprintf(&b, "The change violates %d rules.", r.Violations)
	return b.String()
}
