package controller

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/controller/graphql"
	"github.com/filecoin-project/dealbot/controller/state"
	"github.com/filecoin-project/dealbot/metrics"
	metricslog "github.com/filecoin-project/dealbot/metrics/log"
	"github.com/filecoin-project/dealbot/metrics/prometheus"
	"github.com/libp2p/go-libp2p-core/crypto"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var log = logging.Logger("controller")

type Controller struct {
	server          *http.Server
	gserver         *http.Server
	l               net.Listener
	gl              net.Listener
	doneCh          chan struct{}
	db              state.State
	metricsRecorder metrics.MetricsRecorder
	popTaskLk       sync.Mutex
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
	var gl net.Listener
	if ctx.IsSet("graphql") {
		gl, err = net.Listen("tcp", ctx.String("graphql"))
		if err != nil {
			return nil, err
		}
	}

	var key crypto.PrivKey
	identity := ctx.String("identity")
	if !ctx.IsSet("identity") {
		identity = ".dealbot.key"
	}
	if _, err := os.Stat(identity); os.IsNotExist(err) {
		// make a new identity
		pr, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
		if err != nil {
			return nil, err
		}

		// save it.
		b, err := crypto.MarshalPrivateKey(pr)
		if err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(identity, b, 0600); err != nil {
			return nil, err
		}
		key = pr
	} else {
		// load identity
		bytes, err := ioutil.ReadFile(identity)
		if err != nil {
			return nil, err
		}
		key, err = crypto.UnmarshalPrivateKey(bytes)
		if err != nil {
			return nil, err
		}
	}

	backend, err := state.NewStateDB(ctx.Context, ctx.String("driver"), ctx.String("dbloc"), key, recorder)
	if err != nil {
		return nil, err
	}

	return NewWithDependencies(l, gl, recorder, backend)
}

func NewWithDependencies(listener, graphqlListener net.Listener, recorder metrics.MetricsRecorder, backend state.State) (*Controller, error) {
	srv := new(Controller)
	srv.db = backend

	r := mux.NewRouter().StrictSlash(true)

	// Set a unique request ID.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New().String()[:8])
			next.ServeHTTP(w, r)
		})
	})

	r.HandleFunc("/pop-task", srv.popTaskHandler).Methods("POST")
	r.HandleFunc("/tasks", srv.getTasksHandler).Methods("GET")
	r.HandleFunc("/tasks/storage", srv.newStorageTaskHandler).Methods("POST")
	r.HandleFunc("/tasks/retrieval", srv.newRetrievalTaskHandler).Methods("POST")
	r.HandleFunc("/status", srv.reportStatusHandler).Methods("POST")
	r.HandleFunc("/tasks/{uuid}", srv.updateTaskHandler).Methods("PATCH")
	r.HandleFunc("/tasks/{uuid}", srv.getTaskHandler).Methods("GET")
	r.Methods("OPTIONS").HandlerFunc(srv.sendCORSHeaders)
	metricsHandler := recorder.Handler()
	if metricsHandler != nil {
		r.Handle("/metrics", metricsHandler)
	}
	//r.HandleFunc("/task", srv.getTaskHandler).Methods("GET")

	srv.doneCh = make(chan struct{})
	srv.server = &http.Server{
		Handler:      handlers.LoggingHandler(os.Stdout, r),
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	if graphqlListener != nil {
		gqlHandler, err := graphql.GetHandler(srv.db)
		if err != nil {
			return nil, err
		}

		srv.gserver = &http.Server{
			Handler:      handlers.LoggingHandler(os.Stdout, gqlHandler),
			WriteTimeout: 30 * time.Second,
			ReadTimeout:  30 * time.Second,
		}
	}

	srv.l = listener
	srv.gl = graphqlListener
	srv.metricsRecorder = recorder
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

	if c.gserver != nil {
		go func() {
			log.Infow("graphql listening", "addr", c.gl.Addr().String())
			c.gserver.Serve(c.gl)
		}()
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
	if c.gserver != nil {
		c.gserver.Shutdown(ctx)
	}
	return c.server.Shutdown(ctx)
}
