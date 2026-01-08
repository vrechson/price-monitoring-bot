package bot

import (
	"fmt"
	"log"
	"os"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Init inicializa o bot do Telegram
func Init(token string) (*tgbotapi.BotAPI, error) {
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN não configurado. Verifique o arquivo .env")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		if err.Error() == "Unauthorized" {
			return nil, fmt.Errorf("token do Telegram inválido ou expirado. Verifique o TELEGRAM_BOT_TOKEN no arquivo .env. Para obter um token, fale com @BotFather no Telegram")
		}
		return nil, fmt.Errorf("erro ao conectar com Telegram: %v", err)
	}

	bot.Debug = false
	log.Printf("Bot autorizado como: %s", bot.Self.UserName)
	return bot, nil
}

// GetAuthorizedChatID retorna o Chat ID autorizado (se configurado)
func GetAuthorizedChatID() (int64, bool) {
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" {
		return 0, false
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return 0, false
	}

	return chatID, true
}

