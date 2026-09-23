package questions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	NoulLimit    *float64          `yaml:"noulLimit,omitempty"`
	Choices      map[string]string `yaml:"choices,omitempty"`
	ViolatesWhen string            `yaml:"violatesWhen,omitempty"`
	Scores       []string          `yaml:"scores,omitempty"`
	ScoreLimit   *int              `yaml:"scoreLimit,omitempty"`
}

func Load(path string) (File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return File{}, err
	}
	if info.IsDir() {
		return loadDirectory(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return Parse(data)
}

func loadDirectory(path string) (File, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return File{}, err
	}
	var file File
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(path, name))
		if err != nil {
			return File{}, err
		}
		parsed, err := Parse(data)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", name, err)
		}
		for _, rule := range parsed.Rules {
			if _, ok := seen[rule.ID]; ok {
				return File{}, fmt.Errorf("rule %s appears in more than one questions file", rule.ID)
			}
			seen[rule.ID] = struct{}{}
		}
		file.Rules = append(file.Rules, parsed.Rules...)
	}
	file.Version = 1
	if len(file.Rules) == 0 {
		return File{}, fmt.Errorf("the directory %s holds no questions files", path)
	}
	return file, nil
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

func Marshal(file File) ([]byte, error) {
	return yaml.Marshal(&file)
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
