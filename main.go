package main

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
)

func videoServe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if r.Method != http.MethodGet {
		http.Error(w, "could not handle post request for this route", http.StatusBadRequest)
		return
	}

	id := ps.ByName("id") + ".m3u8"

	mediaFile := fmt.Sprintf("./videos/%s", id)
	fmt.Println("request")

	// if the file exists, return the file data.
	http.ServeFile(w, r, mediaFile)
	w.Header().Set("Content-Type", "application/x-mpegURL")
}

func serveHlsSegments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	mediaFile := fmt.Sprintf("./videos/" + ps.ByName("seg"))
	http.ServeFile(w, r, mediaFile)
	w.Header().Set("Content-Type", "video/MP2T")
}

func servePage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.ServeFile(w, r, "./static/index.html")
}

func uploadVideoHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("upload.gtpl")
		t.Execute(w, nil)
		return
	}

	// 100 mb max
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		http.Error(w, "cannot parse form", http.StatusInternalServerError)
		return
	}

	// parse and validate file and post parameters
	file, fileHeader, err := r.FormFile("uploadFile")
	if err != nil {
		http.Error(w, "INVALID_FILE", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileSize := fileHeader.Size
	if fileSize > (100 * 1024 * 1024) {
		http.Error(w, "FILE_TOO_BIG", http.StatusBadRequest)
		return
	}
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(w, "INVALID_FILE", http.StatusBadRequest)
		return
	}

	// check file type, detectcontenttype only needs the first 512 bytes
	detectedFileType := http.DetectContentType(fileBytes)
	if detectedFileType != "video/mp4" {
		http.Error(w, "invalid file type", http.StatusBadRequest)
		return
	}

	fileName := createRandomIdentifier(12)
	fileEndings, err := mime.ExtensionsByType(detectedFileType)
	if err != nil {
		http.Error(w, "cannot detect the filetype", http.StatusInternalServerError)
		return
	}

	newPath := filepath.Join("./videos", fileName+fileEndings[0])
	fmt.Printf("FileType: %s, File: %s\n", detectedFileType, newPath)

	newFile, err := os.Create(newPath)
	if err != nil {
		http.Error(w, "cannot write the file", http.StatusInternalServerError)
		return
	}

	defer newFile.Close()
	if _, err := newFile.Write(fileBytes); err != nil || newFile.Close() != nil {
		http.Error(w, "cannot write the file", http.StatusInternalServerError)
		return
	}

	// use the ffmpeg command to convert the file .m3u8 and .ts
	arguments := []string{"-i", newPath, "-profile:v", "baseline", "-level", "3.0", "-s",
		"640x360", "-start_number", "0", "-hls_time", "10", "-hls_list_size", "0", "-f", "hls", fileName + ".m3u8"}
	if err := exec.Command("ffmpeg", arguments...).Run(); err != nil {
		http.Error(w, "could not format file using ffmpeg", http.StatusInternalServerError)
		return
	}
}

func createRandomIdentifier(len int) string {
	b := make([]byte, len)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func main() {
	router := httprouter.New()
	router.GET("/:id/stream/", videoServe)
	router.GET("/:id/stream/:seg", serveHlsSegments)
	router.POST("/upload", uploadVideoHandler)
	router.GET("/", servePage)

	if err := http.ListenAndServe("localhost:8080", router); err != nil {
		log.Fatal(err)
	}
}
