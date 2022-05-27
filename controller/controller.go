//go:generate go run ./webutil/gen app static/script.js

package controller

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/filecoin-project/dealbot/controller/publisher"
	"github.com/filecoin-project/dealbot/controller/spawn"
	"github.com/filecoin-project/dealbot/controller/state"
	"github.com/filecoin-project/dealbot/controller/webutil"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/metrics"
	metricslog "github.com/filecoin-project/dealbot/metrics/log"
	"github.com/filecoin-project/dealbot/metrics/prometheus"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-ds-sql/postgres"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var log = logging.Logger("controller")

// Automatically set through -ldflags
// Example: go install -ldflags "-X controller.buildDate=`date -u +%d/%m/%Y@%H:%M:%S`"
var (
	buildDate = "unknown"
)

type Controller struct {
	server          *http.Server
	l               net.Listener
	doneCh          chan struct{}
	db              state.State
	basicauth       string
	metricsRecorder metrics.MetricsRecorder
	spawner         spawn.Spawner
	gateway         api.Gateway
	nodeCloser      lotus.NodeCloser
	pub             publisher.Publisher
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

	connector, err := state.NewDBConnector(ctx.String("driver"), ctx.String("dbloc"))
	if err != nil {
		return nil, err
	}
	migrator, err := state.NewMigrator(ctx.String("driver"))
	if err != nil {
		return nil, err
	}

	backend, err := state.NewStateDB(ctx.Context, connector, migrator, ctx.String("datapointlog"), key, recorder)
	if err != nil {
		return nil, err
	}

	// Get the configured libp2p host listen addresses
	var listenAddrs []multiaddr.Multiaddr
	for _, a := range ctx.StringSlice("libp2p-addrs") {
		addr, err := multiaddr.NewMultiaddr(a)
		if err != nil {
			return nil, err
		}
		listenAddrs = append(listenAddrs, addr)
	}

	// Get the list of multiaddrs to include in legs messages as the external address.
	var legsAdvAddrs []multiaddr.Multiaddr
	for _, a := range ctx.StringSlice("legs-advertised-addrs") {
		addr, err := multiaddr.NewMultiaddr(a)
		if err != nil {
			return nil, err
		}
		legsAdvAddrs = append(legsAdvAddrs, addr)
	}

	// Instantiate a new libp2p host needed by publisher.
	host, err := libp2p.New(libp2p.ListenAddrs(listenAddrs...), libp2p.Identity(key))
	if err != nil {
		return nil, err
	}

	// Get libp2p bootstrap hosts.
	var btstrp []peer.AddrInfo
	for _, b := range ctx.StringSlice("libp2p-bootstrap-addrinfo") {
		b, err := peer.AddrInfoFromString(b)
		if err != nil {
			return nil, err
		}
		btstrp = append(btstrp, *b)
	}

	// Instantiate a datastore backed by DB used internally by the publisher.
	queries := postgres.NewQueries("legs_data")
	ds := state.NewSqlDatastore(connector, queries)

	// Instantiate a store, used to read the state records created by state db.
	store := backend.Store(ctx.Context)

	// Instantiate publisher.
	pub, err := publisher.NewPandoPublisher(
		ds,
		store,
		publisher.WithHost(host),
		publisher.WithBootstrapPeers(btstrp...),
		publisher.WithExternalAddrs(legsAdvAddrs))
	if err != nil {
		return nil, err
	}

	return NewWithDependencies(ctx, l, recorder, backend, pub)
}

type logEcapsulator struct {
	logger *logging.ZapEventLogger
}

func (fw *logEcapsulator) Write(p []byte) (n int, err error) {
	fw.logger.Infow("http req", "logline", string(p))
	return len(p), nil
}

//go:embed static
var static embed.FS

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// allow cross domain AJAX requests
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		next.ServeHTTP(w, r)
	})
}

