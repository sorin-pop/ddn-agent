package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/djavorszky/ddn-common/logger"
	"github.com/djavorszky/ddn-common/model"
	"github.com/djavorszky/sutils"
	_ "github.com/lib/pq"
)

type postgres struct {
	conn *sql.DB
}

func (db *postgres) Connect(c Config) error {
	var err error

	if ok := sutils.Present(c.User, c.Password, c.LocalDBAddr); !ok {
		return fmt.Errorf("missing parameters. Need-Got: {user: %s}, {password: %s}, {dbAddress: %s}", c.User, c.Password, c.LocalDBAddr)
	}

	datasource := fmt.Sprintf("postgres://%s:%s@%s?sslmode=disable", c.User, c.Password, c.LocalDBAddr)
	db.conn, err = sql.Open("postgres", datasource)
	if err != nil {
		return fmt.Errorf("creating connection pool failed: %s", err.Error())
	}

	err = db.conn.Ping()
	if err != nil {
		db.conn.Close()
		return fmt.Errorf("database ping failed: %s", err.Error())
	}

	return nil
}

func (db *postgres) Close() {
	db.conn.Close()
}

func (db *postgres) Alive() error {
	defer func() {
		if p := recover(); p != nil {
			logger.Error("Panic Attack! Database seems to be down.")
		}
	}()

	_, err := db.conn.Exec("select 1 from pg_roles WHERE 1 = 0")
	if err != nil {
		return fmt.Errorf("executing stayalive query failed: %s", err.Error())
	}

	return nil
}

// ListDatabase returns a list of strings - the names of the databases in the server
// All system tables are omitted from the returned list. If there's an error, it is returned.
func (db *postgres) ListDatabase() ([]string, error) {
	var err error

	err = db.Alive()
	if err != nil {
		return nil, fmt.Errorf("alive check failed: %s", err.Error())
	}

	rows, err := db.conn.Query("SELECT datname FROM pg_database WHERE datistemplate = false;")
	if err != nil {
		return nil, fmt.Errorf("listing databases failed: %s", err.Error())
	}
	defer rows.Close()

	list := make([]string, 0, 10)

	var database string
	for rows.Next() {
		err = rows.Scan(&database)
		if err != nil {
			return nil, fmt.Errorf("reading row failed: %s", err.Error())
		}

		switch database {
		case "postgres":
			continue
		}

		list = append(list, database)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error encountered when reading rows: %s", err.Error())
	}

	return list, nil
}

// CreateDatabase creates a Database along with a user, to which all privileges
// are granted on the created database. Fails if database or user already exists.
func (db *postgres) CreateDatabase(dbRequest model.DBRequest) error {
	err := db.Alive()
	if err != nil {
		return fmt.Errorf("alive check failed: %s", err.Error())
	}

	exists, err := db.dbExists(dbRequest.DatabaseName)
	if err != nil {
		return fmt.Errorf("checking if database exists failed: %s", err.Error())
	}
	if exists {
		return fmt.Errorf("database '%s' already exists", dbRequest.DatabaseName)
	}

	exists, err = db.userExists(dbRequest.Username)
	if err != nil {
		return fmt.Errorf("checking if user exists failed: %s", err.Error())
	}
	if exists {
		return fmt.Errorf("user '%s' already exists", dbRequest.Username)
	}

	// Begin transaction so that we can roll it back at any point something goes wrong.
	tx, err := db.conn.Begin()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("starting transaction failed: %s", err.Error())
	}

	_, err = db.conn.Exec(fmt.Sprintf("CREATE DATABASE %q ENCODING 'utf-8';", dbRequest.DatabaseName))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("executing create database query failed: %s", err.Error())
	}

	_, err = db.conn.Exec(fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s';", dbRequest.Username, dbRequest.Password))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("executing create user '%s' failed: %s", dbRequest.Username, err.Error())
	}

	_, err = db.conn.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %q TO %s;", dbRequest.DatabaseName, dbRequest.Username))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("executing grant privileges to user '%s' on database '%s' failed: %s", dbRequest.Username, dbRequest.DatabaseName, err.Error())
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("committing transaction failed: %s", err.Error())
	}

	return nil
}

