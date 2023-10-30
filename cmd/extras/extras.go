package extras

import (
	"github.com/urfave/cli/v2"
)

// const CategoryExtras = "extras"

func NewExtrasCommand() *cli.Command {
	return &cli.Command{
		Name:        "extras",
		Aliases:     []string{"e", "ex", "ext"},
		Usage:       "OpenRegistry extras digest --input='hello-world' --type='sha256'",
		Description: "Generate SHA256, SHA512 or Canonical digest using the OCI go-digest package",
		UsageText:   "",
		// Category:    CategoryExtras,
		Action: nil,
		Subcommands: []*cli.Command{
			newDigestCommand(),
		},
	}
}
