package controller

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/filecoin-project/dealbot/controller/spawn"
	"github.com/google/uuid"

	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"
	_ "github.com/filecoin-project/lotus/lib/sigs/bls"
	_ "github.com/filecoin-project/lotus/lib/sigs/secp"

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
	daemons, err := c.spawner.List(regionid)
	if err != nil {
		if errors.Is(err, spawn.RegionNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			log.Errorw("error listing region", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	json.NewEncoder(w).Encode(&DaemonList{
		Daemons: daemons,
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
	daemon, err := c.spawner.Get(regionid, daemonid)
	if err != nil {
		if errors.Is(err, spawn.DaemonNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			log.Errorw("error getting daemon", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	json.NewEncoder(w).Encode(daemon)
}

func (c *Controller) newDaemonHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))
	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")
	w.Header().Set("Content-Type", "application/json")
	enableCors(&w, r)

	daemon := new(spawn.Daemon)
	if err := json.NewDecoder(r.Body).Decode(daemon); err != nil {
		log.Info("could not decode daemon request", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	regionid := mux.Vars(r)["regionid"]
	daemon.Region = regionid
	// generate random values if they aren't already setup
	daemonDefaults(daemon)
	if err := c.spawner.Spawn(daemon); err != nil {
		log.Errorw("could not spawn daemon", "daemonid", daemon.Id, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// daemon is already created; sanatize output
	daemon.Wallet.Exported = ""
	json.NewEncoder(w).Encode(daemon)
}

func daemonDefaults(d *spawn.Daemon) {
	if !(d.Wallet != nil && d.Wallet.Address != "" && d.Wallet.Exported != "") {
		k, _ := wallet.GenerateKey(types.KTSecp256k1)
		b, _ := json.Marshal(k.KeyInfo)
		d.Wallet = &spawn.Wallet{
			Address:  k.Address.String(),
			Exported: hex.EncodeToString(b),
		}
	}
	if d.Id == "" {
		d.Id = "daemon-" + uuid.New().String()[:8]
	}
	if d.Workers == 0 {
		d.Workers = 1
	}
	if d.DockerRepo == "" {
		d.DockerRepo = "filecoin/dealbot"
	}
	if d.DockerTag == "" {
		d.DockerTag = "latest"
	}
}
