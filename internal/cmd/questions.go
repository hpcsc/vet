package cmd

import (
	"context"
	"fmt"

	"github.com/hpcsc/vet/internal/questions"
	"github.com/urfave/cli/v3"
)

func newQuestionsCommand() *cli.Command {
	return &cli.Command{
		Name:  "questions",
		Usage: "print the example questions file, one rule of each type",
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

// exampleQuestionsFile is the printed example: a rule of each type, with the
// choices the log guideline and the database migration share.
func exampleQuestionsFile() questions.File {
	return questions.File{
		Version: 1,
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
