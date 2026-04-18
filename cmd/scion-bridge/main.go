// Command scion-bridge converts Scion agent state into outbound Envelopes.
//
// v1: poll/tail Scion's WebSocket log endpoint per active agent and emit
// agent.reply / agent.completed / agent.failed events to sclawion.outbound.
// v2: register as a Scion plugin that receives status callbacks directly.
package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("sclawion-scion-bridge listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