// DropDatabase drops a database and a user. Always succeeds, even if droppable database or
// user does not exist
func (db *postgres) DropDatabase(dbRequest model.DBRequest) error {
	var err error

	err = db.Alive()
	if err != nil {
		return fmt.Errorf("alive check failed: %s", err.Error())
	}

	tx, err := db.conn.Begin()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("starting transaction failed: %s", err.Error())
	}

	_, err = db.conn.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %q", dbRequest.DatabaseName))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("dropping database '%s' failed: %s", dbRequest.DatabaseName, err.Error())
	}

	_, err = db.conn.Exec(fmt.Sprintf("DROP USER IF EXISTS %s", dbRequest.Username))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("dropping user '%s' failed: %s", dbRequest.Username, err.Error())
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("committing transaction failed: %s", err.Error())
	}

	return nil
}

// ImportDatabase imports the dumpfile to the database or returns an error
// if it failed for some reason.
func (db *postgres) ImportDatabase(dbreq model.DBRequest) error {
	addr := strings.Split(conf.LocalDBAddr, ":")
	host, port := addr[0], addr[1]

	cmd := exec.Command(conf.Exec, "-h", host, "-p", port, "-U", dbreq.Username, "-d", dbreq.DatabaseName)

	file, err := os.Open(dbreq.DumpLocation)
	if err != nil {
		db.DropDatabase(dbreq)
		return fmt.Errorf("could not open dumpfile '%s': %s", dbreq.DumpLocation, err.Error())
	}
	defer file.Close()

	cmd.Stdin = file

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	os.Setenv("PGPASSWORD", dbreq.Password)
	defer os.Setenv("PGPASSWORD", "")
	err = cmd.Run()
	if err != nil {
		db.DropDatabase(dbreq)
		return fmt.Errorf("could not execute import command: %s", errBuf.String())
	}

	return nil
}

func (db *postgres) ExportDatabase(dbRequest model.DBRequest) (string, error) {
	//fullDumpFilename := fmt.Sprintf("%s_%s.dmp", dbRequest.DatabaseName, time.Now().Format("20060102150405"))

	return "", fmt.Errorf("export not yet implemented for PostgreSQL")
}

func (db *postgres) Version() (string, error) {
	var buf bytes.Buffer

	cmd := exec.Command(conf.Exec, "--version")
	cmd.Stdout = &buf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("could not execute command: %s", err.Error())
	}

	re := regexp.MustCompile("[0-9.]+")

	return re.FindString(buf.String()), nil
}

func (db *postgres) RequiredFields(dbreq model.DBRequest, reqType int) []string {
	req := []string{dbreq.DatabaseName, dbreq.Username}

	switch reqType {
	case createDB:
		req = append(req, dbreq.Password)
	case importDB:
		req = append(req, strconv.Itoa(dbreq.ID), dbreq.Password, dbreq.DumpLocation)
	}

	return req
}

func (db *postgres) ValidateDump(path string) (string, error) {
	toRemove := []string{"ALTER TABLE", "alter table"}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("could not open dumpfile '%s': %s", path, err.Error())
	}
	defer file.Close()

	lines, err := sutils.FindWith(strings.HasPrefix, file, toRemove)
	if err != nil {
		return path, fmt.Errorf("couldn't find occurrences: %v", err)
	}

	// Rewind file
	file.Seek(0, 0)

	if len(lines) > 0 {
		tmpFile, err := ioutil.TempFile(os.TempDir(), "ddnc")
		if err != nil {
			return path, fmt.Errorf("could not create tempfile: %s", err.Error())
		}

		err = sutils.CopyWithoutLines(file, lines, tmpFile)
		if err != nil {
			return path, fmt.Errorf("removing extra lines from dump failed: %s", err.Error())
		}

		tmpFilePath, _ := filepath.Abs(tmpFile.Name())

		os.Rename(tmpFilePath, path)
	}

	// Rewind file
	file.Seek(0, 0)

	return path, nil
}

func (db *postgres) dbExists(database string) (bool, error) {
	var count int

	query := fmt.Sprintf("SELECT count(*) FROM pg_database WHERE datistemplate = false AND datname = '%s'", strings.ToLower(database))

	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return true, fmt.Errorf("executing query failed: %s", err.Error())
	}
	if count != 0 {
		return true, nil
	}

	return false, nil
}

func (db *postgres) userExists(user string) (bool, error) {
	var count int

	query := fmt.Sprintf("SELECT count(1) FROM pg_roles WHERE rolname='%s'", strings.ToLower(user))

	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return true, fmt.Errorf("executing query failed: %s", err.Error())
	}
	if count != 0 {
		return true, nil
	}

	return false, nil
}
