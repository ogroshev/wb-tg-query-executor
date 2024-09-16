package telegram

import (
	"fmt"

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