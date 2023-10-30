package extras

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/opencontainers/go-digest"
	"github.com/urfave/cli/v2"
)

func newDigestCommand() *cli.Command {
	return &cli.Command{
		Name:        "digest",
		Aliases:     []string{"d"},
		UsageText:   "",
		Usage:       "OpenRegistry extras digest --input='hello-world' --type='sha256'",
		Description: "Generate SHA256, SHA512 or CANONICAL digest using the OCI go-digest package",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Required: true,
			},
			&cli.StringFlag{
				Name:        "type",
				DefaultText: "CANONICAL",
				Required:    false,
				Value:       "CANONICAL",
				Usage:       "--type SHA256/SHA512/CANONICAL",
			},
		},
		Action: generateDigest,
	}
}

func generateDigest(ctx *cli.Context) error {
	input := ctx.String("input")
	if input == "" {
		return fmt.Errorf(color.RedString("input is empty"))
	}

	digestType := strings.ToUpper(ctx.String("type"))
	if digestType == "" {
		digestType = "CANONICAL"
	}

	manifestContent, err := json.MarshalIndent(input, "", "\t")
	if err != nil {
		return fmt.Errorf(color.RedString("generateDigest: ERR_MARSHAL_INDENT: %s", err))
	}

	var inputDigest digest.Digest
	switch digestType {
	case "SHA256":
		inputDigest = digest.SHA256.FromBytes(manifestContent)
	case "SHA512":
		inputDigest = digest.SHA512.FromBytes(manifestContent)
	default:
		inputDigest = digest.Canonical.FromBytes(manifestContent)
	}

	color.Green("%s", inputDigest)
	return nil
}
