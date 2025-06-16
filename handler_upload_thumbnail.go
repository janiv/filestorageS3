package main

import (
	"fmt"
	"io"
	"net/http"

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
	byteSlice, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read file", err)
		return
	}
	vid, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Mismatch bro", err)
		return
	}

	tnail := thumbnail{
		data:      byteSlice,
		mediaType: mediaType,
	}
	videoThumbnails[vid.ID] = tnail
	fmt.Printf("%s", vid.ID)
	newURL := "http://localhost:8091/api/thumbnails/" + vid.ID.String()
	vid.ThumbnailURL = &newURL
	fmt.Printf(*vid.ThumbnailURL)
	dbErr := cfg.db.UpdateVideo(vid)
	if dbErr != nil {
		respondWithError(w, http.StatusInternalServerError, "db bork", err)
		return
	}
	respondWithJSON(w, http.StatusOK, vid)
}
