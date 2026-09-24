package questions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

type File struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
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