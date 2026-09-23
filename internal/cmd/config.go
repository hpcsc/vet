package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hpcsc/vet/internal/config"
	"github.com/hpcsc/vet/internal/git"
)

const overlayName = ".vet.yaml"

// resolveConfig merges the config file, when there is one, with the overlay
// file of the working tree, when there is one. An explicit configPath makes a
// missing file an error; otherwise a missing file means the defaults.
func resolveConfig(ctx context.Context, repoDir, configPath string, pathOf func(string) string) (config.File, error) {
	base, err := loadConfigFile(configPath, pathOf)
	if err != nil {
		return config.File{}, err
	}
	overlay, err := loadOverlay(ctx, repoDir)
	if err != nil {
		return config.File{}, err
	}
	return base.Merge(overlay), nil
}

func loadConfigFile(configPath string, pathOf func(string) string) (config.File, error) {
	explicit := configPath != ""
	if !explicit {
		configPath = config.Path(pathOf)
	}
	file, err := config.LoadFile(configPath)
	switch {
	case err == nil:
		return file, nil
	case explicit && errors.Is(err, fs.ErrNotExist):
		return config.File{}, fmt.Errorf("no config file at %s", configPath)
	case errors.Is(err, fs.ErrNotExist):
		return config.File{}, nil
	default:
		return config.File{}, err
	}
}

func loadOverlay(ctx context.Context, repoDir string) (config.File, error) {
	top, err := git.New(repoDir).Toplevel(ctx)
	if err != nil {
		return config.File{}, nil
	}
	path := filepath.Join(top, overlayName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return config.File{}, nil
		}
		return config.File{}, err
	}
	return config.LoadFile(path)
}

func apiURLOf(flag, cfg string, pathOf func(string) string) string {
	if flag != "" {
		return flag
	}
	if url := pathOf("TYPESAFE_API_URL"); url != "" {
		return url
	}
	if cfg != "" {
		return cfg
	}
	return config.DefaultAPIURL
}

func modelOf(flag, cfg string) string {
	if flag != "" {
		return flag
	}
	if cfg != "" {
		return cfg
	}
	return config.DefaultModel
}

func apiKeyOf(flag, command string, pathOf func(string) string, run func(string) (string, error)) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if key := pathOf("TYPESAFE_API_KEY"); key != "" {
		return key, nil
	}
	if command != "" {
		return run(command)
	}
	return "", errors.New("no API key: pass --api-key, set TYPESAFE_API_KEY, or set api-key-command in the config")
}

func runKeyCommand(command string) (string, error) {
	out, err := exec.Command("sh", "-c", command).Output()
	if err != nil {
		return "", fmt.Errorf("run the api-key-command: %w", err)
	}
	key := strings.TrimSpace(string(out))
	if key == "" {
		return "", errors.New("run the api-key-command: it gave no key")
	}
	return key, nil
}

func questionsPathOf(flag, cfg, cwd string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if cfg != "" {
		return cfg, nil
	}
	path := filepath.Join(cwd, "questions.yaml")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", errors.New("no questions file: pass --questions, set questionsFile in the config, or put questions.yaml in the working directory")
}
