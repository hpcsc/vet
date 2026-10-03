package cmd

import (
	"context"
	"fmt"

	"github.com/hpcsc/vet/internal/profile"
	"github.com/urfave/cli/v3"
)

// newProfileCommand reads saved reports and prints how near each rule's answers
// sit to its limit, so a rule author can see which rules are guessing.
func newProfileCommand() *cli.Command {
	return &cli.Command{
		Name:  "profile",
		Usage: "report how near each rule's answers sit to its limit",
		Flags: []cli.Flag{
			&cli.Float64Flag{
				Name:  "max-near-limit",
				Value: 0,
				Usage: "exit non-zero when any noul rule answers near its limit more often than this share, 0 to 1",
			},
		},
		Action: profileAction,
	}
}

func profileAction(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return fmt.Errorf("profile needs the directory a replay saved its reports to")
	}
	maxShare := cmd.Float("max-near-limit")
	if maxShare < 0 || maxShare > 1 {
		return fmt.Errorf("--max-near-limit is a share, so it must be 0 to 1, got %v", maxShare)
	}
	reports, err := profile.Reports(args[0])
	if err != nil {
		return err
	}
	stats := profile.Summarize(reports)
	profile.Write(cmd.Root().Writer, stats)
	return profile.Gate(stats, maxShare)
}
