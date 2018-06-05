package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/djavorszky/ddn-common/logger"
	"github.com/djavorszky/ddn-common/model"
)

type mssql struct {
	conn *sql.DB
}

func (db *mssql) Connect(c Config) error {
	_, err := db.Version()
	if err != nil {
		return fmt.Errorf("connect: %v", err)
	}

	return nil
}

func (db *mssql) createUser(username, password string) error {
	connectArgs := db.getConnectArg()

	// sqlcmd on Linux does not support passing variables on the commandline, so we need to work it around.
	createQuery := fmt.Sprintf(`IF NOT EXISTS (SELECT name FROM [sys].[server_principals] WHERE name = '%[1]s')
	Begin
		CREATE LOGIN %[1]s WITH PASSWORD = '%[2]s';
		CREATE USER %[1]s FOR LOGIN %[1]s;
		GRANT ALL PRIVILEGES TO %[1]s;
		ALTER SERVER ROLE [dbcreator] ADD MEMBER [%[1]s];
	End
	GO`, username, password)

	args := append(connectArgs, "-Q", createQuery)

	res := RunCommand(conf.Exec, args...)
	if res.exitCode != 0 {
		logger.Error("unable to create user:\n> stdout:\n%q\n> stderr:\n%q\n> exitCode: %d", res.stdout, res.stderr, res.exitCode)

		return fmt.Errorf("create user failed with exitcode '%d'", res.exitCode)
	}

	return nil
}

func (db *mssql) Close() {
	// Not needed
}

// dummy return...
func (db *mssql) Alive() error {
	return nil
}

func (db *mssql) CreateDatabase(dbRequest model.DBRequest) error {
	err := db.createUser(dbRequest.Username, dbRequest.Password)
	if err != nil {
		return fmt.Errorf("create database: %v", err)
	}

	connectArgs := db.getConnectSlice(dbRequest.Username, dbRequest.Password)

	args := append(connectArgs, "-Q", fmt.Sprintf("CREATE DATABASE %s", dbRequest.DatabaseName))

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
	connectArgs := db.getConnectArg()

	args := append(connectArgs, "-Q", fmt.Sprintf("DROP DATABASE %s", dbRequest.DatabaseName))

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

	connectArgs := db.getConnectArg()

	args := append(connectArgs,
		"-v", "driveLetter="+driveLetter,
		"-v", "dumpPath="+dumpPath,
		"-v", "targetDatabaseName="+dbRequest.DatabaseName,
		"-i", curDir+"/sql/mssql/import_dump.sql")

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
	connectArgs := db.getConnectArg()

	args := append(connectArgs, "-h", "-1", "-W", "-Q",
		"SET NOCOUNT ON; SELECT (CAST(SERVERPROPERTY('productversion') AS nvarchar(128)) + SPACE(1) + CAST(SERVERPROPERTY('productlevel') AS nvarchar(128)) + SPACE(1) + CAST(SERVERPROPERTY('edition') AS nvarchar(128)))")

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		logger.Error("Unable to get SQL Server version:\n> stdout:\n%q\n> stderr:\n%q\n> exitCode: %d", res.stdout, res.stderr, res.exitCode)

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

func (db *mssql) getConnectArg() []string {
	connect := db.getConnectSlice(conf.User, conf.Password)

	logger.Debug("MSSQL connection argument: %s", connect)

	return connect
}

func (db *mssql) getConnectString(user, password string) string {
	hostAndPort := strings.Split(conf.LocalDBAddr, ":")

	host := hostAndPort[0]
	port := hostAndPort[1]

	res := fmt.Sprintf("-b -S tcp:%s,%s -U %s -P %s",
		host,
		port,
		user,
		password,
	)

	return res
}

func (db *mssql) getConnectSlice(user, password string) []string {
	hostAndPort := strings.Split(conf.LocalDBAddr, ":")

	host := hostAndPort[0]
	port := hostAndPort[1]

	res := []string{
		"-b",
		"-S",
		fmt.Sprintf("tcp:%s,%s", host, port),
		"-U",
		user,
		"-P",
		password,
	}

	return res
}
