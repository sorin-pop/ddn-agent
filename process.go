package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/djavorszky/ddn/common/inet"
	"github.com/djavorszky/ddn/common/logger"
	"github.com/djavorszky/ddn/common/model"
	"github.com/djavorszky/ddn/common/status"
	"github.com/djavorszky/ddn/server/brwsr"
	"github.com/djavorszky/notif"
)

func startImport(dbreq model.DBRequest) {
	upd8Path := fmt.Sprintf("%s/%s", conf.MasterAddress, "upd8")

	ch := notif.New(dbreq.ID, upd8Path)
	defer close(ch)

	ch <- notif.Y{StatusCode: status.DownloadInProgress, Msg: "Downloading dump"}
	logger.Debug("Downloading dump from %q", dbreq.DumpLocation)

	path, err := inet.DownloadFile("dumps", dbreq.DumpLocation)
	if err != nil {
		db.DropDatabase(dbreq)
		logger.Error("could not download file: %v", err)

		ch <- notif.Y{StatusCode: status.DownloadFailed, Msg: "Downloading file failed: " + err.Error()}
		return
	}
	defer os.Remove(path)

	if isArchive(path) {
		ch <- notif.Y{StatusCode: status.ExtractingArchive, Msg: "Extracting archive"}

		logger.Debug("Extracting archive: %v", path)

		var (
			files []string
			err   error
		)

		switch filepath.Ext(path) {
		case ".zip":
			files, err = unzip(path)
		case ".gz":
			files, err = ungzip(path)
		case ".bz2":
			files, err = unbzip2(path)
		case ".tar":
			files, err = untar(path)
		default:
			db.DropDatabase(dbreq)
			logger.Error("import process stopped; encountered unsupported archive")

			ch <- notif.Y{StatusCode: status.ArchiveNotSupported, Msg: "archive not supported"}
			return
		}
		for _, f := range files {
			defer os.Remove(f)
		}

		if err != nil {
			db.DropDatabase(dbreq)
			logger.Error("could not extract archive: %v", err)

			ch <- notif.Y{StatusCode: status.ExtractingArchiveFailed, Msg: "Extracting file failed: " + err.Error()}
			return
		}

		if len(files) > 1 {
			db.DropDatabase(dbreq)
			logger.Error("import process stopped; more than one file found in archive")

			ch <- notif.Y{StatusCode: status.MultipleFilesInArchive, Msg: "Archive contains more than one file, import stopped"}
			return
		}

		path = files[0]
	}

	logger.Debug("Validating dump: %s", path)

	ch <- notif.Y{StatusCode: status.ValidatingDump, Msg: "Validating dump"}
	path, err = db.ValidateDump(path)
	if err != nil {
		db.DropDatabase(dbreq)
		logger.Error("database validation failed: %v", err)

		ch <- notif.Y{StatusCode: status.ValidationFailed, Msg: "Validating dump failed: " + err.Error()}
		return
	}

	if !strings.Contains(path, "dumps") {
		oldPath := path
		path = "dumps" + string(os.PathSeparator) + path

		os.Rename(oldPath, path)
	}

	path, _ = filepath.Abs(path)
	defer os.Remove(path)

	dbreq.DumpLocation = path

	logger.Debug("Importing dump: %v", path)
	ch <- notif.Y{StatusCode: status.ImportInProgress, Msg: "Importing"}

	start := time.Now()

	err = db.ImportDatabase(dbreq)
	if err != nil {
		logger.Error("could not import database: %v", err)

		ch <- notif.Y{StatusCode: status.ImportFailed, Msg: "Importing dump failed: " + err.Error()}
		return
	}

	logger.Debug("Import succeded in %v", time.Since(start))
	ch <- notif.Y{StatusCode: status.Success, Msg: "Completed"}
}

func startExport(dbreq model.DBRequest) {
	upd8Path := fmt.Sprintf("%s/%s", conf.MasterAddress, "upd8")

	ch := notif.New(dbreq.ID, upd8Path)
	defer close(ch)

	logger.Debug("Exporting database: %v", dbreq.DatabaseName)
	ch <- notif.Y{StatusCode: status.ExportInProgress, Msg: "Exporting"}

	start := time.Now()

	fullDumpFilename, err := db.ExportDatabase(dbreq)
	if err != nil {
		logger.Error("could not export database: %v", err)

		ch <- notif.Y{StatusCode: status.ExportFailed, Msg: "Exporting database failed: " + err.Error()}
		return
	}

	// Archive (zip) the created dump file
	ch <- notif.Y{StatusCode: status.ArchivingDump, Msg: "Zipping dump"}
	logger.Debug("Zipping dump file: %v", fullDumpFilename)

	inputFiles := []string{filepath.Join(".", "exports", fullDumpFilename)}
	outputZipFilename := fmt.Sprintf("%s.zip", strings.TrimSuffix(fullDumpFilename, path.Ext(fullDumpFilename)))

	err = zipFiles(filepath.Join(".", "exports", outputZipFilename), inputFiles)

	if err != nil {
		logger.Error("could not zip dump file: %v", err)

		ch <- notif.Y{StatusCode: status.ZippingDumpFailed, Msg: "Zipping dump failed: " + err.Error()}
		os.Remove(filepath.Join(".", "exports", outputZipFilename))
		os.Remove(inputFiles[0])
		return
	}

	os.Remove(inputFiles[0])

	logger.Debug("Export succeeded in %v", time.Since(start))
	ch <- notif.Y{StatusCode: status.Success, Msg: "Export completed:" + outputZipFilename}
}

// This method should always be called asynchronously
func keepAlive() {
	endpoint := fmt.Sprintf("%s/%s/%s", conf.MasterAddress, "alive", conf.ShortName)

	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		// Check if the endpoint is up
		if !inet.AddrExists(fmt.Sprintf("%s/%s", conf.MasterAddress, "heartbeat")) {
			if registered {
				logger.Error("Lost connection to master server, will attempt to reconnect once it's back.")

				registered = false
			}

			continue
		}

		// If it is, check if we're not registered
		if !registered {
			logger.Info("Master server back online.")

			err := registerAgent()
			if err != nil {
				logger.Error("couldn't register with master: %v", err)
			}

			registered = true
		}

		respCode := inet.GetResponseCode(endpoint)
		if respCode == http.StatusOK {
			continue
		}

		// response is not "OK", so we need to register
		err := registerAgent()
		if err != nil {
			logger.Error("couldn't register with master: %v", err)
		}
	}
}

func checkExports() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		files, err := brwsr.List(filepath.Join(workdir, "exports"))
		if err != nil {
			logger.Error("failed to list exports directory: %v", err)
		}

		for _, file := range files.Entries {
			info, _ := os.Stat(file.Path)
			duration := time.Since(info.ModTime())

			// Removing file in 72 hours to make sure that if a database has been
			// exported on a Friday that it will still be available on Monday.
			if duration.Hours() > 72 {
				logger.Debug("Removing %v", info.Name())
				err := os.Remove(info.Name())
				if err != nil {
					logger.Error("couldn't remove file %s: %v", info.Name(), err)
				}
			}
		}
	}
}
