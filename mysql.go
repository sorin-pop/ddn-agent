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
	"time"

	"github.com/djavorszky/ddn-common/logger"
	"github.com/djavorszky/ddn-common/model"
	"github.com/djavorszky/sutils"

	_ "github.com/go-sql-driver/mysql"
)

type mysql struct {
	conn *sql.DB
}

// Connect creates and initialises a Database struct and connects to the database
func (db *mysql) Connect(c Config) error {
	var err error

	if ok := sutils.Present(c.User, c.LocalDBAddr, c.LocalDBPort); !ok {
		return fmt.Errorf("missing parameters. Need-Got: {user: %s}, {dbAddress: %s}, {dbPort: %s}", c.User, c.LocalDBAddr, c.LocalDBPort)
	}

	datasource := fmt.Sprintf("%s:%s@tcp(%s:%s)/", c.User, c.Password, c.LocalDBAddr, c.LocalDBPort)
	db.conn, err = sql.Open("mysql", datasource)
	if err != nil {
		return fmt.Errorf("creating connection pool failed: %s", err.Error())
	}

	err = db.conn.Ping()
	if err != nil {
		db.conn.Close()
		return fmt.Errorf("database ping failed: %s", strip(err.Error()))
	}

	return nil
}

// Close closes the connection to the database
func (db *mysql) Close() {
	db.conn.Close()
}

// Alive checks whether the connection is alive. Returns error if not.
func (db *mysql) Alive() error {
	defer func() {
		if p := recover(); p != nil {
			logger.Error("Panic Attack! Database seems to be down.")
		}
	}()

	_, err := db.conn.Exec("select * from mysql.user WHERE 1 = 0")
	if err != nil {
		return fmt.Errorf("executing stayalive query failed: %s", strip(err.Error()))
	}

	return nil
}

// ListDatabase returns a list of strings - the names of the databases in the server
// All system tables are omitted from the returned list. If there's an error, it is returned.
func (db *mysql) ListDatabase() ([]string, error) {
	var err error

	err = db.Alive()
	if err != nil {
		return nil, fmt.Errorf("alive check failed: %s", err.Error())
	}

	rows, err := db.conn.Query("show databases")
	if err != nil {
		return nil, fmt.Errorf("listing databases failed: %s", strip(err.Error()))
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
		case "information_schema", "performance_schema", "mysql", "nbinfo", "sys":
			continue
		}

		list = append(list, database)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error encountered when reading rows: %s", strip(err.Error()))
	}

	return list, nil
}

// CreateDatabase creates a Database along with a user, to which all privileges
// are granted on the created database. Fails if database or user already exists.
func (db *mysql) CreateDatabase(dbRequest model.DBRequest) error {
	err := db.Alive()
	if err != nil {
		return fmt.Errorf("alive check failed: %s", err.Error())
	}

	switch dbRequest.DatabaseName {
	case "information_schema", "performance_schema", "mysql", "nbinfo", "sys":
		return fmt.Errorf("trying to create system databases not allowed")
	}

	switch dbRequest.Username {
	case "root", conf.User:
		return fmt.Errorf("trying to create root user not allowed")
	}

	exists, err := db.dbExists(dbRequest.DatabaseName)
	if err != nil {
		return fmt.Errorf("checking if database exists failed: %s", err.Error())
	}
	if exists {
		return fmt.Errorf("database '%s' already exists", dbRequest.DatabaseName)
	}

	// Begin transaction so that we can roll it back at any point something goes wrong.
	tx, err := db.conn.Begin()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("starting transaction failed: %s", strip(err.Error()))
	}

	_, err = db.conn.Exec(fmt.Sprintf("CREATE DATABASE %s CHARSET utf8;", dbRequest.DatabaseName))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("executing create database query failed: %s", strip(err.Error()))
	}

	exists, err = db.userExists(dbRequest.Username)
	if err != nil {
		return fmt.Errorf("checking if user exists failed: %s", err.Error())
	}
	if !exists {
		db.conn.Exec(fmt.Sprintf("FLUSH PRIVILEGES"))

		_, err = db.conn.Exec(fmt.Sprintf("CREATE USER '%s' IDENTIFIED BY '%s';", dbRequest.Username, dbRequest.Password))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("executing create user '%s' failed: %s", dbRequest.Username, strip(err.Error()))
		}
	}

	_, err = db.conn.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%s';", dbRequest.DatabaseName, dbRequest.Username, "%"))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("executing grant privileges to user '%s' on database '%s' failed: %s", dbRequest.Username, dbRequest.DatabaseName, strip(err.Error()))
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("committing transaction failed: %s", strip(err.Error()))
	}

	return nil
}

// DropDatabase drops a database and a user. Always succeeds, even if droppable database or
// user does not exist
func (db *mysql) DropDatabase(dbRequest model.DBRequest) error {
	err := db.Alive()
	if err != nil {
		return fmt.Errorf("alive check failed: %s", err.Error())
	}

	switch dbRequest.DatabaseName {
	case "information_schema", "performance_schema", "mysql", "nbinfo", "sys":
		return fmt.Errorf("dropping system databases not allowed")
	}

	switch dbRequest.Username {
	case "root", conf.User:
		return fmt.Errorf("dropping root user not allowed")
	}

	tx, err := db.conn.Begin()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("starting transaction failed: %s", strip(err.Error()))
	}

	_, err = db.conn.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbRequest.DatabaseName))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("dropping database '%s' failed: %s", dbRequest.DatabaseName, strip(err.Error()))
	}

	exists, err := db.userExists(dbRequest.Username)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("checking if user exists failed: %s", err.Error())
	}

	if exists {
		// Silently try to revoke privileges. MySQL errors out if we're trying to revoke a privilege
		// when there's no such privilege.
		db.conn.Exec(fmt.Sprintf("REVOKE ALL PRIVILEGES ON %s.* FROM %q", dbRequest.DatabaseName, dbRequest.Username))

		var count int
		err = db.conn.QueryRow("select count(*) from mysql.db where user = ?", dbRequest.Username).Scan(&count)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("checking grant-count failed: %s", err.Error())
		}

		if count == 0 {
			_, err = db.conn.Exec(fmt.Sprintf("DROP USER %s", dbRequest.Username))
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("dropping user '%s' failed: %s", dbRequest.Username, strip(err.Error()))
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("committing transaction failed: %s", strip(err.Error()))
	}

	return nil
}

