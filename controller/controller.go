package controller

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	metricslog "github.com/filecoin-project/dealbot/metrics/log"
	"github.com/filecoin-project/dealbot/metrics/prometheus"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	//"github.com/filecoin-project/dealbot/controller/postgresdb"
	//_ "github.com/lib/pq"
	"github.com/filecoin-project/dealbot/controller/sqlitedb"
	_ "github.com/mattn/go-sqlite3"
)

// File used for sqlite database
const (
	dbFile    = "state.db"
	pgConnStr = ""
)

var log = logging.Logger("controller")

type Controller struct {
	server          *http.Server
	l               net.Listener
	doneCh          chan struct{}
	metricsRecorder metrics.MetricsRecorder
	state           *state
}

func New(ctx *cli.Context) (*Controller, error) {
	var recorder metrics.MetricsRecorder
	if ctx.String("metrics") == "prometheus" {
		recorder = prometheus.NewPrometheusMetricsRecorder()
	} else {
		recorder = metricslog.NewLogMetricsRecorder(log)
	}
	l, err := net.Listen("tcp", ctx.String("listen"))
	if err != nil {
		return nil, err
	}

	//pgConnStr := PostgresConfig{}.String()
	//db := postgresdb.New(pgConnString)
	db := sqlitedb.New(dbFile)

	st, err := NewState(ctx.Context, db)
	if err != nil {
		return nil, err
	}

	return NewWithDependencies(l, recorder, st), nil
}

func NewWithDependencies(listener net.Listener, recorder metrics.MetricsRecorder, st *state) *Controller {
	srv := new(Controller)

	r := mux.NewRouter().StrictSlash(true)

	// Set a unique request ID.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New().String()[:8])
			next.ServeHTTP(w, r)
		})
	})

	r.HandleFunc("/tasks", srv.getTasksHandler).Methods("GET")
	r.HandleFunc("/status", srv.reportStatusHandler).Methods("POST")
	r.HandleFunc("/task", srv.updateTaskHandler).Methods("PUT")
	metricsHandler := recorder.Handler()
	if metricsHandler != nil {
		r.Handle("/metrics", metricsHandler)
	}
	//r.HandleFunc("/task", srv.getTaskHandler).Methods("GET")

	srv.doneCh = make(chan struct{})
	srv.server = &http.Server{
		Handler:      r,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	srv.l = listener
	srv.metricsRecorder = recorder
	srv.state = st
	return srv
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
