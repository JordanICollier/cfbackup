package cfbackup

import (
	"os"
	"path"

	"github.com/pivotalservices/gtils/log"
)

const (
	BACKUP_LOGGER_NAME  = "Backup"
	RESTORE_LOGGER_NAME = "Restore"
)

// Tile is a deployable component that can be backed up
type Tile interface {
	Backup() error
	Restore() error
}

type BackupContext struct {
	TargetDir string
}

type action func() error

type actionAdaptor func(t Tile) action

func RunBackupPipeline(hostname, username, password, tempestpassword, destination string) (err error) {
	backup := func(t Tile) action {
		return t.Backup
	}
	return runPipelines(hostname, username, password, tempestpassword, destination, BACKUP_LOGGER_NAME, backup)
}

func RunRestorePipeline(hostname, username, password, tempestpassword, destination string) (err error) {
	restore := func(t Tile) action {
		return t.Restore
	}
	return runPipelines(hostname, username, password, tempestpassword, destination, RESTORE_LOGGER_NAME, restore)
}

func runPipelines(hostname, username, password, tempestpassword, destination, loggerName string, actionBuilder actionAdaptor) (err error) {
	var (
		opsmanager     Tile
		elasticRuntime Tile
	)
	backupLogger := log.LogFactory(loggerName, log.Lager, os.Stdout)
	installationFilePath := path.Join(destination, OPSMGR_BACKUP_DIR, OPSMGR_INSTALLATION_SETTINGS_FILENAME)

	if opsmanager, err = NewOpsManager(hostname, username, password, tempestpassword, destination, backupLogger); err == nil {
		elasticRuntime = NewElasticRuntime(installationFilePath, destination, backupLogger)
		tiles := []action{
			actionBuilder(opsmanager),
			actionBuilder(elasticRuntime),
		}
		err = runActions(tiles)
	}
	return
}

func runActions(actions []action) (err error) {
	for _, action := range actions {

		if err = action(); err != nil {
			break
		}
	}
	return
}
