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
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/version"
	"github.com/urfave/cli/v3"
)

const releaseRepository = "hpcsc/vet"

const defaultAPIURL = "https://api.typesafe.ai/v1/systemone"

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
			&cli.StringFlag{Name: "questions", Value: "questions.yaml", Usage: "the path of the questions file"},
			&cli.BoolFlag{Name: "json", Usage: "print the report as JSON"},
			&cli.BoolFlag{Name: "exit-code", Usage: "exit 1 when the change violates a rule"},
			&cli.StringFlag{Name: "api-url", Value: "", Usage: "the System One endpoint (default https://api.typesafe.ai/v1/systemone, or TYPESAFE_API_URL)"},
			&cli.StringFlag{Name: "model", Value: "jev-latest", Usage: "the model to judge with"},
			&cli.StringFlag{Name: "api-key", Value: "", Usage: "the System One API key (default TYPESAFE_API_KEY)"},
		},
		Action: judgeAction,
		Commands: []*cli.Command{
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
	apiURL := cmd.String("api-url")
	if apiURL == "" {
		apiURL = os.Getenv("TYPESAFE_API_URL")
	}
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	apiKey := cmd.String("api-key")
	if apiKey == "" {
		apiKey = os.Getenv("TYPESAFE_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("no API key: pass --api-key, or set TYPESAFE_API_KEY")
	}
	judger := judge{
		out:       cmd.Root().Writer,
		repo:      git.New(repoDir),
		backend:   jev.NewClient(&http.Client{Timeout: 2 * time.Minute}, apiURL, cmd.String("model"), apiKey),
		questions: cmd.String("questions"),
		base:      cmd.String("base"),
		json:      cmd.Bool("json"),
		exit:      cmd.Bool("exit-code"),
	}
	return judger.run(ctx)
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
