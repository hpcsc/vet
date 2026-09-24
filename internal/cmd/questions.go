package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hpcsc/vet/internal/config"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/urfave/cli/v3"
)

func newQuestionsCommand() *cli.Command {
	return &cli.Command{
		Name:     "questions",
		Usage:    "print an example questions file, or write the default one",
		Commands: []*cli.Command{newQuestionsExampleCommand(), newQuestionsInitCommand()},
	}
}

func newQuestionsExampleCommand() *cli.Command {
	return &cli.Command{
		Name:  "example",
		Usage: "print an example questions file, one rule of each type",
		Action: func(_ context.Context, cmd *cli.Command) error {
			file, err := questions.Marshal(exampleQuestionsFile())
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.Root().Writer, string(file))
			return err
		},
	}
}

func newQuestionsInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "write the default questions file where vet looks for it, unless --path gives another place",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Usage: "overwrite a questions file that already exists"},
			&cli.StringFlag{Name: "path", Value: "", Usage: "the path of the questions file to write, instead of the resolved one"},
			&cli.StringFlag{Name: "config", Value: "", Usage: "the path of the config file (default ~/.config/vet/config.yaml, then a .vet.yaml in the repository root)"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			path := cmd.String("path")
			if path != "" {
				return writeDefaultQuestions(cmd, expandHome(path))
			}
			repoDir, err := os.Getwd()
			if err != nil {
				return err
			}
			configFlag := cmd.String("config")
			if configFlag == "" {
				configFlag = cmd.Root().String("config")
			}
			cfg, err := resolveConfig(context.Background(), repoDir, configFlag, os.Getenv)
			if err != nil {
				return err
			}
			configQuestions := expandHome(cfg.Resolve().QuestionsFile)
			if configQuestions == "" {
				return writeDefaultQuestions(cmd, config.QuestionsPath(os.Getenv))
			}
			info, err := os.Stat(configQuestions)
			if err == nil && info.IsDir() {
				return writeDefaultQuestions(cmd, filepath.Join(configQuestions, "questions.yaml"))
			}
			return writeDefaultQuestions(cmd, configQuestions)
		},
	}
}

func writeDefaultQuestions(cmd *cli.Command, path string) error {
	if _, err := os.Stat(path); err == nil && !cmd.Bool("force") {
		return fmt.Errorf("questions file %s already exists (pass --force to overwrite)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := questions.Default()
	if err != nil {
		return err
	}
	text, err := questions.Marshal(file)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.Root().Writer, "Wrote the default questions file to %s.\n", path)
	return err
}

func exampleQuestionsFile() questions.File {
	return questions.File{
		Version: 1,
		Context: "These guidelines apply to every rule below.\n- The codebase logs with slog, never to stdout.",
		Rules: []questions.Rule{
			{
				ID:           "no-flag-field",
				Instructions: "The change adds a flag field to the request struct.",
				Type:         questions.Noul,
				NoulLimit:    pointerTo(0.5),
			},
			{
				ID:           "database-migration",
				Instructions: "Which option describes the change best?",
				Type:         questions.Choice,
				Choices: map[string]string{
					"no-db":    "The change does not touch the database.",
					"uses-db":  "The change reads or writes the database.",
					"migrates": "The change alters the schema.",
				},
				ViolatesWhen: "migrates",
			},
			{
				ID:           "log-guideline",
				Instructions: "Rate how the change follows the logging guideline.",
				Type:         questions.Score,
				Scores:       []string{"first", "second", "third"},
				ScoreLimit:   pointerTo(2),
			},
		},
	}
}

func pointerTo[T any](v T) *T { return &v }
