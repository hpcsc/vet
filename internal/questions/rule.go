package questions

import (
	"errors"
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
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
	Source       string            `yaml:"-"`
}

func (r Rule) AppliesTo(path string) bool {
	if len(r.Files) > 0 {
		matched := false
		for _, pattern := range r.Files {
			if ok, _ := doublestar.Match(pattern, path); ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range r.Exclude {
		if ok, _ := doublestar.Match(pattern, path); ok {
			return false
		}
	}
	return true
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
	return nil
}