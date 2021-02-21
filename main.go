package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
)

// video contains information about a given video
type video struct {
	Name string
}

// list of all the indexed videos
var videos []video

// videoServe handles serving the .m3u8 file to the HLS-client.
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

// serveHlsSegments is required by the HLS-client and is quite straightforward
func serveHlsSegments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	mediaFile := fmt.Sprintf("./videos/" + ps.ByName("id") + "/" + ps.ByName("seg"))
	http.ServeFile(w, r, mediaFile)
	w.Header().Set("Content-Type", "video/MP2T")
}

// servePage returns the video player page, it doesnt take any parameters, since the javascript handles that.
func servePage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.ServeFile(w, r, "./static/index.html")
}

// serveUploadPage serves the upload page used in creating new videos.
func serveUploadPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.ServeFile(w, r, "./static/upload.html")
}

// uploadVideoHandler creates a video entry using a file from a file.
func uploadVideoHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// 100 mb max
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		http.Error(w, "cannot parse form", http.StatusInternalServerError)
		return
	}

	// parse and validate file and post parameters
	file, fileHeader, err := r.FormFile("uploadFile")
	if err != nil {
		http.Error(w, "invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileSize := fileHeader.Size
	if fileSize > (100 * 1024 * 1024) {
		http.Error(w, "file is too large", http.StatusBadRequest)
		return
	}

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(w, "invalid file", http.StatusBadRequest)
		return
	}

	// check file type, detectcontenttype only needs the first 512 bytes
	detectedFileType := http.DetectContentType(fileBytes)
	if detectedFileType != "video/mp4" {
		http.Error(w, "invalid file type", http.StatusBadRequest)
		return
	}

	fileName := strings.Replace(r.Form.Get("fileName"), " ", "-", -1)
	newPath := filepath.Join("./videos", fileName+".mp4")
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

	if err := createFormattedVideo(fileName + ".mp4"); err != nil {
		http.Error(w, "error formatting file", http.StatusInternalServerError)
		return
	}

	videos = append(videos, video{fileName})
	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

// createFormattedVideo creates a folder in which it stores the video segments and .m3u8 file.
// it also run the a command using ffmpeg.
func createFormattedVideo(videoName string) error {
	filename := strings.Replace(videoName, ".mp4", "", -1)

	// create a new folder for the .ts and .m3u8 files.
	if err := os.Mkdir("./videos/"+filename, 0755); err != nil {
		// no need to return a error since the process is still successful since a formatted file already exists.
		return nil
	}

	arguments := []string{"-i", ("./videos/" + videoName), "-profile:v", "baseline", "-level", "3.0", "-s",
		os.Getenv("resolution"), "-start_number", "0", "-hls_time", "10", "-hls_list_size", "0", "-f", "hls", "./videos/" +
			filename + "/index.m3u8"}

	if err := exec.Command("ffmpeg", arguments...).Run(); err != nil {
		return err
	}

	return nil
}

// initVideos formats all videos using ffmpeg and then adds those videos the global 'videos' array.
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
		if ok, err := exists(withoutMp4); ok && err != nil {
			continue
		}

		if err := createFormattedVideo(file.Name()); err != nil {
			log.Printf("error creating .m3u8 file for %s err: %s\n", file.Name(), err)
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

// exists checks if a file exists returning a boolean
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

// videosPage defines what data is displayed in the videos page
type videosPage struct {
	Videos []video
	Amount int
}

// serveVideosPage displays a html in which the user can see all the indexed videos
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
	// load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	router := httprouter.New()
	router.GET("/video/:id/stream/", videoServe)
	router.GET("/video/:id/stream/:seg", serveHlsSegments)
	router.GET("/upload", serveUploadPage)
	router.POST("/upload", uploadVideoHandler)
	router.GET("/", serveVideosPage)
	router.GET("/video/:id", servePage)

	fmt.Println("starting to format videos...")
	if err := initVideos(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("video formatting done...")

	fmt.Println("starting http server...")
	if err := http.ListenAndServe("localhost:"+os.Getenv("port"), router); err != nil {
		log.Fatal(err)
	}
}
