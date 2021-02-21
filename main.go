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
	"strings"

	"github.com/julienschmidt/httprouter"
)

type video struct {
	Name string
}

var videos []video

func videoServe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if r.Method != http.MethodGet {
		http.Error(w, "could not handle post request for this route", http.StatusBadRequest)
		return
	}

	mediaFile := fmt.Sprintf("./videos/%s/index.m3u8", ps.ByName("id"))

	// if the file exists, return the file data.
	http.ServeFile(w, r, mediaFile)
	w.Header().Set("Content-Type", "application/x-mpegURL")
}

func serveHlsSegments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	mediaFile := fmt.Sprintf("./videos/" + ps.ByName("id") + "/" + ps.ByName("seg"))
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

	// we still store the .mp4 file just in case
	if _, err := newFile.Write(fileBytes); err != nil || newFile.Close() != nil {
		http.Error(w, "cannot write the file", http.StatusInternalServerError)
		return
	}

	createFormattedVideo(fileName + fileEndings[0])
}

func createFormattedVideo(videoName string) error {
	filename := strings.Replace(videoName, ".mp4", "", -1)

	// create a new folder for the .ts and .m3u8 files.
	if err := os.Mkdir("./videos/"+filename, 0755); err != nil {
		return err
	}

	arguments := []string{"-i", ("./videos/" + videoName), "-profile:v", "baseline", "-level", "3.0", "-s",
		"640x360", "-start_number", "0", "-hls_time", "10", "-hls_list_size", "0", "-f", "hls", "./videos/" +
			filename + "/index.m3u8"}

	if err := exec.Command("ffmpeg", arguments...).Run(); err != nil {
		return err
	}

	return nil
}

func initVideos() error {
	files, err := ioutil.ReadDir("./videos")
	if err != nil {
		return err
	}

	for _, file := range files {
		// ignore every file other than mp4
		if !strings.HasSuffix(file.Name(), ".mp4") || file.IsDir() {
			continue
		}

		// check that the file doesn't have a formatted video
		withoutMp4 := strings.Replace(file.Name(), ".mp4", "", -1)
		if ok, _ := exists(withoutMp4); ok {
			continue
		}

		if err := createFormattedVideo(file.Name()); err != nil {
			log.Printf("error creating .m3u8 file for %s\n", file.Name())
			continue
		}
	}

	filesNew, err := ioutil.ReadDir("./videos")
	if err != nil {
		return err
	}

	for _, file := range filesNew {
		if !file.IsDir() {
			continue
		}

		videos = append(videos, video{file.Name()})
	}

	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func createRandomIdentifier(len int) string {
	b := make([]byte, len)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

type videosPage struct {
	Videos []video
	Amount int
}

func serveVideosPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	pageData := &videosPage{
		Videos: videos,
		Amount: len(videos),
	}

	tmpl := template.Must(template.ParseFiles("./static/videos.html"))
	if err := tmpl.Execute(w, pageData); err != nil {
		http.Error(w, "error creating template", http.StatusInternalServerError)
		return
	}
}

func main() {
	router := httprouter.New()
	router.GET("/video/:id/stream/", videoServe)
	router.GET("/video/:id/stream/:seg", serveHlsSegments)
	router.POST("/upload", uploadVideoHandler)
	router.GET("/", serveVideosPage)
	router.GET("/video/:id", servePage)

	fmt.Println("starting to format videos...")
	if err := initVideos(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("video formatting done...")

	fmt.Println("starting http server...")
	if err := http.ListenAndServe("localhost:8080", router); err != nil {
		log.Fatal(err)
	}
}
