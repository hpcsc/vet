package questions

import (
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

type Kind string

const (
	Noul   Kind = "noul"
	Choice Kind = "choice"
	Score  Kind = "score"
)

func (k Kind) valid() bool {
	return k == Noul || k == Choice || k == Score
}

type File struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

type Rule struct {
	ID           string            `yaml:"id"`
	Instructions string            `yaml:"instructions"`
	Type         Kind              `yaml:"type"`
	NoulLimit    *float64          `yaml:"noulLimit"`
	Choices      map[string]string `yaml:"choices"`
	ViolatesWhen string            `yaml:"violatesWhen"`
	Scores       []string          `yaml:"scores"`
	ScoreLimit   *int              `yaml:"scoreLimit"`
}

func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return Parse(data)
}

func Parse(data []byte) (File, error) {
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("read the questions file: %w", err)
	}
	if err := file.validate(); err != nil {
		return File{}, err
	}
	return file, nil
}

func (f File) validate() error {
	if f.Version != 1 {
		return errors.New("unsupported version, only version 1 is supported")
	}
	for _, rule := range f.Rules {
		if err := rule.validate(); err != nil {
			return err
		}
	}
	return nil
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
	return nil
}
