package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

var allowVideoMimeType = map[string]struct{}{
	"video/mp4": {},
}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	maxMemory := 1 << 30 // 1 GB
	http.MaxBytesReader(w, r.Body, int64(maxMemory))

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}

	if video.UserID != userID {
		vErr := fmt.Errorf("Unauthorized upload attempt, owner: %s; user: %s", video.UserID, userID)
		respondWithError(w, http.StatusUnauthorized, "You don't own this video", vErr)
		return
	}

	err = r.ParseMultipartForm(int64(maxMemory))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video file from form", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	mimeType, _, err := mime.ParseMediaType(mediaType)
	if _, ok := allowVideoMimeType[mimeType]; !ok {
		respondWithError(w, http.StatusBadRequest, "Unsupported media type", fmt.Errorf("media type %s is not allowed", mimeType))
		return
	}

	tmpFileName, err := getAssetPath(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
		return
	}
	tmpFile, err := os.CreateTemp("", tmpFileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save video to temp file", err)
		return
	}
	fmt.Println("Saved video to temp file:", tmpFile.Name())

	aspectRatio, err := getVideoAspectRatio(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio", err)
		return
	}
	prefix := getVideoPrefix(aspectRatio)
	key := fmt.Sprintf("%s/%s", prefix, tmpFileName)

	// _, err = tmpFile.Seek(0, io.SeekStart)
	// if err != nil {
	// 	respondWithError(w, http.StatusInternalServerError, "Couldn't seek to beginning of temp file", err)
	// 	return
	// }
	processFilePath, err := processVideoForFastStart(tmpFile.Name())
	processFile, err := os.Open(processFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open processed video file", err)
		return
	}
	defer os.Remove(processFile.Name())
	defer processFile.Close()

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        processFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video to S3", err)
		return
	}

	videoUrl := cfg.assetObjectURL(key)
	video.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video record", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
