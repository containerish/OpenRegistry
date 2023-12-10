package config

import "regexp"

var (
	EmailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}" +
		"[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	// Reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	RegistryNSRegex = regexp.MustCompile(`[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*`)

	// Reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	RegistryManifestRefRegex = regexp.MustCompile(`[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}`)
)
