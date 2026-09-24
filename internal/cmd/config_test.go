//go:build unit

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcsc/vet/internal/config"
	"github.com/hpcsc/vet/internal/gittest"
	"github.com/stretchr/testify/require"
)

func TestConfigExampleCommand(t *testing.T) {
	t.Run("prints the default config file", func(t *testing.T) {
		command := newConfigExampleCommand()
		var out bytes.Buffer
		command.Writer = &out

		err := command.Run(context.Background(), []string{"example"})

		require.NoError(t, err)
		file, err := config.ParseFile(out.String())
		require.NoError(t, err)
		require.Equal(t, config.Default(), file.Resolve())
	})
}

func TestConfigInitCommand(t *testing.T) {
	t.Run("writes the default config file to the given path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		var out bytes.Buffer
		command := newConfigInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--path", path})

		require.NoError(t, err)
		file, err := config.LoadFile(path)
		require.NoError(t, err)
		require.Equal(t, config.Default(), file.Resolve())
	})

	t.Run("writes the default config file where vet looks for it", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", home)
		var out bytes.Buffer
		command := newConfigInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init"})

		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(home, "vet", "config.yaml"))
		require.NoError(t, err)
	})

	t.Run("refuses to overwrite an existing file without --force", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("model: rowan\n"), 0o600))
		var out bytes.Buffer
		command := newConfigInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--path", path})

		require.ErrorContains(t, err, "already exists")
		file, err := config.LoadFile(path)
		require.NoError(t, err)
		require.Equal(t, "rowan", file.Resolve().Model)
	})

	t.Run("overwrites an existing file with --force", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("model: rowan\n"), 0o600))
		var out bytes.Buffer
		command := newConfigInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--force", "--path", path})

		require.NoError(t, err)
		file, err := config.LoadFile(path)
		require.NoError(t, err)
		require.Equal(t, config.Default(), file.Resolve())
	})
}

func TestResolveConfig(t *testing.T) {
	ctx := context.Background()
	noHome := func(string) string { return "" }

	t.Run("load", func(t *testing.T) {
		t.Run("an explicit config path that is missing is an error", func(t *testing.T) {
			_, err := resolveConfig(ctx, t.TempDir(), filepath.Join(t.TempDir(), "missing.yaml"), noHome)

			require.Error(t, err)
		})

		t.Run("a missing default config path means the defaults", func(t *testing.T) {
			file, err := resolveConfig(ctx, t.TempDir(), "", noHome)

			require.NoError(t, err)
			require.Equal(t, config.Default(), file.Resolve())
		})

		t.Run("a config file with options is loaded from the explicit path", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte("model: rowan\n"), 0o600))

			file, err := resolveConfig(ctx, t.TempDir(), path, noHome)

			require.NoError(t, err)
			require.Equal(t, "rowan", file.Resolve().Model)
		})
	})

	t.Run("merge", func(t *testing.T) {
		t.Run("a .vet.yaml at the repository root overrides the config file", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Write(".vet.yaml", "model: overlay-model\n")
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte("model: global-model\nquestionsFile: global.yaml\n"), 0o600))

			file, err := resolveConfig(ctx, repo.Dir, path, noHome)

			require.NoError(t, err)
			resolved := file.Resolve()
			require.Equal(t, "overlay-model", resolved.Model)
			require.Equal(t, "global.yaml", resolved.QuestionsFile)
		})

		t.Run("a repository without a .vet.yaml keeps the config file", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte("model: global-model\n"), 0o600))

			file, err := resolveConfig(ctx, repo.Dir, path, noHome)

			require.NoError(t, err)
			require.Equal(t, "global-model", file.Resolve().Model)
		})

		t.Run("a missing default config with a .vet.yaml is just the overlay", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Write(".vet.yaml", "apiKeyCommand: fnox get TYPESAFE_API_KEY\n")

			file, err := resolveConfig(ctx, repo.Dir, "", noHome)

			require.NoError(t, err)
			require.Equal(t, "fnox get TYPESAFE_API_KEY", file.Resolve().APIKeyCommand)
		})
	})
}

