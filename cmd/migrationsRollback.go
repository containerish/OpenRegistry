package cmd

import "github.com/urfave/cli/v2"

func newMigrationsRollbackCommand() *cli.Command {
	return &cli.Command{
		Name:  "rollback",
		Usage: "Rollback the latest migrations",
		Action: func(ctx *cli.Context) error {
			return nil
		},
	}
}
