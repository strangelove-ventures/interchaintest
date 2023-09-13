package util

import (
	"log"
	"net/http"
)

func Write(w http.ResponseWriter, bz []byte) {
	if _, err := w.Write(bz); err != nil {
		log.Default().Println(err)
	}
}

func WriteError(w http.ResponseWriter, err error) {
	Write(w, []byte(`{"error": "`+err.Error()+`"}`))
}
