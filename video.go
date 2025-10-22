package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

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

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	req, err := presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (c *apiConfig) dbVideoToSigedVideo(video database.Video) (database.Video, error) {
	parts := strings.Split(*video.VideoURL, ",")
	if len(parts) != 2 {
		return video, errors.New("invalid video URL format")
	}

	bucket := parts[0]
	key := parts[1]
	presignURL, err := generatePresignedURL(c.s3Client, bucket, key, 15*time.Minute)
	if err != nil {
		return video, err
	}
	video.VideoURL = &presignURL
	return video, nil
}
