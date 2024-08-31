package handlers

import (
	"io"
	"io/fs"
	"log"
	"net/http"
)

func NewStaticFilesHandler(fs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/static/style.css":
			w.Header().Set("content-type", "text/css")
			serveStaticFile(w, fs, "css/style.css")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func serveStaticFile(w http.ResponseWriter, fsys fs.FS, name string) {
	if f, err := fsys.Open(name); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else if content, err := io.ReadAll(f); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			log.Print(err)
		}
	}
}
