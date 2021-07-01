package temporary

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const defaultInterval = 100 * time.Millisecond
const defaultStartupTimeout = 60 * time.Second
const defaultUser = "postgres"
const defaultPassword = "temp"
const defaultDatabase = "postgres"
const defaultHostPort = 5432

type Connector struct {
	*postgresdb.PostgresDB
	containerRef testcontainers.Container
	tmpDir       string
}

type Params struct {
	WaitInterval   time.Duration
	StartupTimeout time.Duration
	User           string
	Password       string
	Database       string
	HostPort       int
}

func (p Params) HostPortString() string {
	return strconv.Itoa(p.HostPort)
}

func withDefaults(p Params) Params {
	newP := Params{
		WaitInterval:   defaultInterval,
		StartupTimeout: defaultStartupTimeout,
		User:           defaultUser,
		Password:       defaultPassword,
		Database:       defaultDatabase,
		HostPort:       defaultHostPort,
	}
	if p.WaitInterval != 0 {
		newP.WaitInterval = p.WaitInterval
	}
	if p.StartupTimeout != 0 {
		newP.StartupTimeout = p.StartupTimeout
	}
	if p.User != "" {
		newP.User = p.User
	}
	if p.Password != "" {
		newP.Password = p.Password
	}
	if p.Database != "" {
		newP.Database = p.Database
	}
	if p.HostPort != 0 {
		newP.HostPort = p.HostPort
	}
	return newP
}

func NewTemporaryPostgres(ctx context.Context, params Params) (*Connector, error) {
	params = withDefaults(params)

	tmpDir, err := ioutil.TempDir("", "testdealbot")
	if err != nil {
		return nil, err
	}
	cw := &connectorWaitor{Params: params}
	req := testcontainers.ContainerRequest{
		Image: "postgres",
		Env: map[string]string{
			"POSTGRES_USER":     params.User,
			"POSTGRES_PASSWORD": params.Password,
			"POSTGRES_DB":       params.Database,
		},
		BindMounts: map[string]string{
			tmpDir: "/var/lib/postgresql/data",
		},
		ExposedPorts: []string{params.HostPortString() + ":5432/tcp"},
		WaitingFor:   cw,
	}

	containerRef, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}
	return &Connector{PostgresDB: cw.connector, containerRef: containerRef, tmpDir: tmpDir}, nil
}

func (tc *Connector) Shutdown(ctx context.Context) error {
	err := tc.PostgresDB.DB.Close()
	if err != nil {
		return err
	}
	err = tc.containerRef.Terminate(ctx)
	if err != nil {
		return err
	}
	err = os.RemoveAll(tc.tmpDir)
	return err
}

type connectorWaitor struct {
	connector *postgresdb.PostgresDB
	Params
}

//WaitUntilReady repeatedly tries to run "SELECT 1" query on the given port using sql and driver.
// If the it doesn't succeed until the timeout value which defaults to 60 seconds, it will return an error
func (cw *connectorWaitor) WaitUntilReady(ctx context.Context, target wait.StrategyTarget) (err error) {
	ctx, cancel := context.WithTimeout(ctx, cw.StartupTimeout)
	defer cancel()

	ticker := time.NewTicker(cw.WaitInterval)
	defer ticker.Stop()

	hp, err := nat.NewPort("tcp", "5432")
	if err != nil {
		return err
	}
	port, err := target.MappedPort(ctx, hp)
	for err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			port, err = target.MappedPort(ctx, hp)
		}
	}

	host, err := target.Host(ctx)
	for err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			host, err = target.Host(ctx)
		}
	}

	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host,
		port.Int(),
		cw.User,
		cw.Password,
		cw.Database,
	)

	cw.connector = postgresdb.New(connString)

	err = cw.connector.Connect()
	for err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err = cw.connector.Connect()
		}
	}
	return nil
}
