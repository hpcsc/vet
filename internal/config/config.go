package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	DefaultAPIURL = "https://api.typesafe.ai/v1/systemone"
	DefaultModel  = "jev-latest"
)

type Config struct {
	APIKeyCommand string `yaml:"apiKeyCommand,omitempty"`
	QuestionsFile string `yaml:"questionsFile,omitempty"`
	APIURL        string `yaml:"apiUrl,omitempty"`
	Model         string `yaml:"model,omitempty"`
}

func Default() Config {
	return Config{APIURL: DefaultAPIURL, Model: DefaultModel}
}

// File is a config file as written, where an option vet never saw stays nil,
// so an overlay can leave a field to the file under it.
type File struct {
	APIKeyCommand *string `yaml:"apiKeyCommand"`
	QuestionsFile *string `yaml:"questionsFile"`
	APIURL        *string `yaml:"apiUrl"`
	Model         *string `yaml:"model"`
}

func (f File) Resolve() Config {
	c := Default()
	if f.APIKeyCommand != nil {
		c.APIKeyCommand = *f.APIKeyCommand
	}
	if f.QuestionsFile != nil {
		c.QuestionsFile = *f.QuestionsFile
	}
	if f.APIURL != nil {
		c.APIURL = *f.APIURL
	}
	if f.Model != nil {
		c.Model = *f.Model
	}
	return c
}

func (f File) Merge(over File) File {
	if over.APIKeyCommand != nil {
		f.APIKeyCommand = over.APIKeyCommand
	}
	if over.QuestionsFile != nil {
		f.QuestionsFile = over.QuestionsFile
	}
	if over.APIURL != nil {
		f.APIURL = over.APIURL
	}
	if over.Model != nil {
		f.Model = over.Model
	}
	return f
}

func Path(getenv func(string) string) string {
	if dir := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "vet", "config.yaml")
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "vet", "config.yaml")
	}
	return ""
}

func QuestionsPath(getenv func(string) string) string {
	if dir := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "vet", "questions.yaml")
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "vet", "questions.yaml")
	}
	return ""
}

func LoadFile(path string) (File, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	f, err := ParseFile(string(text))
	if err != nil {
		message := err.Error()
		if strings.Contains(message, "\n") {
			return File{}, fmt.Errorf("%s:\n  %s", path, strings.ReplaceAll(message, "\n", "\n  "))
		}
		return File{}, fmt.Errorf("%s: %s", path, message)
	}
	return f, nil
}

func ParseFile(text string) (File, error) {
	if strings.TrimSpace(text) == "" {
		return File{}, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return File{}, err
	}
	if len(root.Content) == 0 {
		return File{}, nil
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return File{}, errors.New("want a mapping")
	}
	known := map[string]bool{
		"apiKeyCommand": true,
		"questionsFile": true,
		"apiUrl":        true,
		"model":         true,
	}
	var problems []error
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		if !known[key] {
			problems = append(problems, fmt.Errorf("unknown option %s", key))
			continue
		}
		if node := mapping.Content[i+1]; node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			problems = append(problems, fmt.Errorf("option %s: want a string", key))
		}
	}
	var f File
	if err := mapping.Decode(&f); err != nil {
		problems = append(problems, err)
	}
	if len(problems) > 0 {
		return File{}, errors.Join(problems...)
	}
	return f, nil
}

func (c Config) YAML() ([]byte, error) {
	return yaml.Marshal(&c)
}
