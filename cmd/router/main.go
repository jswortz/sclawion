// Command router subscribes (Pub/Sub push) to sclawion.inbound and dispatches
// to Scion. This is the only service that talks to Scion's Hub.
//
// Per request:
//   1. Validate OIDC push token (audience = router URL).
//   2. Decode Envelope.
//   3. Idempotency check: correlation.MarkProcessed(event.ID).
//   4. Look up existing agent for ConversationID; if none, scion.Dispatch.
//   5. Else scion.Message to the running agent.
package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "router not implemented", http.StatusNotImplemented)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("sclawion-router listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
