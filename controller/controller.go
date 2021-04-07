package controller

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/config"
	logging "github.com/ipfs/go-log/v2"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
)

var log = logging.Logger("controller")

type Controller struct {
	server *http.Server
	l      net.Listener
	doneCh chan struct{}
}

func New(cfg *config.EnvConfig) (srv *Controller, err error) {
	srv = new(Controller)

	r := mux.NewRouter().StrictSlash(true)

	// Set a unique request ID.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New()[:8])
			next.ServeHTTP(w, r)
		})
	})

	r.HandleFunc("/tasks", srv.getTasksHandler).Methods("GET")
	r.HandleFunc("/status", srv.reportStatusHandler).Methods("POST")
	//r.HandleFunc("/task", srv.getTaskHandler).Methods("GET")
	//r.HandleFunc("/task", srv.updateTaskHandler).Methods("PUT")

	srv.doneCh = make(chan struct{})
	srv.server = &http.Server{
		Handler:      r,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	srv.l, err = net.Listen("tcp", cfg.Controller.Listen)
	if err != nil {
		return nil, err
	}

	return srv, nil
}

// Serve starts the server and blocks until the server is closed, either
// explicitly via Shutdown, or due to a fault condition. It propagates the
// non-nil err return value from http.Serve.
func (c *Controller) Serve() error {
	select {
	case <-c.doneCh:
		return fmt.Errorf("tried to reuse a stopped server")
	default:
	}

	log.Infow("controller listening", "addr", c.Addr())
	return c.server.Serve(c.l)
}

func (c *Controller) Addr() string {
	return c.l.Addr().String()
}

func (c *Controller) Port() int {
	return c.l.Addr().(*net.TCPAddr).Port
}

func (c *Controller) Shutdown(ctx context.Context) error {
	defer close(c.doneCh)
	return c.server.Shutdown(ctx)
}
