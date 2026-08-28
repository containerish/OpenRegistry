package types

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func NewUUID() (uuid.UUID, error) {
	return uuid.NewRandom()
}

func CreateIdentifier() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return id.String(), nil
}

func CreateUploadTrackingIdentifier(uploadID, layerIdentifier string) string {
	return uploadID + ":" + layerIdentifier
}

// GetLayerIdentifierFromTrakcingID splits the ID by ":" and returns the PREFIX or [0] indexed value from the trackingID
func GetUploadIDFromTrakcingID(trackingID string) string {
	return strings.Split(trackingID, ":")[0]
}

// GetLayerIdentifierFromTrakcingID splits the ID by ":" and returns the SUFFIX or [1] indexed value from the trackingID
func GetLayerIdentifierFromTrakcingID(trackingID string) string {
	return strings.Split(trackingID, ":")[1]
}

func GetLayerIdentifier(identifier string) string {
	return fmt.Sprintf("layers/%s", identifier)
}

func GetManifestIdentifier(namespace, reference string) string {
	return fmt.Sprintf("%s/manifests/%s", namespace, reference)
}
