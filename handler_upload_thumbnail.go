package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"crypto/rand"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	maxMemory := 10 << 20

	r.ParseMultipartForm(int64(maxMemory))

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "thumbnail not present", err)
		return
	}

	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || (mediaType != "image/png" && mediaType != "image/jpeg") {
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnail file type", err)
		return
	}

	metaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not present", err)
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

	if userID != metaData.UserID {
		respondWithError(w, http.StatusUnauthorized, "Current user is not the owner of the video", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	ext := ".png"
	if mediaType == "image/jpeg" {
		ext = ".jpg"
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating random file name", err)
		return
	}

	fileName := base64.RawURLEncoding.EncodeToString(key)

	filePath := filepath.Join(cfg.assetsRoot, fileName+ext)

	dst, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}

	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:8091/assets/%s%s", fileName, ext)
	metaData.ThumbnailURL = &thumbnailURL

	if err = cfg.db.UpdateVideo(metaData); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metaData)
}
