package state

import "database/sql"

// DBConnector provides an interface for working with the underlying DB implementations
type DBConnector interface {
	Connect() error
	SqlDB() *sql.DB
}
