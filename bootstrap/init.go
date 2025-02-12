package bootstrap

import (
	"dst-admin-go/collect"
	"dst-admin-go/config"
	"dst-admin-go/config/database"
	"dst-admin-go/config/global"
	"dst-admin-go/model"
	"dst-admin-go/schedule"
	"dst-admin-go/service"
	"dst-admin-go/service/autoCheck"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gopkg.in/yaml.v2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

const logPath = "./dst-admin-go.log"

var f *os.File

func Init() {

	initConfig()
	initLog()
	initDB()
	initCollect()
	initSchedule()

	initUpdateModinfos()

	InitSnapshotBackup()
}

func initDB() {
	db, err := gorm.Open(sqlite.Open(global.Config.Db), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to connect database")
	}
	database.DB = db
	err = database.DB.AutoMigrate(
		&model.Spawn{},
		&model.PlayerLog{},
		&model.Connect{},
		&model.Regenerate{},
		&model.ModInfo{},
		&model.Cluster{},
		&model.JobTask{},
		&model.AutoCheck{},
		&model.Announce{},
		&model.WebLink{},
		&model.BackupSnapshot{},
		&model.LogRecord{},
		&model.KV{},
	)
	if err != nil {
		return
	}
}

func initConfig() {
	yamlFile, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		fmt.Println(err.Error())
	}
	var _config *config.Config
	err = yaml.Unmarshal(yamlFile, &_config)
	if err != nil {
		fmt.Println(err.Error())
	}
	if _config.AutoUpdateModinfo.UpdateCheckInterval == 0 {
		_config.AutoUpdateModinfo.UpdateCheckInterval = 10
	}
	if _config.AutoUpdateModinfo.CheckInterval == 0 {
		_config.AutoUpdateModinfo.CheckInterval = 5
	}
	log.Println("config: ", _config)
	global.Config = _config
}

func initLog() {
	var err error
	f, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}

	// 组合一下即可，os.Stdout代表标准输出流
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)

	gin.ForceConsoleColor()
	gin.SetMode(gin.DebugMode)
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func initCollect() {

	//var clusters []model.Cluster
	//database.DB.Find(&clusters)
	//for _, cluster := range clusters {
	//	global.CollectMap.AddNewCollect(cluster.ClusterName)
	//}

	// home, _ := systemUtils.Home()
	dstConfig := dstConfigUtils.GetDstConfig()
	clusterName := dstConfig.Cluster
	newCollect := collect.NewCollect(filepath.Join(dstUtils.GetKleiDstPath(), clusterName), clusterName)
	newCollect.StartCollect()
	global.Collect = newCollect

	// autoCheck.AutoCheckObject = autoCheck.NewAutoCheckConfig(clusterName, dstConfig.Bin, dstConfig.Beta)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		autoCheckManager := autoCheck.AutoCheckManager{}
		autoCheckManager.Start()
		autoCheck.Manager = &autoCheckManager
	}()
}

func initSchedule() {
	schedule.ScheduleSingleton = schedule.NewSchedule()
	// service.InitAnnounce()

}

func initUpdateModinfos() {
	if global.Config.AutoUpdateModinfo.Enable {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			t := global.Config.AutoUpdateModinfo.UpdateCheckInterval
			ticker := time.NewTicker(time.Duration(t) * time.Minute)
			for {
				select {
				case <-ticker.C:
					log.Println("正在定时更新模组配置 间隔: ", t, "分钟")
					// TODO 中英文问题，每隔10分钟执行的任务
					mod.UpdateModinfoList("zh")
				}
			}
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			t := global.Config.AutoUpdateModinfo.CheckInterval
			ticker := time.NewTicker(time.Duration(t) * time.Minute)
			for {
				select {
				case <-ticker.C:
					log.Println("正在定时检查模组配置是否更新 间隔: ", t, "分钟")
					// 每隔10分钟执行的任务
					mod.CheckModInfoUpdate()
				}
			}
		}()
	}
}

func InitSnapshotBackup() {
	var backupService service.BackupService
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		backupService.ScheduleBackupSnapshots()
	}()
}
