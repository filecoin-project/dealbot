package controller

import (
	"encoding/json"
	"net/http"

	"github.com/filecoin-project/dealbot/controller/client"
)

func (c *Controller) getTasksHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(State)
}

func (c *Controller) updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))

	logger.Debugw("handle request", "command", "update task")
	defer logger.Debugw("request handled", "command", "update task")

	w.Header().Set("Content-Type", "application/json")

	var req *client.UpdateTaskRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Errorw("UpdateTaskRequest json decode", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = State.Update(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//spew.Dump(req)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(State)
}
