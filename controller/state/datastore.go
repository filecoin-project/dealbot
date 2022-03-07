package state

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	sqlds "github.com/ipfs/go-ds-sql"
)

var _ datastore.Batching = (*SqlDatastore)(nil)

// SqlDatastore wraps a datastore.Batching with connection retry implemented in state.DBConnector.
// We need this because state.DBConnector seems to exist because db connections may drop and timeout
// during prolonged periods of inactivity, for example. The state.DBConnector is a dealbot
// construct; therefore, here we wrap a datastore to dynamically reconnect when the DB connection is
// closed.
type SqlDatastore struct {
	queries   sqlds.Queries
	connector DBConnector
}

// NewSqlDatastore instantiates a new legs datastore backed by DB.
func NewSqlDatastore(connector DBConnector, queries sqlds.Queries) *SqlDatastore {
	return &SqlDatastore{
		queries:   queries,
		connector: connector,
	}
}

func (sds *SqlDatastore) getDatastore() (*sqlds.Datastore, error) {
	// Make sure we are connection before constructing the datastore.
	// The connector implementation only instantiates a new connection if the existing one is
	// closed. So no need to check it here.
	if err := sds.connector.Connect(); err != nil {
		return nil, err
	}

	// Ideally sqlds should expose a hook to allow re-connection; for now we instantiate a new datastore with the same database.
	return sqlds.NewDatastore(sds.connector.SqlDB(), sds.queries), nil
}

func (sds *SqlDatastore) Get(ctx context.Context, key datastore.Key) (value []byte, err error) {
	ds, err := sds.getDatastore()
	if err != nil {
		return nil, err
	}
	return ds.Get(ctx, key)
}

func (sds *SqlDatastore) Has(ctx context.Context, key datastore.Key) (exists bool, err error) {
	ds, err := sds.getDatastore()
	if err != nil {
		return false, err
	}
	return ds.Has(ctx, key)
}

func (sds *SqlDatastore) GetSize(ctx context.Context, key datastore.Key) (size int, err error) {
	ds, err := sds.getDatastore()
	if err != nil {
		return 0, err
	}
	return ds.GetSize(ctx, key)
}

func (sds *SqlDatastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	ds, err := sds.getDatastore()
	if err != nil {
		return nil, err
	}
	return ds.Query(ctx, q)
}

func (sds *SqlDatastore) Put(ctx context.Context, key datastore.Key, value []byte) error {
	ds, err := sds.getDatastore()
	if err != nil {
		return err
	}
	return ds.Put(ctx, key, value)
}

func (sds *SqlDatastore) Delete(ctx context.Context, key datastore.Key) error {
	ds, err := sds.getDatastore()
	if err != nil {
		return err
	}
	return ds.Delete(ctx, key)
}

func (sds *SqlDatastore) Sync(ctx context.Context, prefix datastore.Key) error {
	ds, err := sds.getDatastore()
	if err != nil {
		return err
	}
	return ds.Sync(ctx, prefix)
}

func (sds *SqlDatastore) Batch(ctx context.Context) (datastore.Batch, error) {
	ds, err := sds.getDatastore()
	if err != nil {
		return nil, err
	}
	return ds.Batch(ctx)
}

func (sds *SqlDatastore) Close() error {
	conn := sds.connector.SqlDB()
	if conn != nil {
		return conn.Close()
	}
	return nil
}
