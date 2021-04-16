package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/engine"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var log = logging.Logger("daemon")

type Daemon struct {
	server *http.Server
	l      net.Listener
	doneCh chan struct{}
	e      *engine.Engine
}

func New(ctx context.Context, cliCtx *cli.Context) (srv *Daemon, err error) {
	srv = new(Daemon)

	r := mux.NewRouter().StrictSlash(true)

	srv.e, err = engine.New(ctx, cliCtx)
	if err != nil {
		return nil, err
	}

	// Set a unique request ID.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New().String()[:8])
			next.ServeHTTP(w, r)
		})
	})

	//r.HandleFunc("/tasks", srv.listTasksHandler).Methods("GET")

	srv.doneCh = make(chan struct{})
	srv.server = &http.Server{
		Handler:      r,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	srv.l, err = net.Listen("tcp", cliCtx.String("listen"))
	fmt.Println(err)
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
	defer func() {
		close(d.doneCh)
		d.e.Close()
	}()
	return d.server.Shutdown(ctx)
}
