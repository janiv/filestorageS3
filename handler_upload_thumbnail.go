package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse from file", err)
		return
	}
	defer file.Close()
	mediaType := header.Header.Get("Content-Type")
	vid, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Mismatch bro", err)
		return
	}
	wd, _ := os.Getwd()
	fileExtens := strings.TrimPrefix(mediaType, "image/")
	key := make([]byte, 32)
	rand.Read(key)
	keyString := base64.RawURLEncoding.EncodeToString(key)
	fileName := keyString + "." + fileExtens
	filePathName := filepath.Join(wd, cfg.assetsRoot, fileName)
	createdFile, err := os.Create(filePathName)
	_, fileErr := io.Copy(createdFile, file)
	if fileErr != nil {
		respondWithError(w, http.StatusInternalServerError, "couldnt create file", err)
	}
	dataURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	vid.ThumbnailURL = &dataURL
	dbErr := cfg.db.UpdateVideo(vid)
	if dbErr != nil {
		respondWithError(w, http.StatusInternalServerError, "db bork", err)
		return
	}
	respondWithJSON(w, http.StatusOK, vid)
}