func TestResolveValues(t *testing.T) {
	emptyEnv := func(string) string { return "" }

	t.Run("api url", func(t *testing.T) {
		t.Run("the flag wins", func(t *testing.T) {
			env := func(string) string { return "https://env.example" }

			require.Equal(t, "https://flag.example", apiURLOf("https://flag.example", "https://cfg.example", env))
		})

		t.Run("falls back to the environment", func(t *testing.T) {
			env := func(string) string { return "https://env.example" }

			require.Equal(t, "https://env.example", apiURLOf("", "https://cfg.example", env))
		})

		t.Run("falls back to the config", func(t *testing.T) {
			require.Equal(t, "https://cfg.example", apiURLOf("", "https://cfg.example", emptyEnv))
		})

		t.Run("falls back to the default", func(t *testing.T) {
			require.Equal(t, config.DefaultAPIURL, apiURLOf("", "", emptyEnv))
		})
	})

	t.Run("model", func(t *testing.T) {
		t.Run("the flag wins", func(t *testing.T) {
			require.Equal(t, "flag-model", modelOf("flag-model", "cfg-model"))
		})

		t.Run("falls back to the config", func(t *testing.T) {
			require.Equal(t, "cfg-model", modelOf("", "cfg-model"))
		})

		t.Run("falls back to the default", func(t *testing.T) {
			require.Equal(t, config.DefaultModel, modelOf("", ""))
		})
	})

	t.Run("api key", func(t *testing.T) {
		run := func(string) (string, error) { return "key-from-command", nil }

		t.Run("the flag wins", func(t *testing.T) {
			key, err := apiKeyOf("flag-key", "command", emptyEnv, run)

			require.NoError(t, err)
			require.Equal(t, "flag-key", key)
		})

		t.Run("falls back to the environment", func(t *testing.T) {
			venv := func(string) string { return "env-key" }

			key, err := apiKeyOf("", "command", venv, run)

			require.NoError(t, err)
			require.Equal(t, "env-key", key)
		})

		t.Run("falls back to the api-key-command", func(t *testing.T) {
			key, err := apiKeyOf("", "command", emptyEnv, run)

			require.NoError(t, err)
			require.Equal(t, "key-from-command", key)
		})

		t.Run("an api-key-command that fails is an error", func(t *testing.T) {
			fail := func(string) (string, error) { return "", errors.New("boom") }

			_, err := apiKeyOf("", "command", emptyEnv, fail)

			require.ErrorContains(t, err, "boom")
		})

		t.Run("no key source at all is an error", func(t *testing.T) {
			_, err := apiKeyOf("", "", emptyEnv, run)

			require.Error(t, err)
		})
	})

	t.Run("questions", func(t *testing.T) {
		t.Run("the flag wins", func(t *testing.T) {
			path, err := questionsPathOf("rules.yaml", "config.yaml", t.TempDir())

			require.NoError(t, err)
			require.Equal(t, "rules.yaml", path)
		})

		t.Run("falls back to the config", func(t *testing.T) {
			path, err := questionsPathOf("", "config.yaml", t.TempDir())

			require.NoError(t, err)
			require.Equal(t, "config.yaml", path)
		})

		t.Run("a ~ in the flag expands to the home directory", func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			path, err := questionsPathOf("~/rules.yaml", "", home)

			require.NoError(t, err)
			require.Equal(t, filepath.Join(home, "rules.yaml"), path)
		})

		t.Run("a ~ in the config expands to the home directory", func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			path, err := questionsPathOf("", "~/.config/vet/questions", home)

			require.NoError(t, err)
			require.Equal(t, filepath.Join(home, ".config", "vet", "questions"), path)
		})

		t.Run("a bare ~ expands to the home directory itself", func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			path, err := questionsPathOf("~", "", home)

			require.NoError(t, err)
			require.Equal(t, home, path)
		})

		t.Run("falls back to questions.yaml in the working directory", func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "questions.yaml"), []byte("version: 1\nrules: []\n"), 0o600))

			path, err := questionsPathOf("", "", dir)

			require.NoError(t, err)
			require.Equal(t, filepath.Join(dir, "questions.yaml"), path)
		})

		t.Run("no questions file at all is an error", func(t *testing.T) {
			_, err := questionsPathOf("", "", t.TempDir())

			require.Error(t, err)
		})
	})
}
