package cfbackup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	cfhttp "github.com/pivotalservices/gtils/http"
	"github.com/pivotalservices/gtils/osutils"
)

const (
	ER_DEFAULT_SYSTEM_USER string = "vcap"
	ER_DIRECTOR_INFO_URL   string = "https://%s:25555/info"
	ER_BACKUP_DIR          string = "elasticruntime"
	ER_VMS_URL             string = "https://%s:25555/deployments/%s/vms"
	ER_DIRECTOR            string = "DirectorInfo"
	ER_CONSOLE             string = "ConsoledbInfo"
	ER_UAA                 string = "UaadbInfo"
	ER_CC                  string = "CcdbInfo"
	ER_MYSQL               string = "MysqldbInfo"
	ER_NFS                 string = "NfsInfo"
	ER_BACKUP_FILE_FORMAT  string = "%s.backup"
)

// ElasticRuntime contains information about a Pivotal Elastic Runtime deployment
type ElasticRuntime struct {
	JsonFile          string
	SystemsInfo       map[string]SystemDump
	PersistentSystems []SystemDump
	HttpGateway       cfhttp.HttpGateway
	InstallationName  string
	BackupContext
}

// NewElasticRuntime initializes an ElasticRuntime intance
var NewElasticRuntime = func(jsonFile string, target string) *ElasticRuntime {
	var (
		uaadbInfo *PgInfo = &PgInfo{
			SystemInfo: SystemInfo{
				Product:   "cf",
				Component: "uaadb",
				Identity:  "root",
			},
		}
		consoledbInfo *PgInfo = &PgInfo{
			SystemInfo: SystemInfo{
				Product:   "cf",
				Component: "consoledb",
				Identity:  "root",
			},
		}
		ccdbInfo *PgInfo = &PgInfo{
			SystemInfo: SystemInfo{
				Product:   "cf",
				Component: "ccdb",
				Identity:  "admin",
			},
		}
		mysqldbInfo *MysqlInfo = &MysqlInfo{
			SystemInfo: SystemInfo{
				Product:   "cf",
				Component: "mysql",
				Identity:  "root",
			},
		}
		directorInfo *SystemInfo = &SystemInfo{
			Product:   "microbosh",
			Component: "director",
			Identity:  "director",
		}
		nfsInfo *NfsInfo = &NfsInfo{
			SystemInfo: SystemInfo{
				Product:   "cf",
				Component: "nfs_server",
				Identity:  "vcap",
			},
		}
	)

	context := &ElasticRuntime{
		JsonFile: jsonFile,
		BackupContext: BackupContext{
			TargetDir: target,
		},
		SystemsInfo: map[string]SystemDump{
			ER_DIRECTOR: directorInfo,
			ER_CONSOLE:  consoledbInfo,
			ER_UAA:      uaadbInfo,
			ER_CC:       ccdbInfo,
			ER_MYSQL:    mysqldbInfo,
			ER_NFS:      nfsInfo,
		},
		PersistentSystems: []SystemDump{
			consoledbInfo,
			uaadbInfo,
			ccdbInfo,
			nfsInfo,
			mysqldbInfo,
		},
	}
	return context
}

// Backup performs a backup of a Pivotal Elastic Runtime deployment
func (context *ElasticRuntime) Backup() (err error) {
	log.Println("Entering Backup() function")
	var (
		ccStop  *CloudController
		ccStart *CloudController
		ccJobs  []string
	)

	if err = context.ReadAllUserCredentials(); err == nil && context.directorCredentialsValid() {
		log.Println("Retrieving All CC VMs")
		if ccJobs, err = context.getAllCloudControllerVMs(); err == nil {
			log.Println("Setting up CC jobs")
			directorInfo := context.SystemsInfo[ER_DIRECTOR]
			ccStop = NewCloudController(directorInfo.Get(SD_IP), directorInfo.Get(SD_USER), directorInfo.Get(SD_PASS), context.InstallationName, "stopped", nil)
			ccStart = NewCloudController(directorInfo.Get(SD_IP), directorInfo.Get(SD_USER), directorInfo.Get(SD_PASS), context.InstallationName, "started", nil)
			defer ccStart.ToggleJobs(CloudControllerJobs(ccJobs))
			ccStop.ToggleJobs(CloudControllerJobs(ccJobs))
		} else {
			log.Fatal(err)
		}
		log.Println("Running RunDbBackups(...)")
		err = context.RunDbBackups(context.PersistentSystems)

	} else if err == nil {
		err = fmt.Errorf("invalid director credentials")
	}
	return
}

// Restore performs a restore of a Pivotal Elastic Runtime deployment
func (context *ElasticRuntime) Restore() (err error) {
	return
}

