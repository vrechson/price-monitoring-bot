package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"bot-produtos/config"
	"bot-produtos/internal/bot"
	"bot-produtos/internal/database"
	"bot-produtos/internal/monitor"
	"bot-produtos/internal/scraper"

	"github.com/joho/godotenv"
)

func main() {
	// Carregar variáveis de ambiente
	if err := godotenv.Load(); err != nil {
		log.Println("Arquivo .env não encontrado, usando variáveis de ambiente do sistema")
	}

	// Carregar configurações
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erro ao carregar configurações: %v", err)
	}

	// Inicializar banco de dados
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Erro ao inicializar banco de dados: %v", err)
	}
	defer db.Close()

	// Inicializar bot do Telegram
	telegramBot, err := bot.Init(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("Erro ao inicializar bot do Telegram: %v", err)
	}

	// Inicializar scrapers
	scraperRegistry := scraper.NewRegistry()

	// Criar gerenciador de monitoramento
	monitorInstance := monitor.New(db, telegramBot, scraperRegistry, cfg.CheckInterval)

	// Iniciar monitoramento em background
	go monitorInstance.Start()

	// Configurar comandos do bot
	bot.SetupCommands(telegramBot, db, monitorInstance, scraperRegistry)

	// Aguardar sinal de interrupção
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Encerrando bot...")
}

