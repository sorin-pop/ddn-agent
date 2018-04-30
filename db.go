package main

import (
	"fmt"
	"strings"

	"github.com/djavorszky/ddn/common/model"
)

var vendors = []string{"mysql", "mariadb", "oracle", "postgres", "mssql"}

const (
	createDB int = iota
	dropDB
	importDB
)

// Database interface to be used when running queries. All DB implementations
// should implement all its methods.
type Database interface {
	// Connect creates and initialises a Database struct and connects to the database
	Connect(c Config) error

	// Close closes the connection to the database
	Close()

	// Alive checks whether the connection is alive. Returns error if not.
	Alive() error

	// CreateDatabase creates a Database along with a user, to which all privileges
	// are granted on the created database. Fails if database or user already exists.
	CreateDatabase(dbRequest model.DBRequest) error

	// DropDatabase drops a database and a user. Always succeeds, even if droppable database or
	// user does not exist
	DropDatabase(dbRequest model.DBRequest) error

	// ImportDatabase imports the dumpfile to the database or returns an error
	// if it failed for some reason.
	ImportDatabase(dbRequest model.DBRequest) error

	// ExportDatabase exports a CloudDB database to a dump file and returns the file's name, or returns an error
	// if it failed for some reason.
	ExportDatabase(dbRequest model.DBRequest) (string, error)

	// ListDatabase returns a list of strings - the names of the databases in the server
	// All system tables are omitted from the returned list. If there's an error, it is returned.
	ListDatabase() ([]string, error)

	// Version returns the database server's version.
	Version() (string, error)

	// RequiredFields returns the fields that are required to be present in an API call, specific
	// to the database vendor
	RequiredFields(dbRequest model.DBRequest, reqType int) []string

	// ValidateDump validates a dumpfile at the given path. Returns another path as a string or
	// an error if something went wrong
	ValidateDump(path string) (string, error)
}

// VendorSupported returns an error if the specified vendor is not supported.
func VendorSupported(vendor string) error {
	vendor = strings.ToLower(vendor)
	for _, v := range vendors {
		if v == vendor {
			return nil
		}
	}

	return fmt.Errorf("vendor not supported: %s", vendor)
}

// GetDB returns the vendor-specific implementation of the Database interface
func GetDB(vendor string) (Database, error) {
	if err := VendorSupported(vendor); err != nil {
		return nil, err
	}

	var db Database
	switch strings.ToLower(vendor) {
	case "mysql", "mariadb":
		db = new(mysql)
	case "postgres":
		db = new(postgres)
	case "oracle":
		db = new(oracle)
	case "mssql":
		db = new(mssql)
	default:
		return nil, fmt.Errorf("database not recognized: %s", vendor)
	}

	return db, nil
}
