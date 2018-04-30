package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"syscall"

	"github.com/djavorszky/ddn/common/inet"
	"github.com/djavorszky/ddn/common/logger"
	"github.com/djavorszky/ddn/common/model"
	"github.com/djavorszky/notif"
)

const defaultFailedCode = 1

// RunCommand executes a command with specified arguments and returns its exitcode, stdout
// and stderr as well.
func RunCommand(name string, args ...string) CommandResult {
	var (
		outbuf, errbuf bytes.Buffer
		exitCode       int
	)

	logger.Debug("Running command: %s %s", name, args)

	cmd := exec.Command(name, args...)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Run()
	stdout := outbuf.String()
	stderr := errbuf.String()

	if err != nil {
		// try to get the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			// This will happen (in OSX) if `name` is not available in $PATH,
			// in this situation, exit code could not be get, and stderr will be
			// empty string very likely, so we use the default fail code, and format err
			// to string and set to stderr
			logger.Error("Could not get exit code for failed program: %v, %v", name, args)

			exitCode = defaultFailedCode

			if stderr == "" {
				stderr = err.Error()
			}
		}
	} else {
		// success, exitCode should be 0 if go is ok
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}

	stdout = strings.TrimSuffix(stdout, "\n")
	stderr = strings.TrimSuffix(stderr, "\n")

	return CommandResult{stdout, stderr, exitCode}
}

// CommandResult is a struct that contains the stdout, stderr and exitcode
// of a command that was executed.
type CommandResult struct {
	stdout, stderr string
	exitCode       int
}

func registerAgent() error {
	endpoint := fmt.Sprintf("%s/%s", conf.MasterAddress, "heartbeat")

	if !inet.AddrExists(endpoint) {
		return fmt.Errorf("master server does not exist at given endpoint")
	}

	longname := fmt.Sprintf("%s %s", conf.Vendor, conf.Version)

	ddnc := model.RegisterRequest{
		AgentName: conf.AgentName,
		ShortName: conf.ShortName,
		LongName:  longname,
		Version:   version,
		DBVendor:  conf.Vendor,
		DBPort:    conf.AgentDBPort,
		DBAddr:    conf.AgentDBHost,
		DBSID:     conf.SID,
		Port:      conf.AgentPort,
		Addr:      conf.AgentAddr,
	}

	register := fmt.Sprintf("%s/%s", conf.MasterAddress, "register")

	resp, err := notif.SndLoc(ddnc, register)
	if err != nil {
		return fmt.Errorf("register: %v", err)
	}

	agent = model.Agent{
		ShortName:  conf.ShortName,
		LongName:   longname,
		Identifier: conf.AgentName,
		Version:    version,
		Up:         true,
	}
	err = json.NewDecoder(bytes.NewBufferString(resp)).Decode(&agent)
	if err != nil {
		logger.Fatal("response decoding: %v", err)
	}

	registered = true

	logger.Info("Registered with master server. Got assigned ID '%d'", agent.ID)

	return nil
}

func unregisterAgent() {
	agent.Up = false

	unregister := fmt.Sprintf("%s/%s", conf.MasterAddress, "unregister")
	_, err := notif.SndLoc(agent, unregister)
	if err != nil {
		logger.Fatal("unregister: %v", err)
	}

	log.Fatalf("Successfully unregistered the agent.")
}
