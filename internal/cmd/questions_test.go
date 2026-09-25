//go:build unit

package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcsc/vet/internal/questions"
	"github.com/stretchr/testify/require"
)

func TestQuestionsExampleCommand(t *testing.T) {
	t.Run("prints the practical default questions file", func(t *testing.T) {
		var out bytes.Buffer
		command := newQuestionsExampleCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"example"})

		require.NoError(t, err)
		file, err := questions.Parse(out.Bytes(), "")
		require.NoError(t, err)
		expected, err := questions.Default()
		require.NoError(t, err)
		require.Equal(t, expected, file)
		require.Len(t, file.Rules, 6)
	})
}

func TestQuestionsInitCommand(t *testing.T) {
	t.Run("writes the default questions file to the given path", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "questions.yaml")
		var out bytes.Buffer
		command := newQuestionsInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--path", path})

		require.NoError(t, err)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		file, err := questions.Parse(data, "")
		require.NoError(t, err)
		require.Len(t, file.Rules, 6)
	})

	t.Run("refuses to overwrite an existing file without --force", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "questions.yaml")
		require.NoError(t, os.WriteFile(path, []byte("version: 1\nrules: []\n"), 0o600))
		var out bytes.Buffer
		command := newQuestionsInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--path", path})

		require.ErrorContains(t, err, "already exists")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "version: 1\nrules: []\n", string(data))
	})

	t.Run("overwrites an existing file with --force", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "questions.yaml")
		require.NoError(t, os.WriteFile(path, []byte("version: 1\nrules: []\n"), 0o600))
		var out bytes.Buffer
		command := newQuestionsInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--force", "--path", path})

		require.NoError(t, err)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		file, err := questions.Parse(data, "")
		require.NoError(t, err)
		require.Len(t, file.Rules, 6)
	})

	t.Run("expands a ~ in the path flag to the home directory", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		var out bytes.Buffer
		command := newQuestionsInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--path", "~/questions.yaml"})

		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(home, "questions.yaml"))
		require.NoError(t, err)
	})

	t.Run("expands a ~ questions file in the config", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "vet", "questions"), 0o700))
		dir := t.TempDir()
		configPath := filepath.Join(dir, "vet.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("questionsFile: \"~/.config/vet/questions\"\n"), 0o600))
		var out bytes.Buffer
		command := newQuestionsInitCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"init", "--config", configPath})

		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(home, ".config", "vet", "questions", "questions.yaml"))
		require.NoError(t, err)
	})
}