func NewWithDependencies(ctx *cli.Context, listener net.Listener, recorder metrics.MetricsRecorder, backend state.State, pub publisher.Publisher) (*Controller, error) {
	srv := new(Controller)
	srv.db = backend
	srv.pub = pub
	srv.basicauth = ctx.String("basicauth")
	if ctx.String("daemon-driver") == "kubernetes" {
		srv.spawner = spawn.NewKubernetes()
	} else {
		srv.spawner = spawn.NewLocal(ctx.String("listen"))
	}

	if ctx.IsSet("gateway-api") {
		var err error
		srv.gateway, srv.nodeCloser, err = lotus.SetupGateway(ctx)
		if err != nil {
			return nil, fmt.Errorf("Error setting up lotus gateway: %w", err)
		}
	}

	r := mux.NewRouter().StrictSlash(true)
	r.Use(CorsMiddleware)

	// Set a unique request ID.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New().String()[:8])
			next.ServeHTTP(w, r)
		})
	})

	statDir, err := fs.Sub(static, "static")
	if err != nil {
		return nil, err
	}
	r.HandleFunc("/drain/{workedby}", srv.drainHandler).Methods("POST")
	r.HandleFunc("/reset-worker/{workedby}", srv.resetWorkerHandler).Methods("POST")
	r.HandleFunc("/complete/{workedby}", srv.completeHandler).Methods("POST")
	r.HandleFunc("/pop-task", srv.popTaskHandler).Methods("POST")
	r.HandleFunc("/tasks", srv.getTasksHandler).Methods("GET")
	r.HandleFunc("/tasks/storage", srv.newStorageTaskHandler).Methods("POST")
	r.HandleFunc("/tasks/retrieval", srv.newRetrievalTaskHandler).Methods("POST")
	r.HandleFunc("/status", srv.reportStatusHandler).Methods("POST")
	r.HandleFunc("/tasks/{uuid}", srv.updateTaskHandler).Methods("PATCH")
	r.HandleFunc("/tasks/{uuid}", srv.getTaskHandler).Methods("GET")
	r.HandleFunc("/tasks/{uuid}", srv.deleteTaskHandler).Methods("DELETE")
	r.HandleFunc("/car", srv.carHandler).Methods("GET")
	r.HandleFunc("/health", srv.healthHandler).Methods("GET")
	r.HandleFunc("/regions", srv.getRegionsHandler).Methods("GET")
	r.HandleFunc("/regions/{regionid}", srv.getDaemonsHandler).Methods("GET")
	r.HandleFunc("/regions/{regionid}", srv.newDaemonHandler).Methods("POST")
	r.HandleFunc("/regions/{regionid}/{daemonid}", srv.getDaemonHandler).Methods("GET")
	r.HandleFunc("/regions/{regionid}/{daemonid}", srv.deleteDaemonHandler).Methods("DELETE")
	r.HandleFunc("/cred.js", srv.authHandler).Methods("GET")
	r.Methods("OPTIONS").HandlerFunc(srv.sendCORSHeaders)
	metricsHandler := recorder.Handler()
	if metricsHandler != nil {
		r.Handle("/metrics", metricsHandler)
	}

	if ctx.IsSet("devAssetDir") {
		scriptResolver := func(w http.ResponseWriter, r *http.Request) {
			data, _ := webutil.Compile(path.Join(ctx.String("devAssetDir"), "app"), false)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, data)
		}
		r.HandleFunc("/script.js", scriptResolver)
		r.PathPrefix("/").Handler(http.FileServer(http.Dir(path.Join(ctx.String("devAssetDir"), "static"))))
	} else {
		r.PathPrefix("/").Handler(http.FileServer(http.FS(statDir)))
	}

	srv.doneCh = make(chan struct{})
	srv.server = &http.Server{
		Handler:      handlers.LoggingHandler(&logEcapsulator{log}, r),
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	srv.l = listener
	srv.metricsRecorder = recorder
	return srv, nil
}

// Serve starts the server and blocks until the server is closed, either
// explicitly via Shutdown, or due to a fault condition. It propagates the
// non-nil err return value from http.Serve.
func (c *Controller) Serve() error {
	if c.pub != nil {
		if err := c.pub.Start(context.TODO()); err != nil {
			log.Errorw("Failed to start publisher", "err", err)
			return err
		}
	}

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
	if c.gateway != nil {
		c.nodeCloser()
	}

	if c.pub != nil {
		if err := c.pub.Shutdown(ctx); err != nil {
			log.Errorw("Failed to shut down publisher", "err", err)
		}
	}

	return c.server.Shutdown(ctx)
}
