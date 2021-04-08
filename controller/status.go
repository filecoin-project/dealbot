package controller

import (
	"encoding/json"
	"net/http"
)

type TaskStatus struct {
	UUID    string      `json:"uuid"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (c *Controller) reportStatusHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")

	var status TaskStatus
	err := json.NewDecoder(r.Body).Decode(&status)
	if err != nil {
		log.Debug(err)
		w.WriteHeader(http.StatusBadRequest)
	}

	log.Infow(status.Type, "UUID", status.UUID, "payload", status.Payload)

	w.WriteHeader(http.StatusOK)
}
