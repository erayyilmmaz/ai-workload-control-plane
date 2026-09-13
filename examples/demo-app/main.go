// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"log"
	"net/http"
)

var version = "dev"

func main() {
	log.Fatal(http.ListenAndServe(":8080", newHandler(version)))
}

func newHandler(release string) http.Handler {
	mux := http.NewServeMux()
	for _, path := range []string{"/health", "/ready"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprintln(w, "ok")
		})
	}
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, release)
	})
	return mux
}
