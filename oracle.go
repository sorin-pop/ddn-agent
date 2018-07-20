package main

import (
	"database/sql"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/djavorszky/ddn-common/logger"
	"github.com/djavorszky/ddn-common/model"
)

type oracle struct {
	conn *sql.DB
}

func (db *oracle) Connect(c Config) error {
	if c.SysPassword != "" {
		logger.Info("Granting roles to %s", conf.User)
		err := db.doGrants()
		if err != nil {
			return fmt.Errorf("failed to connect to database: %v", err)
		}
	}

	args := []string{
		"-L",
		"-S",
		db.getConnectArg(),
		"SELECT 1;",
	}

	res := RunCommand(c.Exec, args...)

	if res.exitCode != 0 {
		return fmt.Errorf("failed to connect to database: %v", res)
	}

	return nil
}

func (db *oracle) doGrants() error {
	args := []string{
		db.getConnectString("sys", conf.SysPassword),
		"@./sql/oracle/grant_user.sql",
		conf.User,
	}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		return fmt.Errorf("failed granting roles to %s: %v", conf.User, res)
	}

	return nil
}

func (db *oracle) Close() {
	db.conn.Close()
}

func (db *oracle) Alive() error {
	return nil
}

func (db *oracle) CreateDatabase(dbRequest model.DBRequest) error {
	err := db.Alive()
	if err != nil {
		return fmt.Errorf("alive check failed: %s", err.Error())
	}

	args := []string{
		"-L",
		"-S",
		db.getConnectArg(),
		"@./sql/oracle/create_schema.sql",
		dbRequest.Username,
		dbRequest.Password,
		appendFileSeparator(conf.DatafileDir),
	}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode == 1920 {
		return fmt.Errorf("user/schema %s already exists", dbRequest.Username)
	}

	if res.exitCode != 0 {
		return fmt.Errorf("unable to create database: %v", res)
	}

	return nil
}

func (db *oracle) DropDatabase(dbRequest model.DBRequest) error {
	args := []string{
		"-L",
		"-S",
		db.getConnectArg(),
		"@./sql/oracle/drop_schema.sql",
		dbRequest.Username,
	}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode == 1918 { // ORA-01918: user xxx does not exist ---> return with success
		return nil
	}

	if res.exitCode != 0 {
		return fmt.Errorf("unable to drop database: %v", res)
	}

	return nil
}

func (db *oracle) ImportDatabase(dbRequest model.DBRequest) error {
	dumpDir, fileName := filepath.Split(dbRequest.DumpLocation)

	if conf.RemoteDumpsDir != "" {
		dumpDir = conf.RemoteDumpsDir
	}

	args := []string{
		"-L",
		"-S",
		db.getConnectArg(),
		"@./sql/oracle/import_dump.sql",
		dumpDir,
		fileName,
		dbRequest.Username,
		dbRequest.Password,
		appendFileSeparator(conf.DatafileDir),
	}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		return fmt.Errorf("dump import seems to have failed: %v", res)
	}

	return nil
}

func (db *oracle) ExportDatabase(dbRequest model.DBRequest) (string, error) {
	fullDumpFilename := fmt.Sprintf("%s_%s.dmp", dbRequest.DatabaseName, time.Now().Format("20060102150405"))
	// Start the export
	args := []string{
		db.getConnectArg(),
		fmt.Sprintf("schemas=%s", dbRequest.DatabaseName),
		"directory=EXP_DIR",
		fmt.Sprintf("dumpfile=%s", fullDumpFilename),
		fmt.Sprintf("logfile=%s.log", strings.TrimSuffix(fullDumpFilename, path.Ext(fullDumpFilename))),
	}

	res := RunCommand("expdp", args...)

	if res.exitCode != 0 {
		return "", fmt.Errorf("schema export seems to have failed: %v", res)
	}

	return fullDumpFilename, nil
}

func (db *oracle) ListDatabase() ([]string, error) {
	return nil, nil
}

func (db *oracle) Version() (string, error) {
	args := []string{
		"-L",
		"-S",
		db.getConnectArg(),
		"@./sql/oracle/get_db_version.sql",
	}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		return "", fmt.Errorf("unable to get Oracle version: %v", res)
	}

	return strings.TrimSpace(res.stdout), nil
}

func (db *oracle) RequiredFields(dbreq model.DBRequest, reqType int) []string {
	req := []string{dbreq.Username}

	switch reqType {
	case createDB:
		req = append(req, dbreq.Password)
	case importDB:
		req = append(req, strconv.Itoa(dbreq.ID), dbreq.Password, dbreq.DumpLocation)
	}

	return req
}

func (db *oracle) ValidateDump(path string) (string, error) {
	return path, nil
}

func (db *oracle) RefreshImportStoredProcedure() error {
	args := []string{
		"-L",
		"-S",
		db.getConnectArg(),
		"@./sql/oracle/import_procedure.sql",
	}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		logger.Error("Missing grants from SYS perhaps?")
		logger.Error("grant select on dba_datapump_jobs to %s;", conf.User)
		logger.Error("grant create any directory to %s;", conf.User)
		logger.Error("grant create external job to %s;", conf.User)

		return fmt.Errorf("creating import procedure failed: %v", res)
	}

	return nil
}

func (db *oracle) CreateExpDir(expDirPath string) error {
	args := []string{
		"-L",
		"-S",
		db.getConnectArg(),
		"@./sql/oracle/create_exp_dir.sql",
		expDirPath}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		return fmt.Errorf("creating EXP_DIR directory failed: %v", res)
	}

	return nil
}

func (db *oracle) getConnectArg() string {
	connect := db.getConnectString(conf.User, conf.Password)

	logger.Debug("Oracle connection argument: %s", connect)

	return connect
}

func (db *oracle) getConnectString(user, password string) string {
	hostAndPort := strings.Split(conf.LocalDBAddr, ":")

	host := hostAndPort[0]
	port := hostAndPort[1]

	res := fmt.Sprintf("%s/%s@'(DESCRIPTION=(ADDRESS=(PROTOCOL=tcp)(HOST=%s)(PORT=%s))(CONNECT_DATA=(SERVICE_NAME=%s)))'",
		user,
		password,
		host,
		port,
		conf.SID,
	)

	if user == "sys" {
		res += " as sysdba"
	}

	return res
}

func appendFileSeparator(path string) string {
	if strings.Contains(path, "\\") {
		if !strings.HasSuffix(path, "\\") {
			path += "\\"
		}
	}

	if strings.Contains(path, "/") {
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
	}
	return path
}
