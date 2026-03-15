package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ImageMetadata struct {
	DefaultUser *User   `json:"default_user,omitempty"`
	Groups      []Group `json:"groups,omitempty"`
}

func LoadImageMetadata(imagePath string) (*ImageMetadata, error) {
	metaPath := filepath.Join(imagePath, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return &ImageMetadata{}, nil
	}

	var meta ImageMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse meta.json: %v", err)
	}

	return &meta, nil
}
