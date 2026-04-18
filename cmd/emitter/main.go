// Command emitter receives Pub/Sub push messages from sclawion.outbound and
// posts them back to the originating chat platform. One binary, --platform flag
// selects which connector's Encoder to use; deploy as four Cloud Run services.
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	platform := flag.String("platform", "", "one of: slack|discord|gchat|whatsapp|imessage")
	flag.Parse()
	if *platform == "" {
		log.Fatal("--platform is required")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, *platform+" emitter not implemented", http.StatusNotImplemented)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("sclawion-emitter[%s] listening on :8080", *platform)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
