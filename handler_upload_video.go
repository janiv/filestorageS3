package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
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
	aspectRatio, aspectErr := getVideoAspectRatio(tmpFile.Name())
	if aspectErr != nil {
		respondWithError(w, http.StatusInternalServerError, "aspect ratio broke", aspectErr)
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
	prefix := getAscpectRatioPrefix(aspectRatio)
	fileName := prefix + "/" + keyString + "." + fileExtens

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

type FFProbeData struct {
	Streams []struct {
		Index              int    `json:"index"`
		CodecName          string `json:"codec_name,omitempty"`
		CodecLongName      string `json:"codec_long_name,omitempty"`
		Profile            string `json:"profile,omitempty"`
		CodecType          string `json:"codec_type"`
		CodecTagString     string `json:"codec_tag_string"`
		CodecTag           string `json:"codec_tag"`
		Width              int    `json:"width,omitempty"`
		Height             int    `json:"height,omitempty"`
		CodedWidth         int    `json:"coded_width,omitempty"`
		CodedHeight        int    `json:"coded_height,omitempty"`
		ClosedCaptions     int    `json:"closed_captions,omitempty"`
		FilmGrain          int    `json:"film_grain,omitempty"`
		HasBFrames         int    `json:"has_b_frames,omitempty"`
		SampleAspectRatio  string `json:"sample_aspect_ratio,omitempty"`
		DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
		PixFmt             string `json:"pix_fmt,omitempty"`
		Level              int    `json:"level,omitempty"`
		ColorRange         string `json:"color_range,omitempty"`
		ColorSpace         string `json:"color_space,omitempty"`
		ColorTransfer      string `json:"color_transfer,omitempty"`
		ColorPrimaries     string `json:"color_primaries,omitempty"`
		ChromaLocation     string `json:"chroma_location,omitempty"`
		FieldOrder         string `json:"field_order,omitempty"`
		Refs               int    `json:"refs,omitempty"`
		IsAvc              string `json:"is_avc,omitempty"`
		NalLengthSize      string `json:"nal_length_size,omitempty"`
		ID                 string `json:"id"`
		RFrameRate         string `json:"r_frame_rate"`
		AvgFrameRate       string `json:"avg_frame_rate"`
		TimeBase           string `json:"time_base"`
		StartPts           int    `json:"start_pts"`
		StartTime          string `json:"start_time"`
		DurationTs         int    `json:"duration_ts"`
		Duration           string `json:"duration"`
		BitRate            string `json:"bit_rate,omitempty"`
		BitsPerRawSample   string `json:"bits_per_raw_sample,omitempty"`
		NbFrames           string `json:"nb_frames"`
		ExtradataSize      int    `json:"extradata_size"`
		Disposition        struct {
			Default         int `json:"default"`
			Dub             int `json:"dub"`
			Original        int `json:"original"`
			Comment         int `json:"comment"`
			Lyrics          int `json:"lyrics"`
			Karaoke         int `json:"karaoke"`
			Forced          int `json:"forced"`
			HearingImpaired int `json:"hearing_impaired"`
			VisualImpaired  int `json:"visual_impaired"`
			CleanEffects    int `json:"clean_effects"`
			AttachedPic     int `json:"attached_pic"`
			TimedThumbnails int `json:"timed_thumbnails"`
			NonDiegetic     int `json:"non_diegetic"`
			Captions        int `json:"captions"`
			Descriptions    int `json:"descriptions"`
			Metadata        int `json:"metadata"`
			Dependent       int `json:"dependent"`
			StillImage      int `json:"still_image"`
			Multilayer      int `json:"multilayer"`
		} `json:"disposition"`
		Tags struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			VendorID    string `json:"vendor_id"`
			Encoder     string `json:"encoder"`
			Timecode    string `json:"timecode"`
		} `json:"tags,omitempty"`
		SampleFmt      string `json:"sample_fmt,omitempty"`
		SampleRate     string `json:"sample_rate,omitempty"`
		Channels       int    `json:"channels,omitempty"`
		ChannelLayout  string `json:"channel_layout,omitempty"`
		BitsPerSample  int    `json:"bits_per_sample,omitempty"`
		InitialPadding int    `json:"initial_padding,omitempty"`
		Tags0          struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			VendorID    string `json:"vendor_id"`
		} `json:"tags0,omitempty"`
		Tags1 struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			Timecode    string `json:"timecode"`
		} `json:"tags1,omitempty"`
	} `json:"streams"`
}

func getAscpectRatioPrefix(aspectRatio string) string {
	if aspectRatio == "16:9" {
		return "landscape"
	}
	if aspectRatio == "9:16" {
		return "portrait"
	}
	return "other"
}
func getVideoAspectRatio(filePath string) (string, error) {
	args := []string{"-v", "error", "-print_format", "json", "-show_streams", filePath}
	cmd := exec.Command("ffprobe", args...)
	var bytesBuff bytes.Buffer
	cmd.Stdout = &bytesBuff
	cmd.Run()
	data := FFProbeData{}
	jsonErr := json.Unmarshal(bytesBuff.Bytes(), &data)
	if jsonErr != nil {
		return "", jsonErr
	}
	width := data.Streams[0].Width
	height := data.Streams[0].Height
	ratio := float32(width) / float32(height)
	if ratio < 0.6 && ratio > 0.5 {
		return "9:16", nil
	}
	if ratio < 1.8 && ratio > 1.68 {
		return "16:9", nil
	}
	return "other", nil

}

func processVideoForFastStart(filePath string) (string, error) {
	return "", nil
}
