package verdict

import (
	"fmt"
	"strings"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/questions"
)

type Row struct {
	Rule       string   `json:"rule"`
	Path       string   `json:"path"`
	Value      any      `json:"value"`
	Violates   bool     `json:"violates,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type Group struct {
	Name    string `json:"name"`
	Answers []Row  `json:"answers"`
}

type Report struct {
	Base       string  `json:"base"`
	Groups     []Group `json:"groups"`
	Violations int     `json:"violations"`
}

func Judge(base string, file questions.File, answers []backend.Answer) (Report, error) {
	rules := make(map[string]questions.Rule, len(file.Rules))
	for _, rule := range file.Rules {
		rules[rule.ID] = rule
	}

	report := Report{Base: base}
	groupIndex := map[string]int{}
	for _, answer := range answers {
		rule, ok := rules[answer.Rule]
		if !ok {
			return Report{}, fmt.Errorf("answer for unknown rule %s", answer.Rule)
		}
		row, err := judge(rule, answer)
		if err != nil {
			return Report{}, err
		}
		row.Path = answer.Path
		index, ok := groupIndex[rule.Source]
		if !ok {
			index = len(report.Groups)
			groupIndex[rule.Source] = index
			report.Groups = append(report.Groups, Group{Name: rule.Source})
		}
		report.Groups[index].Answers = append(report.Groups[index].Answers, row)
		if row.Violates {
			report.Violations++
		}
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

func (r Report) Text() string {
	var b strings.Builder
	for _, g := range r.Groups {
		prefix := ""
		if g.Name != "" {
			b.WriteString(g.Name)
			b.WriteString("\n")
			prefix = "  "
		}
		lastPath := ""
		for _, a := range g.Answers {
			if a.Path != lastPath {
				if lastPath != "" {
					b.WriteString("\n")
				}
				b.WriteString(prefix)
				b.WriteString(a.Path)
				b.WriteString("\n")
				lastPath = a.Path
			}
			mark := "✓"
			if a.Violates {
				mark = "✗"
			}
			b.WriteString(prefix)
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
		b.WriteString("The change violates 1 rule.\n")
	} else {
		fmt.Fprintf(&b, "The change violates %d rules.\n", r.Violations)
	}
	for _, g := range r.Groups {
		for _, a := range g.Answers {
			if !a.Violates {
				continue
			}
			b.WriteString("  - ")
			b.WriteString(a.Rule)
			if a.Path != "" {
				b.WriteString(" in ")
				b.WriteString(a.Path)
			}
			if g.Name != "" {
				b.WriteString(" (")
				b.WriteString(g.Name)
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}