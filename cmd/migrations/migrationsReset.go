package migrations

import (
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/urfave/cli/v2"
)

func newDatabaseResetCommand() *cli.Command {
	return &cli.Command{
		Name:    "reset",
		Aliases: []string{"re"},
		Usage:   "Re-initialise the database, first delete everything & then create tables, roles, indexes, etc",
		Flags:   getOpenRegistryDatabaseCmdFlags(),
		Action:  migrationResetCmd,
	}
}

func migrationResetCmd(ctx *cli.Context) error {
	opts := parseDatabaseFlags(ctx)
	connector := getOpenRegistryDBConnectorFromCtx(opts)
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
}
