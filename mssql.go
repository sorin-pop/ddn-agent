package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/djavorszky/ddn/common/logger"
	"github.com/djavorszky/ddn/common/model"
)

type mssql struct {
	conn *sql.DB
}

func (db *mssql) Connect(c Config) error {
	return fmt.Errorf("operation not supported: Connect")
}

func (db *mssql) Close() {
	// Not needed
}

func (db *mssql) Alive() error {
	return fmt.Errorf("operation not supported: Alive")
}

func (db *mssql) CreateDatabase(dbRequest model.DBRequest) error {

	args := []string{"-b", "-U", conf.User, "-P", conf.Password, "-Q", fmt.Sprintf("CREATE DATABASE %s", dbRequest.DatabaseName)}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		if strings.Contains(res.stderr, "already exists") {
			return fmt.Errorf("database %q already exists", dbRequest.DatabaseName)
		}
		logger.Error("unable to create database:\n> stdout:\n%q\n> stderr:\n%q\n> exitCode: %d", res.stdout, res.stderr, res.exitCode)

		return fmt.Errorf("create database failed with exitcode '%d'", res.exitCode)
	}

	return nil
}

func (db *mssql) DropDatabase(dbRequest model.DBRequest) error {

	args := []string{"-b", "-U", conf.User, "-P", conf.Password, "-Q", fmt.Sprintf("DROP DATABASE %s", dbRequest.DatabaseName)}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		if !(strings.Contains(res.stderr, "it does not exist")) {
			logger.Error("Unable to drop database:\n> stdout:\n'%s'\n> stderr:\n'%s'\n> exitCode: %d", res.stdout, res.stderr, res.exitCode)

			return fmt.Errorf("drop database failed with exitcode '%d'", res.exitCode)
		}
	}

	return nil
}

func (db *mssql) ImportDatabase(dbRequest model.DBRequest) error {
	curDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine current directory")
	}

	s := strings.Split(dbRequest.DumpLocation, ":")

	driveLetter, dumpPath := s[0], s[1]

	args := []string{
		"-b",
		"-U", conf.User,
		"-P", conf.Password,
		"-v", "driveLetter=" + driveLetter,
		"-v", "dumpPath=" + dumpPath,
		"-v", "targetDatabaseName=" + dbRequest.DatabaseName,
		"-i", curDir + "\\sql\\mssql\\import_dump.sql"}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		logger.Error("Dump import seems to have failed:\n> stdout:\n'%s'\n> stderr:\n'%s'\n> exitCode: %d", res.stdout, res.stderr, res.exitCode)

		return fmt.Errorf("import failed with exitcode '%q'", res.exitCode)
	}

	return nil
}

func (db *mssql) ExportDatabase(dbRequest model.DBRequest) (string, error) {
	//fullDumpFilename := fmt.Sprintf("%s_%s.dmp", dbRequest.DatabaseName, time.Now().Format("20060102150405"))

	return "", fmt.Errorf("export not yet implemented for MSSQL")
}

func (db *mssql) ListDatabase() ([]string, error) {
	return nil, fmt.Errorf("operation not supported: ListDatabase")
}

func (db *mssql) Version() (string, error) {

	args := []string{"-b", "-h", "-1", "-W", "-U", conf.User, "-P", conf.Password, "-Q",
		"SET NOCOUNT ON; SELECT (CAST(SERVERPROPERTY('productversion') AS nvarchar(128)) + SPACE(1) + CAST(SERVERPROPERTY('productlevel') AS nvarchar(128)) + SPACE(1) + CAST(SERVERPROPERTY('edition') AS nvarchar(128)))"}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		logger.Error("Unable to get SQL Server version:\n> stdout:\n'%s'\n> stderr:\n'%s'\n> exitCode: %d", res.stdout, res.stderr, res.exitCode)

		return "", fmt.Errorf("getting version failed with exitcode '%d'", res.exitCode)
	}

	return strings.TrimSpace(res.stdout), nil
}

func (db *mssql) RequiredFields(dbreq model.DBRequest, reqType int) []string {
	req := []string{dbreq.DatabaseName}

	switch reqType {
	case createDB:
		req = append(req, dbreq.Password)
	case importDB:
		req = append(req, strconv.Itoa(dbreq.ID), dbreq.Password, dbreq.DumpLocation)
	}

	return req
}

func (db *mssql) ValidateDump(path string) (string, error) {
	return path, nil
}
