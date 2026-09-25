package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hpcsc/vet/internal/backend/jev"
	"github.com/hpcsc/vet/internal/config"
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/style"
	"github.com/hpcsc/vet/internal/version"
	"github.com/urfave/cli/v3"
)

const releaseRepository = "hpcsc/vet"

func Run(ctx context.Context) int {
	if err := newCommand().Run(ctx, os.Args); err != nil {
		var code exitCode
		if errors.As(err, &code) {
			return int(code)
		}

		fmt.Println(style.Fail(err.Error()))
		return 2
	}

	return 0
}

func newCommand() *cli.Command {
	return &cli.Command{
		Name:                  "vet",
		Version:               version.Current(),
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "base", Usage: "the git ref to compare against, instead of the detected base"},
			&cli.StringFlag{Name: "questions", Value: "", Usage: "the path of the questions file or directory (default questions.yaml in the working directory)"},
			&cli.StringFlag{
				Name:      "output",
				Aliases:   []string{"o"},
				Value:     string(outputModeText),
				Usage:     "output mode: text, json, or tui",
				Validator: func(value string) error { _, err := outputModeOf(value); return err },
			},
			&cli.BoolFlag{Name: "all", Usage: "show passing and failing rules"},
			&cli.BoolFlag{Name: "exit-code", Usage: "exit 1 when the change violates a rule"},
			&cli.StringFlag{Name: "api-url", Value: "", Usage: "the System One endpoint (default https://api.typesafe.ai/v1/systemone, or TYPESAFE_API_URL)"},
			&cli.StringFlag{Name: "model", Value: "jev-latest", Usage: "the model to judge with"},
			&cli.StringFlag{Name: "api-key", Value: "", Usage: "the System One API key (default TYPESAFE_API_KEY, then the api-key-command in the config)"},
			&cli.StringFlag{Name: "config", Value: "", Usage: "the path of the config file (default ~/.config/vet/config.yaml, then a .vet.yaml in the repository root)"},
		},
		Action: judgeAction,
		Commands: []*cli.Command{
			newConfigCommand(),
			newQuestionsCommand(),
			newVersionCommand(),
			newUpdateCommand(),
		},
	}
}

func judgeAction(ctx context.Context, cmd *cli.Command) error {
	repoDir, err := os.Getwd()
	if err != nil {
		return err
	}
	output, err := outputModeOf(cmd.String("output"))
	if err != nil {
		return err
	}
	if output == outputModeTUI && !tuiTerminalAvailable(cmd.Root().Writer) {
		return errors.New("tui output requires an interactive terminal")
	}
	cfg, err := resolveConfig(ctx, repoDir, cmd.String("config"), os.Getenv)
	if err != nil {
		return err
	}
	resolved := cfg.Resolve()
	apiURL := apiURLOf(cmd.String("api-url"), resolved.APIURL, os.Getenv)
	modelFlag := ""
	if cmd.IsSet("model") {
		modelFlag = cmd.String("model")
	}
	model := modelOf(modelFlag, resolved.Model)
	apiKey, err := apiKeyOf(cmd.String("api-key"), resolved.APIKeyCommand, os.Getenv, runKeyCommand)
	if err != nil {
		return err
	}
	questions, err := questionsPathOf(cmd.String("questions"), resolved.QuestionsFile, repoDir)
	if err != nil {
		return err
	}
	judger := judge{
		out:       cmd.Root().Writer,
		repo:      git.New(repoDir),
		backend:   jev.NewClient(&http.Client{Timeout: 2 * time.Minute}, apiURL, model, apiKey),
		questions: questions,
		base:      cmd.String("base"),
		output:    output,
		all:       cmd.Bool("all"),
		exit:      cmd.Bool("exit-code"),
	}
	return judger.run(ctx)
}

func newConfigCommand() *cli.Command {
	return &cli.Command{
		Name:     "config",
		Usage:    "print an example config file, or write the default one",
		Commands: []*cli.Command{newConfigExampleCommand(), newConfigInitCommand()},
	}
}

func newConfigExampleCommand() *cli.Command {
	return &cli.Command{
		Name:  "example",
		Usage: "print the default config file",
		Action: func(_ context.Context, cmd *cli.Command) error {
			text, err := config.Default().YAML()
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.Root().Writer, string(text))
			return err
		},
	}
}

func newConfigInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "write the default config file where vet looks for it, unless --path gives another place",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Usage: "overwrite a config file that already exists"},
			&cli.StringFlag{Name: "path", Value: "", Usage: "the path of the config file to write, instead of the resolved one"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return writeDefaultConfig(cmd, cmd.String("path"))
		},
	}
}

func writeDefaultConfig(cmd *cli.Command, path string) error {
	if path == "" {
		path = config.Path(os.Getenv)
	}
	if _, err := os.Stat(path); err == nil && !cmd.Bool("force") {
		return fmt.Errorf("config file %s already exists (pass --force to overwrite)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	text, err := config.Default().YAML()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.Root().Writer, "Wrote the default config file to %s.\n", path)
	return err
}

func newVersionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "print the tag vet was built from, or its commit when it has no tag",
		Action: func(_ context.Context, cmd *cli.Command) error {
			_, err := fmt.Fprintln(cmd.Root().Writer, version.Current())
			return err
		},
	}
}