// ImportDatabase imports the dumpfile to the database or returns an error
// if it failed for some reason.
func (db *mysql) ImportDatabase(dbreq model.DBRequest) error {
	var errBuf bytes.Buffer

	file, err := os.Open(dbreq.DumpLocation)
	if err != nil {
		db.DropDatabase(dbreq)
		return fmt.Errorf("could not open dumpfile '%s': %s", dbreq.DumpLocation, err.Error())
	}
	defer file.Close()

	// Start the import
	args := []string{
		fmt.Sprintf("--host=%s", conf.LocalDBAddr),
		fmt.Sprintf("--port=%s", conf.LocalDBPort),
		fmt.Sprintf("-u%s", dbreq.Username),
		fmt.Sprintf("-p%s", dbreq.Password),
		dbreq.DatabaseName,
	}

	cmd := exec.Command(conf.Exec, args...)

	cmd.Stdin = file
	cmd.Stderr = &errBuf

	err = cmd.Run()
	if err != nil {
		db.DropDatabase(dbreq)
		return fmt.Errorf("could not execute import command: %s", strip(errBuf.String()))
	}

	return nil
}

// ExportDatabase exports the database to dumpfile or returns an error
// if it failed for some reason.
func (db *mysql) ExportDatabase(dbreq model.DBRequest) (string, error) {
	var errBuf bytes.Buffer

	fullDumpFilename := fmt.Sprintf("%s_%s.sql", dbreq.DatabaseName, time.Now().Format("20060102150405"))

	outputfile, err := os.Create(filepath.Join(workdir, "exports", fullDumpFilename))
	if err != nil {
		return "", fmt.Errorf("could not create dumpfile '%s': %s", fullDumpFilename, err.Error())
	}
	defer outputfile.Close()

	args := []string{
		fmt.Sprintf("--host=%s", conf.LocalDBAddr),
		fmt.Sprintf("--port=%s", conf.LocalDBPort),
		fmt.Sprintf("-u%s", dbreq.Username),
		fmt.Sprintf("-p%s", dbreq.Password),
		dbreq.DatabaseName,
	}

	cmd := exec.Command("mysqldump", args...)

	cmd.Stdout = outputfile
	cmd.Stderr = &errBuf

	err = cmd.Run()

	if err != nil {
		return "", fmt.Errorf("could not execute mysqldump command: %s", strip(errBuf.String()))
	}

	return fullDumpFilename, nil
}

func (db *mysql) Version() (string, error) {
	var buf bytes.Buffer

	cmd := exec.Command(conf.Exec, "--version")
	cmd.Stdout = &buf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("could not execute command: %s", err.Error())
	}
	re := regexp.MustCompile("[0-9]+.[0-9]+.[0-9]+")

	return re.FindString(buf.String()), nil
}

func (db *mysql) RequiredFields(dbreq model.DBRequest, reqType int) []string {
	req := []string{dbreq.DatabaseName, dbreq.Username}

	switch reqType {
	case createDB:
		req = append(req, dbreq.Password)
	case importDB:
		req = append(req, strconv.Itoa(dbreq.ID), dbreq.Password, dbreq.DumpLocation)
	}

	return req
}

func (db *mysql) dbExists(databasename string) (bool, error) {
	var count int

	err := db.conn.QueryRow("SELECT count(*) FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", databasename).Scan(&count)
	if err != nil {
		return true, fmt.Errorf("executing query failed: %s", strip(err.Error()))
	}
	if count != 0 {
		return true, nil
	}

	return false, nil
}

func (db *mysql) userExists(username string) (bool, error) {
	var count int

	err := db.conn.QueryRow("SELECT count(*) FROM mysql.user WHERE user = ?", username).Scan(&count)
	if err != nil {
		return true, fmt.Errorf("executing query failed: %s", strip(err.Error()))
	}
	if count != 0 {
		return true, nil
	}

	return false, nil
}

func strip(test string) string {
	return strings.TrimSuffix(test, "\n")
}

func (db *mysql) ValidateDump(path string) (string, error) {
	toRemove := []string{
		"CREATE DATABASE",
		"DROP DATABASE",
		"/*!50013 DEFINER",
		"USE ",
		"SET @@SESSION.SQL_LOG_BIN",
		"SET @@GLOBAL.GTID",

		"create database",
		"drop database",
		"/*!50013 definer",
		"use ",
		"set @@session.sql_log_bin",
		"set @@global.gtid",
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("could not open dumpfile '%s': %s", path, err.Error())
	}
	defer file.Close()

	lines, err := sutils.FindWith(strings.HasPrefix, file, toRemove)
	if err != nil {
		return path, fmt.Errorf("couldn't find occurrences: %v", err)
	}

	logger.Debug("Problematic lines in the dump: %v", lines)

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
