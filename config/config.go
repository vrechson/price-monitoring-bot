package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contém as configurações da aplicação
type Config struct {
	TelegramBotToken    string
	TelegramChatID      int64
	CheckIntervalMinutes int
	CheckInterval       time.Duration
	DatabasePath        string
}

// Load carrega as configurações das variáveis de ambiente
func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN não configurado")
	}

	cfg := &Config{
		TelegramBotToken:    token,
		CheckIntervalMinutes: 30,
		DatabasePath:        "./products.db",
	}

	// Chat ID é opcional (pode ser usado para restrições, mas não obrigatório)
	if chatIDStr := os.Getenv("TELEGRAM_CHAT_ID"); chatIDStr != "" {
		if chatID, err := strconv.ParseInt(chatIDStr, 10, 64); err == nil {
			cfg.TelegramChatID = chatID
		}
	}

	// Intervalo de verificação
	if envInterval := os.Getenv("CHECK_INTERVAL_MINUTES"); envInterval != "" {
		if parsed, err := strconv.Atoi(envInterval); err == nil && parsed > 0 {
			cfg.CheckIntervalMinutes = parsed
		}
	}
	cfg.CheckInterval = time.Duration(cfg.CheckIntervalMinutes) * time.Minute

	return cfg, nil
}

