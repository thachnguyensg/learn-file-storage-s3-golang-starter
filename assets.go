package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func (cfg apiConfig) assetObjectURL(assetPath string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, assetPath)
}

func getAssetPath(mediaType string) (string, error) {
	randBuf := make([]byte, 32)
	_, err := rand.Read(randBuf)
	if err != nil {
		return "", err
	}

	fileName := base64.RawURLEncoding.EncodeToString(randBuf)
	ext := mediaTypeToFileExt(mediaType)
	fmt.Println("media type:", mediaType)
	fmt.Println("ext:", ext)
	fmt.Println("Generated temp file name:", fileName)
	return fmt.Sprintf("%s%s", fileName, ext), nil
}

func mediaTypeToFileExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	fmt.Println("Media type parts:", parts)
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getVideoAspectRatio(filePath string) (string, error) {
	var ffprobeOutput struct {
		Streams []struct {
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			CodecType   string `json:"codec_type"`
			AspectRatio string `json:"display_aspect_ratio"`
		} `json:"streams"`
	}

	buf := bytes.Buffer{}
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	decoder := json.NewDecoder(&buf)
	err = decoder.Decode(&ffprobeOutput)
	if err != nil {
		return "", err
	}

	if len(ffprobeOutput.Streams) == 0 {
		return "", errors.New("no streams found in video file")
	}

	videoStream := ffprobeOutput.Streams[0]

	return videoStream.AspectRatio, nil
}

func getVideoPrefix(aspectRatio string) string {
	switch aspectRatio {
	case "16:9":
		return "landscape"
	case "9:16":
		return "portrait"
	default:
		return "other"
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	log.Println("Processing video for fast start:", filePath)
	processFile := fmt.Sprintf("%s.processing", filePath)
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", processFile)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	log.Println("Processed video saved to:", processFile)
	return processFile, nil
}
