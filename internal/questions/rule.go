package questions

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hpcsc/vet/internal/diff"
)

type Rule struct {
	ID           string            `yaml:"id"`
	Description  string            `yaml:"description,omitempty"`
	Instructions string            `yaml:"instructions"`
	Type         Kind              `yaml:"type"`
	NoulLimit    *float64          `yaml:"noulLimit,omitempty"`
	Choices      map[string]string `yaml:"choices,omitempty"`
	ViolatesWhen string            `yaml:"violatesWhen,omitempty"`
	Scores       []string          `yaml:"scores,omitempty"`
	ScoreLimit   *int              `yaml:"scoreLimit,omitempty"`
	Files        []string          `yaml:"files,omitempty"`
	Exclude      []string          `yaml:"exclude,omitempty"`
	// RequiresAddedLine and RequiresRemovedLine are patterns that the change
	// must touch for the rule to apply. A rule with one of them is skipped when
	// the change adds or removes no line that matches.
	RequiresAddedLine   string `yaml:"requiresAddedLine,omitempty"`
	RequiresRemovedLine string `yaml:"requiresRemovedLine,omitempty"`
	Source              string `yaml:"-"`
}

func (r Rule) AppliesTo(change diff.File) bool {
	if len(r.Files) > 0 {
		matched := false
		for _, pattern := range r.Files {
			if ok, _ := doublestar.Match(pattern, change.Path); ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range r.Exclude {
		if ok, _ := doublestar.Match(pattern, change.Path); ok {
			return false
		}
	}
	return r.Needs(change)
}

func (r Rule) Needs(change diff.File) bool {
	return r.need(r.RequiresAddedLine, change.AddedLines()) &&
		r.need(r.RequiresRemovedLine, change.RemovedLines())
}

func (r Rule) need(pattern string, lines []string) bool {
	if pattern == "" {
		return true
	}
	expression, err := regexp.Compile(pattern)
	if err != nil {
		// validate rejects a bad pattern at load, so this is unreachable. Judging
		// the rule anyway keeps a broken pattern from quietly dropping it.
		return true
	}
	for _, line := range lines {
		if expression.MatchString(line) {
			return true
		}
	}
	return false
}

func (r Rule) validate() error {
	if r.ID == "" {
		return errors.New("a rule has no id")
	}
	if r.Instructions == "" {
		return fmt.Errorf("rule %s has no instructions", r.ID)
	}
	if !r.Type.valid() {
		return fmt.Errorf("rule %s has no supported type", r.ID)
	}
	switch r.Type {
	case Noul:
		if r.NoulLimit == nil {
			return fmt.Errorf("rule %s has no noulLimit", r.ID)
		}
		if *r.NoulLimit < 0 || *r.NoulLimit > 1 {
			return fmt.Errorf("rule %s has noulLimit %f outside 0 to 1", r.ID, *r.NoulLimit)
		}
	case Choice:
		if _, ok := r.Choices[r.ViolatesWhen]; !ok {
			return fmt.Errorf("rule %s names violatesWhen %q that is not a choice", r.ID, r.ViolatesWhen)
		}
	case Score:
		if r.ScoreLimit == nil {
			return fmt.Errorf("rule %s has no scoreLimit", r.ID)
		}
		if *r.ScoreLimit < 0 || *r.ScoreLimit >= len(r.Scores) {
			return fmt.Errorf("rule %s has scoreLimit %d outside the scores range", r.ID, *r.ScoreLimit)
		}
	}
	for _, pattern := range r.Files {
		if !doublestar.ValidatePattern(pattern) {
			return fmt.Errorf("rule %s has an invalid files pattern %s", r.ID, pattern)
		}
	}
	for _, pattern := range r.Exclude {
		if !doublestar.ValidatePattern(pattern) {
			return fmt.Errorf("rule %s has an invalid exclude pattern %s", r.ID, pattern)
		}
	}
	for name, pattern := range map[string]string{
		"requiresAddedLine":   r.RequiresAddedLine,
		"requiresRemovedLine": r.RequiresRemovedLine,
	} {
		if pattern == "" {
			continue
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("rule %s has an invalid %s pattern %s", r.ID, name, pattern)
		}
	}
	return nil
}