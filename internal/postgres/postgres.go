package postgres

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/shomali11/xsql"
)

type Task struct {
	Id              uint64
	BotToken        string
	ChatId          int64
	ChatDescribe    string
	SqlQuery        string
	ScheduleCron    string
	LastExecutionTs sql.NullTime
	Settings        struct {
		Preformatted bool
		AddHeader    bool
	}
}

func GetTaskList(db *sql.DB) ([]Task, error) {
	q := `
		SELECT 
			id, 
			bot_token, 
			bot_chat_id, 
			chat_describe, 
			sql_query, 
			schedule_cron, 
			last_execution_ts,
			preformatted,
			add_header
		FROM tg_query_executor
		WHERE enable IS NULL OR enable = TRUE
	`
	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("processing query: %s. error: %s", q, err)
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		t := Task{}
		err := rows.Scan(
			&t.Id, 
			&t.BotToken, 
			&t.ChatId,
			&t.ChatDescribe, 
			&t.SqlQuery, 
			&t.ScheduleCron, 
			&t.LastExecutionTs, 
			&t.Settings.Preformatted, 
			&t.Settings.AddHeader)
		if err != nil {
			return nil, fmt.Errorf("scan row: %s", err)
		}
		fixTimezone(&t.LastExecutionTs)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func UpdateLastExecutionTs(db *sql.DB, t Task) error {
	q := `
		UPDATE tg_query_executor
		SET last_execution_ts = $2
		WHERE id = $1
	`
	_, err := db.Exec(q, t.Id, t.LastExecutionTs)
	if err != nil {
		return fmt.Errorf("processing query: %s. error: %s", q, err)
	}
	return nil
}

func ExecQuery(db *sql.DB, q string) (bool, string, error) {
	rows, err := db.Query(q)
	if err != nil {
		return false, "", fmt.Errorf("query: %s error: %s", q, err)
	}
	defer rows.Close()

	var dataExist = false
	for rows.Next() {
		dataExist = true
		break
	}

	if dataExist {
		rows2, err := db.Query(q)
		if err != nil {
			return false, "", fmt.Errorf("query: %s error: %s", q, err)
		}
		defer rows2.Close()
		str, err := xsql.Pretty(rows2)
		return true, str, err
	} else {
		return false, "", nil
	}
}

// it is subtracted 3 hours (database timezone settings set to UTC, but time is MSK)
func fixTimezone(nt *sql.NullTime) {
	if nt.Valid {
		nt.Time = nt.Time.Local().Add(-3 * time.Hour)
	}
}
