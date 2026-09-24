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

func TestQuestionsCommand(t *testing.T) {
	t.Run("prints a valid example questions file with all three rule types", func(t *testing.T) {
		var out bytes.Buffer
		command := newQuestionsCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"questions"})

		require.NoError(t, err)
		file, err := questions.Parse(out.Bytes(), "")
		require.NoError(t, err)
		require.Len(t, file.Rules, 3)
		require.Equal(t, questions.Noul, file.Rules[0].Type)
		require.Equal(t, questions.Choice, file.Rules[1].Type)
		require.Equal(t, questions.Score, file.Rules[2].Type)
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
		require.Len(t, file.Rules, 3)
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
		require.Len(t, file.Rules, 3)
	})
}
