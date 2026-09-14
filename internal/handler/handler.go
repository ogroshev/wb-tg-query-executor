package handler

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/postgres"
	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/telegram"
)

const (
	// Telegram hard limit on message text length (in characters).
	kTelegramMaxMessageLen = 4096
	// After this many consecutive failures the task is marked as executed
	// so that it does not spin forever re-running the query every loop.
	kMaxAttempts = 3
)

// consecutive failures per task id (in-memory, reset on restart)
var failCounts = map[uint64]int{}

func DoWork(db *sql.DB) {
	tasks, err := postgres.GetTaskList(db)
	if err != nil {
		log.Printf("could not get tasks: %s", err)
	}

	for _, t := range tasks {
		if !isItTimeToWork(t.Id, t.ScheduleCron, t.LastExecutionTs) {
			continue
		}
		log.Printf("Handle task id: %d. desc: %s", t.Id, t.ChatDescribe)
		ok := handleTask(db, t)
		if ok {
			failCounts[t.Id] = 0
		} else {
			failCounts[t.Id]++
			if failCounts[t.Id] < kMaxAttempts {
				log.Printf("id: %d failed (attempt %d/%d), will retry", t.Id, failCounts[t.Id], kMaxAttempts)
				continue
			}
			log.Printf("id: %d failed %d times in a row, giving up until next schedule", t.Id, failCounts[t.Id])
			failCounts[t.Id] = 0
		}
		t.LastExecutionTs = sql.NullTime{Time: time.Now(), Valid: true}
		postgres.UpdateLastExecutionTs(db, t)
		log.Printf("id: %d last_execution_ts updated to: %s\n", t.Id, t.LastExecutionTs.Time.String())
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

	nextTime := schedule.Next(lastExecTime)
	now := time.Now()
	diff := nextTime.Sub(now)
	log.Printf("id: %d. Next run: %s left: %s", id, nextTime, diff.String())
	return time.Now().After(nextTime)
}

func handleTask(db *sql.DB, task postgres.Task) bool {
	log.Printf("Executing query...\n")
	resultNotEmpty, resultPretty, err := postgres.ExecQuery(db, task.SqlQuery)
	if err != nil {
		log.Println(err)
		return false
	}
	log.Printf("Executing query... OK\n")
	if !resultNotEmpty {
		log.Printf("Empty resultset. Skip message")
		return true
	}
	msgs := makeMessages(task.ChatDescribe, resultPretty, task.Settings.Preformatted, task.Settings.AddHeader)
	for i, msg := range msgs {
		log.Printf("chat_id: %v sending message %d/%d (%d chars)...\n", task.ChatId, i+1, len(msgs), len([]rune(msg)))
		err := telegram.SendMessage(task.BotToken, task.ChatId, msg)
		if err != nil {
			log.Printf("chat_id: %d error: %v \n", task.ChatId, err)
			return false
		}
	}
	log.Printf("chat_id: %v sending message... OK\n", task.ChatId)
	return true
}

// makeMessages renders the query result into one or more Telegram messages.
// Trailing whitespace of every line is removed: xsql.Pretty pads every row to
// the widest one, which is invisible in Telegram but easily blows the 4096 limit.
// If the text still does not fit, it is split by lines (preferably at blank
// lines, i.e. between logical blocks), each chunk wrapped into its own <pre>.
func makeMessages(desc string, queryRes string, preformatted bool, addHeader bool) []string {
	lines := strings.Split(queryRes, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}

	header := ""
	if addHeader {
		header = fmt.Sprintf("%s\n", desc)
	}
	wrap := func(body string) string {
		if preformatted {
			return fmt.Sprintf("<pre>%s</pre>", body)
		}
		return body
	}
	overhead := 0
	if preformatted {
		overhead = len("<pre></pre>")
	}

	var msgs []string
	var chunk []string
	chunkLen := 0 // runes in chunk incl. newlines
	flush := func() {
		if len(chunk) == 0 {
			return
		}
		body := strings.Join(chunk, "\n")
		if len(msgs) == 0 {
			msgs = append(msgs, header+wrap(body))
		} else {
			msgs = append(msgs, wrap(body))
		}
		chunk = nil
		chunkLen = 0
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		lineLen := len([]rune(line)) + 1
		limit := kTelegramMaxMessageLen - overhead
		if len(msgs) == 0 {
			limit -= len([]rune(header))
		}
		if chunkLen+lineLen > limit && len(chunk) > 0 {
			// prefer to cut at the last blank line inside the current chunk
			cut := -1
			for j := len(chunk) - 1; j > 0; j-- {
				if strings.TrimSpace(chunk[j]) == "" {
					cut = j
					break
				}
			}
			if cut > 0 {
				rest := append([]string{}, chunk[cut+1:]...)
				chunk = chunk[:cut]
				flush()
				chunk = rest
				for _, r := range rest {
					chunkLen += len([]rune(r)) + 1
				}
			} else {
				flush()
			}
		}
		// a single line longer than the limit: hard-cut it
		for len([]rune(line))+1 > limit {
			flush()
			r := []rune(line)
			chunk = []string{string(r[:limit-1])}
			flush()
			line = string(r[limit-1:])
			lineLen = len([]rune(line)) + 1
		}
		chunk = append(chunk, line)
		chunkLen += lineLen
	}
	flush()
	return msgs
}
