package extras

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	oci_digest "github.com/opencontainers/go-digest"
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

	// inputBz := bytes.TrimSpace([]byte(input))

	manifestContent, err := json.Marshal([]byte(input))
	if err != nil {
		return fmt.Errorf(color.RedString("generateDigest: ERR_MARSHAL_INDENT: %s", err))
	}

	var inputDigest oci_digest.Digest
	switch digestType {
	case "SHA256":
		inputDigest = oci_digest.SHA256.FromBytes(manifestContent)
	case "SHA512":
		inputDigest = oci_digest.SHA512.FromBytes(manifestContent)
	default:
		inputDigest = oci_digest.Canonical.FromBytes(manifestContent)
	}

	color.Green("input=\n%s\ndigest=%s", manifestContent, inputDigest)
	return nil
}
