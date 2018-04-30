package main

import (
	"database/sql"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/djavorszky/ddn/common/logger"
	"github.com/djavorszky/ddn/common/model"
)

type oracle struct {
	conn *sql.DB
}

func (db *oracle) Connect(c Config) error {
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
		fmt.Sprintf("%s/%s", conf.User, conf.Password),
		"@./sql/oracle/create_schema.sql",
		dbRequest.Username,
		dbRequest.Password,
		conf.DatafileDir,
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
		fmt.Sprintf("%s/%s", conf.User, conf.Password),
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

	args := []string{
		"-L",
		"-S",
		fmt.Sprintf("%s/%s", conf.User, conf.Password),
		"@./sql/oracle/import_dump.sql",
		dumpDir,
		fileName,
		dbRequest.Username,
		dbRequest.Password,
		conf.DatafileDir,
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
		fmt.Sprintf("%s/%s", conf.User, conf.Password),
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
		fmt.Sprintf("%s/%s", conf.User, conf.Password),
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
		fmt.Sprintf("%s/%s", conf.User, conf.Password),
		"@./sql/oracle/import_procedure.sql",
	}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		logger.Error("Missing grants from SYS perhaps?")
		logger.Error("sgrant select on dba_datapump_jobs to %s;", conf.User)
		logger.Error("%sgrant create any directory to %s;\n", conf.User)
		logger.Error("%sgrant create external job to %s;\n", conf.User)

		return fmt.Errorf("creating import procedure failed: %v", res)
	}

	return nil
}

func (db *oracle) CreateExpDir(expDirPath string) error {
	args := []string{"-L", "-S", fmt.Sprintf("%s/%s", conf.User, conf.Password), "@./sql/oracle/create_exp_dir.sql", expDirPath}

	res := RunCommand(conf.Exec, args...)

	if res.exitCode != 0 {
		return fmt.Errorf("creating EXP_DIR directory failed: %v", res)
	}

	return nil
}
