package cmd

import (
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/urfave/cli/v2"
)

func newDatabaseResetCommand() *cli.Command {
	return &cli.Command{
		Name:    "reset",
		Aliases: []string{"re"},
		Usage:   "Re-initialise the database, first delete everything & then create tables, roles, indexes, etc",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "openregistry-db-dsn",
				Value:    "postgres://localhost:5432/open_registry",
				Required: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			opts := parseDatabaseFlags(ctx)
			connector := getDBConnectorFromCtx(false, opts)
			db := getOpenRegistryDB(connector)

			return db.ResetModel(
				ctx.Context,
				&types.ContainerImageRepository{},
				&types.ContainerImageLayer{},
				&types.ImageManifest{},
				&types.User{},
				&types.Session{},
				&types.WebauthnSession{},
				&types.WebauthnCredential{},
				&types.Email{},
			)
		},
	}
}
