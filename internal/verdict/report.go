package verdict

import (
	"fmt"
	"math"
	"strings"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/hpcsc/vet/internal/style"
)

type Row struct {
	Rule          string             `json:"rule"`
	Description   string             `json:"description,omitempty"`
	Path          string             `json:"path"`
	Value         any                `json:"value"`
	Type          questions.Kind     `json:"-"`
	Label         string             `json:"label,omitempty"`
	Violates      bool               `json:"violates,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

type Group struct {
	Name    string `json:"name"`
	Answers []Row  `json:"answers"`
}

type Report struct {
	Base       string  `json:"base"`
	Groups     []Group `json:"groups"`
	Violations int     `json:"violations"`
	fileOrder  []string
}

func Judge(base string, file questions.File, answers []backend.Answer) (Report, error) {
	rules := make(map[string]questions.Rule, len(file.Rules))
	for _, rule := range file.Rules {
		rules[rule.ID] = rule
	}

	report := Report{Base: base, Groups: make([]Group, 0), fileOrder: make([]string, 0)}
	groupIndex := map[string]int{}
	fileIndexes := map[string]struct{}{}
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
		if _, ok := fileIndexes[answer.Path]; !ok {
			fileIndexes[answer.Path] = struct{}{}
			report.fileOrder = append(report.fileOrder, answer.Path)
		}
		report.Groups[index].Answers = append(report.Groups[index].Answers, row)
		if row.Violates {
			report.Violations++
		}
	}
	return report, nil
}

func judge(rule questions.Rule, answer backend.Answer) (Row, error) {
	row := Row{
		Rule:          answer.Rule,
		Description:   rule.Description,
		Type:          rule.Type,
		Confidence:    answer.Confidence,
		Probabilities: answer.Probabilities,
		Legend:        answer.Legend,
	}
	switch rule.Type {
	case questions.Noul:
		row.Value = *answer.Noul
		row.Violates = row.Value.(float64) >= *rule.NoulLimit
	case questions.Choice:
		row.Value = *answer.Choice
		row.Label = rule.Choices[row.Value.(string)]
		row.Violates = row.Value.(string) == rule.ViolatesWhen
	case questions.Score:
		row.Value = *answer.Score
		if label, ok := row.Legend[fmt.Sprintf("%d", row.Value)]; ok {
			row.Label = label
		} else if *answer.Score < len(rule.Scores) {
			row.Label = rule.Scores[*answer.Score]
		}
		row.Violates = row.Value.(int) >= *rule.ScoreLimit
	default:
		return Row{}, fmt.Errorf("rule %s has no supported type", rule.ID)
	}
	return row, nil
}

func (a Row) displayRule() string {
	if a.Description != "" {
		return a.Description
	}
	return a.Rule
}

func (a Row) displayValue() string {
	if a.Type == questions.Noul {
		if v, ok := a.Value.(float64); ok {
			return fmt.Sprintf("%d%%", int(math.Round(v*100)))
		}
	}
	return fmt.Sprint(a.Value)
}

func (r Report) ViolationsOnly() Report {
	filtered := Report{
		Base:       r.Base,
		Groups:     make([]Group, 0),
		Violations: r.Violations,
		fileOrder:  append([]string(nil), r.fileOrder...),
	}
	for _, group := range r.Groups {
		answers := make([]Row, 0, len(group.Answers))
		for _, answer := range group.Answers {
			if answer.Violates {
				answers = append(answers, answer)
			}
		}
		if len(answers) > 0 {
			filtered.Groups = append(filtered.Groups, Group{Name: group.Name, Answers: answers})
		}
	}
	return filtered
}

type fileGroup struct {
	path         string
	groups       []Group
	groupIndexes map[string]int
}

func (r Report) Text() string {
	return r.text(false)
}

func (r Report) TextWithPassing() string {
	return r.text(true)
}

func (r Report) text(showPassing bool) string {
	files := make([]fileGroup, 0)
	fileIndexes := map[string]int{}
	for _, group := range r.Groups {
		for _, answer := range group.Answers {
			file, ok := fileIndexes[answer.Path]
			if !ok {
				file = len(files)
				fileIndexes[answer.Path] = file
				files = append(files, fileGroup{path: answer.Path, groupIndexes: map[string]int{}})
			}
			groupIndex, ok := files[file].groupIndexes[group.Name]
			if !ok {
				groupIndex = len(files[file].groups)
				files[file].groupIndexes[group.Name] = groupIndex
				files[file].groups = append(files[file].groups, Group{Name: group.Name})
			}
			files[file].groups[groupIndex].Answers = append(files[file].groups[groupIndex].Answers, answer)
		}
	}
	if len(r.fileOrder) > 0 {
		ordered := make([]fileGroup, 0, len(files))
		filesByPath := make(map[string]fileGroup, len(files))
		for _, file := range files {
			filesByPath[file.path] = file
		}
		for _, path := range r.fileOrder {
			if file, ok := filesByPath[path]; ok {
				ordered = append(ordered, file)
			}
		}
		files = ordered
	}

	var b strings.Builder
	for _, file := range files {
		fileWritten := false
		for _, group := range file.groups {
			answers := group.Answers
			if !showPassing {
				answers = make([]Row, 0, len(group.Answers))
				for _, answer := range group.Answers {
					if answer.Violates {
						answers = append(answers, answer)
					}
				}
			}
			if len(answers) == 0 {
				continue
			}
			if !fileWritten {
				b.WriteString(style.File(file.path))
				b.WriteString("\n")
				fileWritten = true
			}
			prefix := "  "
			if group.Name != "" {
				b.WriteString(prefix)
				b.WriteString(style.Group(group.Name))
				b.WriteString("\n")
				prefix += "  "
			}
			for _, answer := range answers {
				mark := style.Pass(style.PassMark)
				if answer.Violates {
					mark = style.Fail(style.FailMark)
				}
				b.WriteString(prefix)
				fmt.Fprintf(&b, "%s [%s] %s: %s", mark, style.Type(string(answer.Type)), style.Rule(answer.displayRule()), answer.displayValue())
				if answer.Label != "" {
					fmt.Fprintf(&b, " (%s)", answer.Label)
				}
				if answer.Confidence != nil {
					fmt.Fprintf(&b, " (confidence %v)", *answer.Confidence)
				}
				b.WriteString("\n")
			}
		}
		if fileWritten {
			b.WriteString("\n")
		}
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
	for _, file := range files {
		for _, group := range file.groups {
			for _, answer := range group.Answers {
				if !answer.Violates {
					continue
				}
				b.WriteString("  - ")
				b.WriteString(style.Rule(answer.displayRule()))
				if file.path != "" {
					b.WriteString(" in ")
					b.WriteString(style.File(file.path))
				}
				if group.Name != "" {
					b.WriteString(" (")
					b.WriteString(style.Group(group.Name))
					b.WriteString(")")
				}
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}
