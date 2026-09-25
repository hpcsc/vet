package questions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.yaml.in/yaml/v3"
)

type File struct {
	Version int      `yaml:"version"`
	Name    string   `yaml:"name,omitempty"`
	Context string   `yaml:"context,omitempty"`
	Files   []string `yaml:"files,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
	Rules   []Rule   `yaml:"rules"`
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
	file, err := Parse(data, filepath.Dir(path))
	if err != nil {
		return File{}, err
	}
	for i := range file.Rules {
		file.Rules[i].Source = nameOf(file.Name, filepath.Base(path))
	}
	return file, nil
}

func nameOf(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

func loadDirectory(path string) (File, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return File{}, err
	}
	var file File
	seen := map[string]struct{}{}
	contexts := []string{}
	seenContexts := map[string]struct{}{}
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
		parsed, err := Parse(data, path)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", name, err)
		}
		for i := range parsed.Rules {
			parsed.Rules[i].Source = nameOf(parsed.Name, name)
			if _, ok := seen[parsed.Rules[i].ID]; ok {
				return File{}, fmt.Errorf("rule %s appears in more than one questions file", parsed.Rules[i].ID)
			}
			seen[parsed.Rules[i].ID] = struct{}{}
		}
		if _, ok := seenContexts[parsed.Context]; !ok {
			seenContexts[parsed.Context] = struct{}{}
			if parsed.Context != "" {
				contexts = append(contexts, parsed.Context)
			}
		}
		file.Rules = append(file.Rules, parsed.Rules...)
	}
	file.Version = 1
	if len(contexts) > 0 {
		file.Context = strings.Join(contexts, "\n\n")
	}
	if len(file.Rules) == 0 {
		return File{}, fmt.Errorf("the directory %s holds no questions files", path)
	}
	return file, nil
}

func Parse(data []byte, dir string) (File, error) {
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("read the questions file: %w", err)
	}
	if err := file.resolveReferences(dir); err != nil {
		return File{}, err
	}
	if err := file.validate(); err != nil {
		return File{}, err
	}
	file.foldScope()
	return file, nil
}

func (f *File) foldScope() {
	if len(f.Files) == 0 && len(f.Exclude) == 0 {
		return
	}
	for i := range f.Rules {
		rule := &f.Rules[i]
		if len(rule.Files) == 0 {
			rule.Files = append([]string(nil), f.Files...)
		}
		if len(f.Exclude) > 0 {
			rule.Exclude = append(append([]string(nil), f.Exclude...), rule.Exclude...)
		}
	}
	f.Files = nil
	f.Exclude = nil
}

func (f *File) resolveReferences(dir string) error {
	if f.Context != "" {
		resolved, err := f.readReference(dir, f.Context)
		if err != nil {
			return fmt.Errorf("read the context: %w", err)
		}
		f.Context = resolved
	}
	for i := range f.Rules {
		instructions, err := f.readReference(dir, f.Rules[i].Instructions)
		if err != nil {
			return fmt.Errorf("read the instructions of rule %s: %w", f.Rules[i].ID, err)
		}
		f.Rules[i].Instructions = instructions
	}
	return nil
}

func (f File) readReference(dir, value string) (string, error) {
	if value == "" || value[0] != '@' {
		return value, nil
	}
	ref := strings.TrimPrefix(value, "@")
	if ref == "" {
		return "", errors.New("an @ reference has no path")
	}
	if ref == "~" || strings.HasPrefix(ref, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			ref = filepath.Join(home, strings.TrimPrefix(ref, "~"))
		}
	}
	if !filepath.IsAbs(ref) {
		ref = filepath.Join(dir, ref)
	}
	data, err := os.ReadFile(ref)
	if err != nil {
		return "", fmt.Errorf("read the referenced file %s: %w", ref, err)
	}
	return string(data), nil
}

func Marshal(file File) ([]byte, error) {
	return yaml.Marshal(&file)
}

func (f File) validate() error {
	if f.Version != 1 {
		return errors.New("unsupported version, only version 1 is supported")
	}
	for _, pattern := range f.Files {
		if !doublestar.ValidatePattern(pattern) {
			return fmt.Errorf("the questions file has an invalid files pattern %s", pattern)
		}
	}
	for _, pattern := range f.Exclude {
		if !doublestar.ValidatePattern(pattern) {
			return fmt.Errorf("the questions file has an invalid exclude pattern %s", pattern)
		}
	}
	for _, rule := range f.Rules {
		if err := rule.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (f File) ForPath(path string) (File, bool) {
	var rules []Rule
	for _, rule := range f.Rules {
		if rule.AppliesTo(path) {
			rules = append(rules, rule)
		}
	}
	f.Rules = rules
	return f, len(rules) > 0
}
