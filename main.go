package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func videoServeHelper(w http.ResponseWriter, r *http.Request, id string) {
	// format the file name
	mediaFile := fmt.Sprintf("videos/%s", id)

	// check if the file exists
	if _, err := os.Stat(mediaFile); !os.IsNotExist(err) {
		http.Error(w, "video does not exist", http.StatusNotFound)
		return
	}

	// if the file exists, return the file data.
	http.ServeFile(w, r, mediaFile)
	w.Header().Set("Content-Type", "application/x-mpegURL")
}

func videoServe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "could not handle post request for this route", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	ids, ok := query["id"]
	if !ok || len(ids) == 0 {
		http.Error(w, "video id was not provided", http.StatusNotFound)
		return
	}

	id := ids[0] + ".m3u8"
	videoServeHelper(w, r, id)
}

func servePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func main() {
	http.HandleFunc("/stream", videoServe)
	http.HandleFunc("/", servePage)

	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal(err)
	}
}
