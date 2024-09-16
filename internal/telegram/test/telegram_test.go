package test

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/telegram"
)


func TestSendMessage(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	log.SetLevel(log.TraceLevel)

	err := godotenv.Load(".env")
	if err != nil {
		t.Fatal("Error loading .env file")
	}	
	token := os.Getenv("TELEGRAM_TOKEN")
	chatId := os.Getenv("TELEGRAM_CHAT_ID")
	//to int64
	chatIdInt, err := strconv.ParseInt(chatId, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	err = telegram.SendMessage(token, chatIdInt, "test from go")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("log output: ", logBuffer.String())
}