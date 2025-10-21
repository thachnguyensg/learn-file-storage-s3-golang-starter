package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func (cfg apiConfig) assetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) assetURL(assetPath string) string {
	return fmt.Sprintf("%s/%s", "/assets", assetPath)
}

func getAssetPath(videoId uuid.UUID, mediaType string) string {
	return fmt.Sprintf("%s%s", videoId.String(), mediaTypeToFileExt(mediaType))
}

func mediaTypeToFileExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}
