// Command ingress is the public webhook receiver. One Cloud Run service handles
// all chat platforms via path-based routing (/v1/<platform>).
//
// Per request:
//   1. Read raw body (HMAC verifiers need byte-exact input).
//   2. Look up the platform's Verifier; reject on mismatch / skew / replay.
//   3. Decode into a normalized Envelope.
//   4. Publish to sclawion.inbound with platform/kind attributes.
//
// All long-lived state (replay cache, secrets) is fetched from Firestore /
// Secret Manager — the service itself is stateless and scales to zero.
package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/slack", todo("slack"))
	mux.HandleFunc("/v1/discord", todo("discord"))
	mux.HandleFunc("/v1/gchat", todo("gchat"))
	mux.HandleFunc("/v1/whatsapp", todo("whatsapp"))
	mux.HandleFunc("/v1/imessage", todo("imessage"))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("sclawion-ingress listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func todo(platform string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, platform+" handler not implemented", http.StatusNotImplemented)
	}
}
