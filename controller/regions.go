package controller

import (
	"encoding/json"
	"net/http"
)

type RegionList struct {
	Regions []string `json:"regions"`
}

func (c *Controller) getRegionsHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")

	w.Header().Set("Content-Type", "application/json")

	enableCors(&w, r)

	json.NewEncoder(w).Encode(&RegionList{
		Regions: c.spawner.Regions(),
	})
}
