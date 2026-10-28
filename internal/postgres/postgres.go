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
		Format       string // pre | table | html (see handler/rich.go)
		Footer       string // rich-HTML appended after the result (rich formats only)
		Header       string // rich-HTML used instead of <h3>chat_describe</h3> (rich formats only)
	}
}

const taskColumns = `
			id, 
			bot_token, 
			bot_chat_id, 
			chat_describe, 
			sql_query, 
			schedule_cron, 
			last_execution_ts,
			preformatted,
			add_header,
			coalesce(format, 'pre'),
			coalesce(rich_footer, ''),
			coalesce(rich_header, '')`

// GetTask loads one task regardless of enable (used by cmd/preview).
func GetTask(db *sql.DB, id uint64) (Task, error) {
	t := Task{}
	err := db.QueryRow(`SELECT `+taskColumns+` FROM executors.tg WHERE id = $1`, id).Scan(
		&t.Id, &t.BotToken, &t.ChatId, &t.ChatDescribe, &t.SqlQuery, &t.ScheduleCron, &t.LastExecutionTs,
		&t.Settings.Preformatted, &t.Settings.AddHeader, &t.Settings.Format, &t.Settings.Footer, &t.Settings.Header)
	return t, err
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
			add_header,
			coalesce(format, 'pre'),
			coalesce(rich_footer, ''),
			coalesce(rich_header, '')
		FROM executors.tg
		WHERE enable = true
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
			&t.Settings.AddHeader,
			&t.Settings.Format,
			&t.Settings.Footer,
			&t.Settings.Header)
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
		UPDATE executors.tg
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

// Result is a raw query result for rich formats: values as text, nil = NULL.
type Result struct {
	Columns []string
	Numeric []bool
	Rows    [][]*string
}

func QueryResult(db *sql.DB, q string) (Result, error) {
	res := Result{}
	rows, err := db.Query(q)
	if err != nil {
		return res, fmt.Errorf("query: %s error: %s", q, err)
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		return res, err
	}
	for _, ct := range types {
		res.Columns = append(res.Columns, ct.Name())
		switch ct.DatabaseTypeName() {
		case "INT2", "INT4", "INT8", "NUMERIC", "FLOAT4", "FLOAT8":
			res.Numeric = append(res.Numeric, true)
		default:
			res.Numeric = append(res.Numeric, false)
		}
	}
	for rows.Next() {
		vals := make([]sql.NullString, len(types))
		ptrs := make([]any, len(types))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return res, fmt.Errorf("scan row: %s", err)
		}
		row := make([]*string, len(types))
		for i, v := range vals {
			if v.Valid {
				s := v.String
				row[i] = &s
			}
		}
		res.Rows = append(res.Rows, row)
	}
	return res, rows.Err()
}

// it is subtracted 3 hours (database timezone settings set to UTC, but time is MSK)
func fixTimezone(nt *sql.NullTime) {
	if nt.Valid {
		nt.Time = nt.Time.Local().Add(-3 * time.Hour)
	}
}
