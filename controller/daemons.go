package controller

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/filecoin-project/dealbot/controller/spawn"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/google/uuid"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"
	_ "github.com/filecoin-project/lotus/lib/sigs/bls"
	_ "github.com/filecoin-project/lotus/lib/sigs/secp"

	"github.com/gorilla/mux"
)

type Funds struct {
	Balance big.Int `json:"balance,omitempty"`
	DataCap big.Int `json:"datacap,omitempty"`
}

type DaemonWithFunds struct {
	Daemon *spawn.Daemon `json:"daemon"`
	Funds  Funds         `json:"funds,omitempty"`
}

type DaemonList struct {
	Daemons []DaemonWithFunds `json:"daemons"`
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
	daemonsWithFunds := make([]DaemonWithFunds, 0, len(daemons))
	for _, daemon := range daemons {
		daemonsWithFunds = append(daemonsWithFunds, toDaemonWithFunds(r.Context(), daemon, c.gateway))
	}
	json.NewEncoder(w).Encode(&DaemonList{
		Daemons: daemonsWithFunds,
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
	json.NewEncoder(w).Encode(toDaemonWithFunds(r.Context(), daemon, c.gateway))
}

func (c *Controller) deleteDaemonHandler(w http.ResponseWriter, r *http.Request) {
	logger := log.With("req_id", r.Header.Get("X-Request-ID"))
	logger.Debugw("handle request", "command", "list tasks")
	defer logger.Debugw("request handled", "command", "list tasks")
	w.Header().Set("Content-Type", "application/json")
	enableCors(&w, r)

	regionid := mux.Vars(r)["regionid"]
	daemonid := mux.Vars(r)["daemonid"]
	err := c.spawner.Shutdown(regionid, daemonid)
	if err != nil {
		if errors.Is(err, spawn.DaemonNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			log.Errorw("error getting daemon", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	if daemon.LotusDockerRepo == "" {
		daemon.LotusDockerRepo = "filecoin/lotus"
	}
	if daemon.LotusDockerTag == "" {
		daemon.LotusDockerTag = "nightly"
	}
	// generate random values if they aren't already setup
	daemonDefaults(daemon)
	if err := c.spawner.Spawn(daemon); err != nil {
		log.Errorw("could not spawn daemon", "daemonid", daemon.Id, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// daemon is already created; sanatize output
	daemon.Wallet.Exported = ""
	json.NewEncoder(w).Encode(toDaemonWithFunds(r.Context(), daemon, c.gateway))
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

func toDaemonWithFunds(ctx context.Context, daemon *spawn.Daemon, gateway api.Gateway) (daemonWithFunds DaemonWithFunds) {
	daemonWithFunds.Daemon = daemon
	if gateway == nil {
		return
	}
	if daemon.Wallet == nil {
		return
	}
	addr, err := address.NewFromString(daemon.Wallet.Address)
	if err != nil {
		return
	}
	balance, err := gateway.WalletBalance(ctx, addr)
	if err == nil {
		daemonWithFunds.Funds.Balance = balance
	}

	head, err := gateway.ChainHead(ctx)
	if err == nil {
		dataCap, err := gateway.StateVerifiedClientStatus(ctx, addr, head.Key())
		if err == nil && dataCap != nil {
			daemonWithFunds.Funds.DataCap = *dataCap
		}
	}
	return
}
