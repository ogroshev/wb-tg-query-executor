package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SendMessage(token string, chatId int64, message string) error {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("could not create bot: %s", err)
	}
	// bot.Debug = true

	m := tgbotapi.NewMessage(chatId, message)
	m.ParseMode = tgbotapi.ModeHTML
	_, err = bot.Send(m)
	if err != nil {
		return fmt.Errorf("could not send message: %s", err)
	}
	return nil
}

// SendRichMessage sends a rich message (Bot API 10.1+: native tables, headings, details, buttons).
// tgbotapi v5 does not know this method, so it is a plain JSON request.
func SendRichMessage(token string, chatId int64, htmlText string) error {
	body, _ := json.Marshal(map[string]any{
		"chat_id":      chatId,
		"rich_message": map[string]any{"html": htmlText, "skip_entity_detection": true},
	})
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post("https://api.telegram.org/bot"+token+"/sendRichMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("could not send rich message: %s", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || !r.Ok {
		return fmt.Errorf("could not send rich message: %d %s %s", resp.StatusCode, r.Description, string(raw))
	}
	return nil
}
