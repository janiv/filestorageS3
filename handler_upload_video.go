package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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
	vid, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "where vidya", err)
		return
	}
	if userID != vid.UserID {
		respondWithError(w, http.StatusUnauthorized, "Not yours", err)
		return
	}
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "parse failed", err)
		return
	}
	defer file.Close()
	mediaType, params, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "header broke", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "need video", errors.New("must be video"))
		return
	}
	fmt.Println(params)

	tmpFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not save", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	writeAmt, fileErr := io.Copy(tmpFile, file)
	if fileErr != nil {
		respondWithError(w, http.StatusInternalServerError, "copy fail", err)
		return
	}
	fmt.Printf("Wrote %v from file to file\n", writeAmt)
	_, rewErr := tmpFile.Seek(0, io.SeekStart)
	if rewErr != nil {
		respondWithError(w, http.StatusInternalServerError, "could not rewind", err)
		return
	}
	fileExtens := strings.TrimPrefix(mediaType, "video/")
	key := make([]byte, 32)
	rand.Read(key)
	keyString := base64.RawURLEncoding.EncodeToString(key)
	fileName := keyString + "." + fileExtens

	fmt.Printf("s3Bucket: %s\nkey: %s\ncontent-type:%s", cfg.s3Bucket, fileName, mediaType)
	_, upErr := cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(fileName),
		Body:        tmpFile,
		ContentType: aws.String(mediaType),
	})
	if upErr != nil {
		respondWithError(w, http.StatusInternalServerError, "could not put into bucket", err)
		return
	}
	vidURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileName)
	vid.VideoURL = &vidURL
	dbErr := cfg.db.UpdateVideo(vid)
	if dbErr != nil {
		respondWithError(w, http.StatusInternalServerError, "db bork", err)
		return
	}
	respondWithJSON(w, http.StatusOK, vid)
}
