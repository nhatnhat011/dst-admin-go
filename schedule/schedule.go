package schedule

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/service"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var ScheduleSingleton *Schedule

var StrategyMap = map[string]Strategy{}

var gameConsoleService = service.GameConsoleService{}

func init() {
	StrategyMap["backup"] = &BackupStrategy{}
	StrategyMap["update"] = &UpdateStrategy{}
	StrategyMap["start"] = &StartStrategy{}
	StrategyMap["stop"] = &StopStrategy{}
	StrategyMap["restart"] = &RestartStrategy{}
	StrategyMap["regenerate"] = &RegenerateStrategy{}

	StrategyMap["startGame"] = &StartGameStrategy{}
	StrategyMap["stopGame"] = &StopGameStrategy{}

	// 无动作
	StrategyMap["none"] = &NoneStrategy{}

}

type Task struct {
	Id           uint
	Corn         string
	F            func(string, string)
	ClusterName  string
	LevelName    string
	Announcement string
	Sleep        int
	Times        int
}

type Schedule struct {
	cron  *cron.Cron
	cache sync.Map
}

func NewSchedule() *Schedule {
	c := cron.New()
	schedule := Schedule{
		cron: c,
	}
	schedule.initDBTask()
	c.Start()
	return &schedule
}

func (s *Schedule) Stop() {
	s.cron.Stop()
}

func (s *Schedule) AddJob(task Task) {
	jobId, err := s.cron.AddFunc(task.Corn, func() {
		// 发送公告
		s.SendAnnouncement(task.ClusterName, task.Announcement, task.Sleep, task.Times)
		task.F(task.ClusterName, task.LevelName)
	})
	if err != nil {
		log.Panicln("创建任务失败，cron:", task.Corn, err)
	}
	s.cache.Store(jobId, task.Id)
}

func (s *Schedule) DeleteJob(jobId int) {
	taskId, loaded := s.cache.LoadAndDelete(cron.EntryID(jobId))
	if loaded {
		log.Println("找到 ", cron.EntryID(jobId))
		var entryId = cron.EntryID(jobId)
		s.cron.Remove(entryId)
		s.removeDB(taskId.(uint))
	} else {
		log.Println("未找到 ", cron.EntryID(jobId))
	}
}

func (s *Schedule) GetInstructList() []map[string]string {
	var instructList = []map[string]string{
		{"backup": "备份"},
		{"update": "更新"},
	}
	return instructList
}

func (s *Schedule) GetJobs() []map[string]interface{} {

	var results []map[string]interface{}
	entries := s.cron.Entries()
	log.Println("cron size: ", len(entries))
	for _, entry := range entries {
		taskId, _ := s.cache.Load(entry.ID)
		task := s.findDB(taskId.(uint))
		results = append(results, map[string]interface{}{
			"clusterName":  task.ClusterName,
			"levelName":    task.LevelName,
			"uuid":         task.Uuid,
			"jobId":        entry.ID,
			"next":         entry.Next,
			"prev":         entry.Prev,
			"valid":        entry.Valid(),
			"cron":         task.Cron,
			"comment":      task.Comment,
			"category":     task.Category,
			"announcement": task.Announcement,
		})
	}
	return results
}

func (s *Schedule) initDBTask() {
	// 从数据库中读取
	db := database.DB

	var jobTaskList []model.JobTask
	db.Find(&jobTaskList)

	for _, task := range jobTaskList {
		// 根据类型不同 执行不同的函数
		entryID, err := s.cron.AddFunc(task.Cron, func() {
			// 发送公告
			s.SendAnnouncement(task.ClusterName, task.Announcement, task.Sleep, task.Times)
			StrategyMap[task.Category].Execute(task.ClusterName, task.Uuid)
		})
		if err != nil {
			log.Println("初始化任务失败", err)
		}
		s.cache.Store(entryID, task.ID)
	}
}

func (s *Schedule) removeDB(taskId uint) {
	db := database.DB
	db.Where("ID = ?", taskId).Delete(&model.JobTask{})
}

func (s *Schedule) findDB(taskId uint) *model.JobTask {
	db := database.DB
	task := model.JobTask{}
	db.Where("ID = ?", taskId).First(&task)

	return &task
}

func (s *Schedule) SendAnnouncement(clusterName string, announcement string, sleep int, times int) {
	if announcement == "" {
		return
	}
	for i := 0; i < times; i++ {
		log.Println("开始发送公告")
		lines := strings.Split(announcement, "\n")
		log.Println(lines)
		for i := range lines {
			gameConsoleService.SentBroadcast(clusterName, lines[i])
		}
		time.Sleep(time.Duration(sleep) * time.Second)
		log.Println("结束发送公告")
	}
}
