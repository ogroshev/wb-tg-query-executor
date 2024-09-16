package handler

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/postgres"
	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/telegram"
)

func DoWork (db *sql.DB) {
    tasks, err := postgres.GetTaskList(db)
    if err != nil {
        log.Printf("could not get tasks: %s", err)
    }

	for _, t := range tasks {		
		if isItTimeToWork(t.Id, t.ScheduleCron, t.LastExecutionTs) {
			log.Printf("Handle task id: %d. desc: %s", t.Id, t.ChatDescribe)
			if handleTask(db, t) {
				t.LastExecutionTs = sql.NullTime{Time: time.Now(), Valid: true}
				postgres.UpdateLastExecutionTs(db, t)
				log.Printf("id: %d last_execution_ts updated to: %s\n", t.Id, t.LastExecutionTs.Time.String())					
			}
		}
	}
}

func isItTimeToWork(id uint64, cronStr string, lastExecTs sql.NullTime) bool {
	schedule, err := cron.ParseStandard(cronStr)
	if err != nil {
		log.Printf("id: %d Could not parse cron string '%s'. error: %s", id, cronStr, err)
		return false
	}
	var lastExecTime time.Time
	if lastExecTs.Valid {
		lastExecTime = lastExecTs.Time
	}
	
	// log.Printf("id: %d last: %s\n", id, lastExecTime.String())
	nextTime := schedule.Next(lastExecTime)
	// log.Printf("id: %d next: %s\n", id, nextTime.String())
	// log.Printf("id: %d now : %s\n", id, time.Now().String())
	now := time.Now()
	diff := nextTime.Sub(now)
	log.Printf("id: %d. Next run: %s left: %s", id, nextTime, diff.String())
	return time.Now().After(nextTime)
}

func handleTask(db *sql.DB, task postgres.Task) bool {
	log.Printf("Executing query...\n")
	resultNotEmpty, resultPretty, err := postgres.ExecQuery(db, task.SqlQuery)
	log.Printf("Executing query... OK\n")	
	if err != nil {
		log.Println(err)
		return false
	}
	if resultNotEmpty {
		msg := makeMessage(task.ChatDescribe, resultPretty, task.Settings.Preformatted, task.Settings.AddHeader)
		log.Printf("chat_id: %v sending message...\n", task.ChatId)
		err := telegram.SendMessage(task.BotToken, task.ChatId, msg)
		if err != nil {
			log.Printf("chat_id: %d error: %v \n", task.ChatId, err)
			return false
		}
		log.Printf("chat_id: %v sending message... OK\n", task.ChatId)
	} else {
		log.Printf("Empty resultset. Skip message")
	}
	return true
}

func makeMessage(desc string, queryRes string, preformatted bool, addHeader bool) string {
	msg := ""
	if addHeader {
		msg += fmt.Sprintf("%s\n", desc)
	}
	if preformatted {
		msg += fmt.Sprintf("<pre>%s</pre>", queryRes)
	} else {
		msg += queryRes
	}
	return msg
}

