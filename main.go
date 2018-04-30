package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/djavorszky/ddn-common/logger"
	"github.com/djavorszky/ddn-common/model"
)

const version = "3"

var (
	conf       Config
	db         Database
	port       string
	hostname   string
	startup    time.Time
	registered bool
	workdir    string
	agent      model.Agent
)

func main() {
	defer func() {
		if p := recover(); p != nil {
			logger.Error("Panic... Unregistering")
			unregisterAgent()
		}
	}()

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		unregisterAgent()
		os.Exit(1)
	}()

	logger.Level = logger.INFO

	var err error
	confLocation := flag.String("p", "env", "Specify whether to read a configuration from a file (e.g. server.conf) or from environment variables.")
	logname := flag.String("l", "std", "Specify the log's filename. If set to std, logs to the terminal.")

	flag.Parse()

	err = loadProperties(*confLocation)
	if err != nil {
		logger.Fatal("Failed loading configuration: %v", err)
	}

	if _, err := os.Stat(conf.Exec); os.IsNotExist(err) {
		logger.Fatal("database executable doesn't exist: %v", conf.Exec)
	}

	if *logname != "std" {
		if _, err := os.Stat(*logname); err == nil {
			rotated := fmt.Sprintf("%s.%s", *logname, time.Now().Format("2006-01-02_03:04"))
			logger.Debug("Rotating logfile to %s", rotated)

			os.Rename(*logname, rotated)
		}

		logOut, err := os.OpenFile(*logname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Printf("error opening file %s, will continue logging to stderr: %s", *logname, err.Error())
			logOut = os.Stderr
		}
		defer logOut.Close()

		log.SetOutput(logOut)
	}

	hostname, err = os.Hostname()
	if err != nil {
		logger.Fatal("couldn't get hostname: ", err.Error())
	}

	logger.Debug("Hostname: %s", hostname)

	db, err = GetDB(conf.Vendor)
	if err != nil {
		logger.Fatal("couldn't get database instance:", err)
	}

	logger.Info("Starting with properties:")
	conf.Print()

	err = db.Connect(conf)
	if err != nil {
		logger.Fatal("couldn't establish database connection: %v", err)
	}
	defer db.Close()
	logger.Info("Database connection established")

	ver, err := db.Version()
	if err != nil {
		logger.Fatal("database: %v", err)
	}

	if ver != conf.Version {
		logger.Warn("Database version mismatch: Config: %q, Actual: %q", conf.Version, ver)

		conf.Version = ver
	}

	workdir, err = os.Getwd()
	if err != nil {
		logger.Fatal("could not determine current directory")
	}

	// Check and create the 'dumps' folder
	dumps := filepath.Join(workdir, "dumps")
	if _, err = os.Stat(dumps); os.IsNotExist(err) {
		err = os.Mkdir(dumps, os.ModePerm)
		if err != nil {
			logger.Fatal("Couldn't create dumps folder, please create it manually: %v", err)
		}

		logger.Info("Created 'dumps' folder")
	}

	// Check and create the 'exports' folder
	exports := filepath.Join(workdir, "exports")
	if _, err = os.Stat(exports); os.IsNotExist(err) {
		err = os.Mkdir(exports, os.ModePerm)
		if err != nil {
			logger.Fatal("Couldn't create 'exports' folder, please create it manually: %v", err)
		}

		logger.Info("Created 'exports' folder")
	}

	// For Oracle, create or replace the stored procedure that executes the import, by running the sql/oracle/import_procedure.sql file
	if odb, ok := db.(*oracle); ok {
		logger.Info("Creating or replacing the import_dump stored procedure.")
		err := odb.RefreshImportStoredProcedure()
		if err != nil {
			logger.Fatal("oracle: %v", err)
		}

		logger.Info("Creating or replacing the EXP_DIR directory object.")
		err = odb.CreateExpDir(filepath.Join(workdir, "exports"))
		if err != nil {
			logger.Fatal("Error creating or replacing the EXP_DIR directory object: %v", err)
		}
	}

	err = registerAgent()
	if err != nil {
		logger.Error("Could not register agent, will keep trying: %s", err.Error())
	}

	go keepAlive()
	go checkExports()

	sl := strings.Split(conf.AgentAddr, ":")

	port := sl[len(sl)-1]

	logger.Info("Starting to listen on %s", conf.AgentAddr)

	port = fmt.Sprintf(":%s", port)

	startup = time.Now()

	logger.Debug("Started up at %s", startup.Round(time.Millisecond))

	logger.Fatal("server: %v", http.ListenAndServe(port, Router()))
}

func loadProperties(confLocation string) error {
	if confLocation != "env" {
		return loadPropertiesFromFile(confLocation)
	}

	return loadPropertiesFromEnv()
}

func loadPropertiesFromFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file doesn't exist: %s", filename)
	}

	if _, err := toml.DecodeFile(filename, &conf); err != nil {
		return fmt.Errorf("couldn't read configuration file: %v", err)
	}

	return nil
}

const (
	envDBVendor          = "DB_VENDOR"
	envDBExecutable      = "DB_EXECUTABLE"
	envDBUser            = "DB_USER"
	envDBPass            = "DB_PASSWORD"
	envOracleSID         = "ORACLE_SID"
	envOracleDatafileDir = "ORACLE_DATAFILES_PATH"
	envDBLocalAddress    = "DB_LOCAL_ADDRESS"
	envDBRemoteAddress   = "DB_REMOTE_ADDRESS"
	envAgentAddress      = "AGENT_ADDRESS"
	envAgentName         = "AGENT_NAME"
	envServerAddress     = "SERVER_ADDRESS"
)

func loadPropertiesFromEnv() error {
	dbVendor, err := loadRequiredProperty(envDBVendor)
	if err != nil {
		return err
	}
	conf.Vendor = dbVendor

	executor, err := loadRequiredProperty(envDBExecutable)
	if err != nil {
		return err
	}
	conf.Exec = executor

	dbUser, err := loadRequiredProperty(envDBUser)
	if err != nil {
		return err
	}
	conf.User = dbUser

	conf.Password = loadOptionalProperty(envDBPass)

	if conf.Vendor == "oracle" {
		sid, err := loadRequiredProperty(envOracleSID)
		if err != nil {
			return err
		}
		conf.SID = sid

		datafileDir, err := loadRequiredProperty(envOracleDatafileDir)
		if err != nil {
			return err
		}
		conf.DatafileDir = datafileDir
	}

	remoteAddr, err := loadRequiredProperty(envDBRemoteAddress)
	if err != nil {
		return err
	}
	conf.AgentDBHost = remoteAddr

	localAddr := loadOptionalProperty(envDBLocalAddress)
	if localAddr == "" {
		localAddr = remoteAddr
	}
	conf.LocalDBAddr = localAddr

	agentAddr, err := loadRequiredProperty(envAgentAddress)
	if err != nil {
		return err
	}
	conf.AgentAddr = agentAddr

	agentName, err := loadRequiredProperty(envAgentName)
	if err != nil {
		return err
	}
	conf.AgentName = agentName
	conf.ShortName = agentName

	serverAddress, err := loadRequiredProperty(envServerAddress)
	if err != nil {
		return err
	}
	conf.MasterAddress = serverAddress

	return nil
}

func loadRequiredProperty(key string) (string, error) {
	val, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("required environment variable missing: %s", key)
	}

	return val, nil
}

func loadOptionalProperty(key string) string {
	return os.Getenv(key)
}
