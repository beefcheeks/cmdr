package daemon

import (
	"encoding/json"
	"net/http"
)

// handleNotify bridges CLI commands to SSE — the CLI inserts into the DB
// directly, then POSTs here so the frontend gets a real-time update.
func handleNotify(bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Event string `json:"event"`
			Data  any    `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Event == "" {
			http.Error(w, "need event", http.StatusBadRequest)
			return
		}
		bus.Publish(Event{Type: body.Event, Data: body.Data})
		w.WriteHeader(http.StatusNoContent)
	}
}
