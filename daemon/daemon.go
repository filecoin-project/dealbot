package daemon

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

var log = logging.Logger("daemon")

type Daemon struct {
	server *http.Server
	l      net.Listener
	doneCh chan struct{}
}

func New(cfg *config.EnvConfig) (srv *Daemon, err error) {
	srv = new(Daemon)

	r := mux.NewRouter().StrictSlash(true)

	// Set a unique request ID.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New()[:8])
			next.ServeHTTP(w, r)
		})
	})

	//r.HandleFunc("/tasks", srv.listTasksHandler(engine)).Methods("GET")

	srv.doneCh = make(chan struct{})
	srv.server = &http.Server{
		Handler:      r,
		WriteTimeout: 7200 * time.Second,
		ReadTimeout:  7200 * time.Second,
	}

	srv.l, err = net.Listen("tcp", cfg.Daemon.Listen)
	if err != nil {
		return nil, err
	}

	return srv, nil
}

// Serve starts the server and blocks until the server is closed, either
// explicitly via Shutdown, or due to a fault condition. It propagates the
// non-nil err return value from http.Serve.
func (d *Daemon) Serve() error {
	select {
	case <-d.doneCh:
		return fmt.Errorf("tried to reuse a stopped server")
	default:
	}

	log.Infow("daemon listening", "addr", d.Addr())
	return d.server.Serve(d.l)
}

func (d *Daemon) Addr() string {
	return d.l.Addr().String()
}

func (d *Daemon) Port() int {
	return d.l.Addr().(*net.TCPAddr).Port
}

func (d *Daemon) Shutdown(ctx context.Context) error {
	defer close(d.doneCh)
	return d.server.Shutdown(ctx)
}
