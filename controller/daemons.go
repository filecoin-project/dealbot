package controller

import (
	"encoding/json"
	"net/http"

	"github.com/filecoin-project/dealbot/controller/spawn"
	"github.com/gorilla/mux"
)

type DaemonList struct {
	Daemons []*spawn.Daemon `json:"daemons"`
}

func (c *Controller) getDaemonsHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))
	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")
	w.Header().Set("Content-Type", "application/json")
	enableCors(&w, r)

	regionid := mux.Vars(r)["regionid"]
	json.NewEncoder(w).Encode(&DaemonList{
		Daemons: c.spawner.List(regionid),
	})
}

func (c *Controller) getDaemonHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))
	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")
	w.Header().Set("Content-Type", "application/json")
	enableCors(&w, r)

	regionid := mux.Vars(r)["regionid"]
	daemonid := mux.Vars(r)["daemonid"]

	json.NewEncoder(w).Encode(c.spawner.Get(regionid, daemonid))
}

func (c *Controller) newDaemonHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))
	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")
	w.Header().Set("Content-Type", "application/json")
	enableCors(&w, r)

	daemon := new(spawn.Daemon)
	if err := json.NewDecoder(r.Body).Decode(daemon); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := c.spawner.Spawn(daemon); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
