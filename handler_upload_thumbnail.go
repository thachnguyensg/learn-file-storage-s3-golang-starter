package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

var allowMimeType = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
}

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	maxMemory := 10 << 20 // 10 MB
	err = r.ParseMultipartForm(int64(maxMemory))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail from form", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	mimeType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if _, ok := allowMimeType[mimeType]; !ok {
		respondWithError(w, http.StatusBadRequest, "Unsupported media type", fmt.Errorf("media type %s is not allowed", mimeType))
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}
	if video.UserID != userID {
		log.Println("video user ID:", video.UserID, "request user ID:", userID)
		respondWithError(w, http.StatusUnauthorized, "You don't own this video", nil)
		return
	}

	fileName, err := getAssetPath(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate asset path", err)
		return
	}

	filePath := cfg.assetDiskPath(fileName)
	tnFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file", err)
		return
	}
	defer tnFile.Close()

	_, err = io.Copy(tnFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save thumbnail file", err)
		return
	}

	fileUrl := cfg.assetURL(fileName)
	video.ThumbnailURL = &fileUrl
	cfg.db.UpdateVideo(video)

	respondWithJSON(w, http.StatusOK, video)
}
