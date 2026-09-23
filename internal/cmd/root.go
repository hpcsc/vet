package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/hpcsc/vet/internal/backend/jev"
	"github.com/hpcsc/vet/internal/config"
	"github.com/hpcsc/vet/internal/git"
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

		color.Red(err.Error())
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
			&cli.BoolFlag{Name: "json", Usage: "print the report as JSON"},
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
		json:      cmd.Bool("json"),
		exit:      cmd.Bool("exit-code"),
	}
	return judger.run(ctx)
}

func newConfigCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
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
