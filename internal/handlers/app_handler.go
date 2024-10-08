package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
)

func NewAppHandler(fs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("Inside main app handler")
		templateChain := []string{
			"base.html.tmpl",
			"app.html.tmpl",
		}
		t := template.New(templateChain[0])
		if parsedTemplates, err := t.ParseFS(fs, templateChain...); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(fmt.Sprintf("Error parsing template chain: %s", err))); err != nil {
				log.Print(err)
			}
		} else {
			var b bytes.Buffer
			data := struct{}{}
			if err := parsedTemplates.Execute(&b, data); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				if _, err := w.Write([]byte(fmt.Sprintf("Error executing template: %s", err))); err != nil {
					log.Print(err)
				}
			} else {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write(b.Bytes()); err != nil {
					log.Print(err)
				}
			}
		}
	}
}