func (context *ElasticRuntime) getAllCloudControllerVMs() (ccvms []string, err error) {

	log.Println("Entering getAllCloudControllerVMs() function")
	directorInfo := context.SystemsInfo[ER_DIRECTOR]
	connectionURL := fmt.Sprintf(ER_VMS_URL, directorInfo.Get(SD_IP), context.InstallationName)
	gateway := context.HttpGateway
	if gateway == nil {
		gateway = cfhttp.NewHttpGateway(connectionURL, directorInfo.Get(SD_USER), directorInfo.Get(SD_PASS), "application/json", nil)
	}

	log.Println("Retrieving CC vms")
	if body, err := gateway.Execute("GET"); err == nil {
		var jsonObj []VMObject

		log.Println("Unmarshalling CC vms")
		contents := body.(*bytes.Buffer)
		if err = json.Unmarshal(contents.Bytes(), &jsonObj); err == nil {
			ccvms, err = GetCCVMs(jsonObj)
			if err != nil {
				log.Fatalf("Error unmarshalling ccvms.", err)
			}
		} else {
			log.Fatalf("Error unmarshalling contents.", err)
		}
	}
	return
}

func (context *ElasticRuntime) RunDbBackups(dbInfoList []SystemDump) (err error) {
	log.Println("Entering RunDbBackups() function")

	for _, info := range dbInfoList {

		if err = info.Error(); err == nil {
			err = context.openWriterAndDump(info, context.TargetDir)
		}

		if err != nil {
			break
		}
	}
	return
}

func (context *ElasticRuntime) openWriterAndDump(dbInfo SystemDump, databaseDir string) (err error) {
	log.Println("Entering openWriterAndDump() function")
	var (
		outfile *os.File
	)
	filename := fmt.Sprintf(ER_BACKUP_FILE_FORMAT, dbInfo.Get(SD_COMPONENT))

	if outfile, err = osutils.SafeCreate(databaseDir, filename); err == nil {
		err = context.dump(outfile, dbInfo)
	}
	return
}

func (context *ElasticRuntime) dump(dest io.Writer, s SystemDump) (err error) {
	log.Println("Entering dump() function")
	var dumper PersistanceBackup

	if dumper, err = s.GetPersistanceBackup(); err == nil {
		err = dumper.Dump(dest)
	}
	return
}

func (context *ElasticRuntime) ReadAllUserCredentials() (err error) {
	var (
		fileRef *os.File
		jsonObj InstallationCompareObject
	)
	defer fileRef.Close()

	if fileRef, err = os.Open(context.JsonFile); err == nil {

		if jsonObj, err = ReadAndUnmarshal(fileRef); err == nil {
			err = context.assignCredentialsAndInstallationName(jsonObj)
		}
	}
	return
}

func (context *ElasticRuntime) assignCredentialsAndInstallationName(jsonObj InstallationCompareObject) (err error) {

	if err = context.assignCredentials(jsonObj); err == nil {
		context.InstallationName, err = GetDeploymentName(jsonObj)
	}
	return
}

func (context *ElasticRuntime) assignCredentials(jsonObj InstallationCompareObject) (err error) {

	for name, sysInfo := range context.SystemsInfo {
		var (
			ip    string
			pass  string
			vpass string
		)
		sysInfo.Set(SD_VCAPUSER, ER_DEFAULT_SYSTEM_USER)
		sysInfo.Set(SD_USER, sysInfo.Get(SD_IDENTITY))

		if ip, pass, err = GetPasswordAndIP(jsonObj, sysInfo.Get(SD_PRODUCT), sysInfo.Get(SD_COMPONENT), sysInfo.Get(SD_IDENTITY)); err == nil {
			sysInfo.Set(SD_IP, ip)
			sysInfo.Set(SD_PASS, pass)
			_, vpass, err = GetPasswordAndIP(jsonObj, sysInfo.Get(SD_PRODUCT), sysInfo.Get(SD_COMPONENT), sysInfo.Get(SD_VCAPUSER))
			sysInfo.Set(SD_VCAPPASS, vpass)
			context.SystemsInfo[name] = sysInfo
		}
	}
	return
}

func (context *ElasticRuntime) directorCredentialsValid() (ok bool) {
	var directorInfo SystemDump

	if directorInfo, ok = context.SystemsInfo[ER_DIRECTOR]; ok {
		connectionURL := fmt.Sprintf(ER_DIRECTOR_INFO_URL, directorInfo.Get(SD_IP))
		gateway := context.HttpGateway
		if gateway == nil {
			gateway = cfhttp.NewHttpGateway(connectionURL, directorInfo.Get(SD_USER), directorInfo.Get(SD_PASS), "application/json", nil)
		}
		_, err := gateway.Execute("GET")
		ok = (err == nil)
	}
	return
}
