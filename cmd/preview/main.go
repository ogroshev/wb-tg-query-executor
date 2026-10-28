// preview: run one executors.tg task right now and send the result to another chat,
// without touching last_execution_ts. For debugging alert formatting in a private chat.
//
//	preview -id 64 -chat 110751423 [-format table] [-footer '<p>..</p>']
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"

	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/config"
	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/handler"
	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/postgres"
)

func main() {
	id := flag.Uint64("id", 0, "executors.tg id")
	chat := flag.Int64("chat", 0, "chat id to send to (required)")
	format := flag.String("format", "", "override format: pre | table | html")
	footer := flag.String("footer", "", "override rich footer (rich-HTML)")
	sqlFile := flag.String("sql", "", "override query from file")
	desc := flag.String("desc", "", "override chat_describe (header)")
	header := flag.String("header", "", "override rich header (rich-HTML)")
	flag.Parse()
	if *id == 0 || *chat == 0 {
		flag.Usage()
		os.Exit(2)
	}
	conf, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("could not read config: %s", err)
	}
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		conf.Database.Host, conf.Database.Port, conf.Database.User, conf.Database.Password, conf.Database.Name))
	if err != nil {
		log.Fatal(err)
	}
	t, err := postgres.GetTask(db, *id)
	if err != nil {
		log.Fatal(err)
	}
	t.ChatId = *chat
	if *format != "" {
		t.Settings.Format = *format
	}
	if *footer != "" {
		t.Settings.Footer = *footer
	}
	if *header != "" {
		t.Settings.Header = *header
	}
	if *desc != "" {
		t.ChatDescribe = *desc
	}
	if *sqlFile != "" {
		b, err := os.ReadFile(*sqlFile)
		if err != nil {
			log.Fatal(err)
		}
		t.SqlQuery = string(b)
	}
	if !handler.RunTask(db, t) {
		os.Exit(1)
	}
}
