package main

import (
	"fmt"
	"runtime"

	"github.com/djavorszky/ddn-common/logger"
)

// Config to hold the database server and agent information
type Config struct {
	Vendor        string `toml:"db-vendor"`
	Version       string `toml:"db-version"`
	Exec          string `toml:"db-executable"`
	User          string `toml:"db-username"`
	Password      string `toml:"db-userpass"`
	SID           string `toml:"oracle-sid"`
	DatafileDir   string `toml:"oracle-datafiles-path"`
	LocalDBAddr   string `toml:"db-local-addr"`
	AgentDBHost   string `toml:"db-remote-addr"`
	AgentAddr     string `toml:"agent-addr"`
	ShortName     string `toml:"agent-shortname"`
	AgentName     string `toml:"agent-longname"`
	MasterAddress string `toml:"server-address"`
}

// Print prints the Config object to the log.
func (c Config) Print() {
	logger.Info("Vendor:\t\t%s", conf.Vendor)
	logger.Info("Version:\t\t%s", conf.Version)
	logger.Info("Executable:\t\t%s", conf.Exec)

	logger.Info("Username:\t\t%s", conf.User)
	logger.Info("Password:\t\t****")

	if conf.Vendor == "oracle" {
		logger.Info("SID:\t\t%s", conf.SID)
		logger.Info("DatafileDir:\t\t%s", conf.DatafileDir)
	}

	logger.Info("Local DB addr:\t%s", conf.LocalDBAddr)

	logger.Info("Remote DB addr:\t%s", conf.AgentDBHost)

	logger.Info("Agent addr:\t%s", conf.AgentAddr)

	logger.Info("Short name:\t\t%s", conf.ShortName)
	logger.Info("Agent name:\t%s", conf.AgentName)

	logger.Info("Master address:\t%s", conf.MasterAddress)
}

// NewConfig returns a configuration file based on the vendor
func NewConfig(vendor string) Config {
	var conf Config

	switch vendor {
	case "mysql":
		conf = Config{
			Vendor:        "mysql",
			Version:       "5.5.53",
			ShortName:     "mysql-55",
			LocalDBAddr:   "localhost:3306",
			AgentAddr:     "http://localhost:7000",
			AgentDBHost:   "localhost:3306",
			User:          "root",
			Password:      "root",
			MasterAddress: "http://localhost:7010",
		}

		switch runtime.GOOS {
		case "windows":
			conf.Exec = "C:\\path\\to\\mysql.exe"
		case "darwin":
			conf.Exec = "/usr/local/mysql/bin/mysql"
		default:
			conf.Exec = "/usr/bin/mysql"
		}
	case "postgres":
		conf = Config{
			Vendor:        "postgres",
			Version:       "9.4.9",
			ShortName:     "postgre-94",
			LocalDBAddr:   "localhost:5432",
			AgentAddr:     "http://localhost:7000",
			AgentDBHost:   "localhost:5432",
			User:          "postgres",
			Password:      "password",
			MasterAddress: "http://localhost:7010",
		}

		switch runtime.GOOS {
		case "windows":
			conf.Exec = "C:\\path\\to\\psql.exe"
		case "darwin":
			conf.Exec = "/Library/PostgreSQL/9.4/bin/psql"
		default:
			conf.Exec = "/usr/bin/psql"
		}
	case "oracle":
		conf = Config{
			Vendor:        "oracle",
			Version:       "11g",
			ShortName:     "oracle-11g",
			LocalDBAddr:   "localhost:1521",
			AgentAddr:     "http://localhost:7000",
			AgentDBHost:   "localhost:1521",
			User:          "system",
			Password:      "password",
			SID:           "orcl",
			DatafileDir:   "",
			MasterAddress: "http://localhost:7010",
		}
		switch runtime.GOOS {
		case "windows":
			conf.Exec = "C:\\path\\to\\sqlplus.exe"
		case "darwin":
			conf.Exec = "/path/to/sqlplus"
		default:
			conf.Exec = "/path/to/sqlplus"
		}
	}

	conf.AgentName = fmt.Sprintf("%s-%s", hostname, conf.ShortName)

	return conf
}
