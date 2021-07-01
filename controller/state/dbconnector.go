package state

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
)

// DBConnector provides an interface for working with the underlying DB implementations
type DBConnector interface {
	Connect() error
	Name() string
	RetryableError(error) bool
	SqlDB() *sql.DB
}

func NewDBConnector(driver string, conn string) (DBConnector, error) {
	switch driver {
	case "postgres":
		if conn == "" {
			conn = postgresdb.PostgresConfig{}.String()
		}
		return postgresdb.New(conn), nil
	default:
		return nil, fmt.Errorf("database driver %q is not supported", driver)
	}
}

func dropAllRecords(tx *sql.Tx) error {
	_, err := tx.Exec(dropAllRecordsSQL)
	return err
}

func WipeAndReset(dbConn DBConnector, migrator Migrator) error {
	// Check connection and reconnect if down
	err := dbConn.Connect()
	if err != nil {
		return err
	}
	var start time.Time

	err = migrator(dbConn.SqlDB())
	if err != nil {
		return err
	}
	for {
		err = withTransaction(context.Background(), dbConn.SqlDB(), dropAllRecords)
		if err != nil {
			if dbConn.RetryableError(err) && (start.IsZero() || time.Since(start) < maxRetryTime) {
				if start.IsZero() {
					start = time.Now()
				}
				log.Warnw("retrying transaction after error", "err", err)
				continue
			}
			return err
		}
		return nil
	}
}
