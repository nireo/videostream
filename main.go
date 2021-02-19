package main

import (
	"fmt"
	"log"
	"net/http"

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

func main() {
	router := httprouter.New()
	router.GET("/:id/stream/", videoServe)
	router.GET("/:id/stream/:seg", serveHlsSegments)
	router.GET("/", servePage)

	if err := http.ListenAndServe("localhost:8080", router); err != nil {
		log.Fatal(err)
	}
}
