package monitor

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"bot-produtos/internal/database"
	"bot-produtos/internal/models"
	"bot-produtos/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Monitor gerencia o monitoramento periódico de produtos
type Monitor struct {
	db       *database.DB
	bot      *tgbotapi.BotAPI
	registry *scraper.Registry
	interval time.Duration
}

// New cria uma nova instância do monitor
func New(db *database.DB, bot *tgbotapi.BotAPI, registry *scraper.Registry, interval time.Duration) *Monitor {
	return &Monitor{
		db:       db,
		bot:      bot,
		registry: registry,
		interval: interval,
	}
}

// Start inicia o monitoramento em background
func (m *Monitor) Start() {
	log.Printf("Monitor iniciado. Verificando produtos a cada %v", m.interval)

	// Verificar imediatamente na primeira execução
	m.checkAllProducts()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for range ticker.C {
		m.checkAllProducts()
	}
}

// CheckProduct verifica um produto específico (usado pelo comando /check)
// Retorna o preço atual encontrado e um erro se houver
func (m *Monitor) CheckProduct(product models.Product) (float64, error) {
	// Encontrar scraper apropriado
	scraper := m.registry.FindScraper(product.URL)
	if scraper == nil {
		return 0, fmt.Errorf("nenhum scraper encontrado para URL: %s", product.URL)
	}

	// Buscar preço atual, original e desconto
	currentPrice, err := scraper.GetPrice(product.URL)
	if err != nil {
		return 0, fmt.Errorf("erro ao buscar preço: %v", err)
	}

	originalPrice, _ := scraper.GetOriginalPrice(product.URL)
	discount, _ := scraper.GetDiscount(product.URL)

	// Atualizar preços no banco
	if discount > 0 || originalPrice > 0 {
		if err := m.db.UpdateProductPricesWithDiscount(product.ID, currentPrice, originalPrice, discount); err != nil {
			return currentPrice, fmt.Errorf("erro ao atualizar preços no banco: %v", err)
		}
	} else {
		if err := m.db.UpdateProductPrice(product.ID, currentPrice); err != nil {
			return currentPrice, fmt.Errorf("erro ao atualizar preço no banco: %v", err)
		}
	}

	// Não verificar promoções aqui, apenas atualizar o preço
	return currentPrice, nil
}

func (m *Monitor) checkAllProducts() {
	products, err := m.db.GetActiveProducts()
	if err != nil {
		log.Printf("Erro ao buscar produtos: %v", err)
		return
	}

	if len(products) > 0 {
		for _, product := range products {
			m.checkProduct(product)
			// Pequeno delay entre requisições para não sobrecarregar
			time.Sleep(2 * time.Second)
		}
	}
}

func (m *Monitor) checkProduct(product models.Product) {
	// Encontrar scraper apropriado
	scraper := m.registry.FindScraper(product.URL)
	if scraper == nil {
		log.Printf("Nenhum scraper encontrado para URL: %s", product.URL)
		return
	}

	// Buscar preço atual, original e desconto
	currentPrice, err := scraper.GetPrice(product.URL)
	if err != nil {
		log.Printf("Erro ao buscar preço do produto %d (%s): %v", product.ID, product.URL, err)
		return
	}

	originalPrice, _ := scraper.GetOriginalPrice(product.URL)
	discount, _ := scraper.GetDiscount(product.URL)

	// Atualizar preços no banco (sempre atualizar, mesmo se o preço não mudou)
	if discount > 0 || originalPrice > 0 {
		if err := m.db.UpdateProductPricesWithDiscount(product.ID, currentPrice, originalPrice, discount); err != nil {
			log.Printf("Erro ao atualizar preços no banco: %v", err)
			return
		}
	} else {
		if err := m.db.UpdateProductPrice(product.ID, currentPrice); err != nil {
			log.Printf("Erro ao atualizar preço no banco: %v", err)
			return
		}
	}

	// Verificar se há promoção
	shouldNotify := false
	message := ""

	// Verificar se atingiu preço alvo
	if product.TargetPrice > 0 && currentPrice <= product.TargetPrice {
		// Só notificar se o preço mudou (não é a primeira verificação) ou se já está abaixo do alvo
		if product.CurrentPrice == 0 || currentPrice < product.CurrentPrice {
			shouldNotify = true
			discount := 0.0
			if product.CurrentPrice > 0 {
				discount = ((product.CurrentPrice - currentPrice) / product.CurrentPrice) * 100
			}
			message = fmt.Sprintf(
				"🎉 PROMOÇÃO DETECTADA!\n\n"+
					"Produto: %s\n"+
					"Preço atual: R$ %.2f\n"+
					"Preço alvo: R$ %.2f\n",
				product.Name,
				currentPrice,
				product.TargetPrice,
			)
			if discount > 0 {
				message += fmt.Sprintf("Desconto: %.1f%%\n", discount)
			}
			message += fmt.Sprintf("\nLink: %s", product.URL)
		}
	}

	// Verificar se atingiu desconto alvo
	// Para Mercado Livre, usar o desconto do site quando disponível
	if product.TargetDiscount > 0 {
		var currentDiscount float64
		
		// Se o produto tem desconto do site (Mercado Livre), usar esse valor
		if discount > 0 {
			currentDiscount = discount
		} else if product.CurrentPrice > 0 && currentPrice < product.CurrentPrice {
			// Se não tem desconto do site, calcular baseado na mudança de preço
			currentDiscount = ((product.CurrentPrice - currentPrice) / product.CurrentPrice) * 100
		} else if originalPrice > 0 && originalPrice > currentPrice {
			// Se tem preço original, calcular desconto baseado nele
			currentDiscount = ((originalPrice - currentPrice) / originalPrice) * 100
		}
		
		// Verificar se atingiu o desconto alvo
		if currentDiscount >= product.TargetDiscount {
			// Só notificar se o desconto mudou ou se é a primeira verificação
			if product.Discount == 0 || discount != product.Discount {
				shouldNotify = true
				message = fmt.Sprintf(
					"🎉 PROMOÇÃO DETECTADA!\n\n"+
						"Produto: %s\n"+
						"Preço atual: R$ %.2f\n"+
						"Desconto: %.1f%% (meta: %.1f%%)\n",
					product.Name,
					currentPrice,
					currentDiscount,
					product.TargetDiscount,
				)
				if originalPrice > 0 {
					message += fmt.Sprintf("Preço original: R$ %.2f\n", originalPrice)
				}
				message += fmt.Sprintf("\nLink: %s", product.URL)
			}
		}
	}

	// Enviar notificação se necessário
	if shouldNotify {
		chatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
		if err != nil {
			log.Printf("Erro ao parsear TELEGRAM_CHAT_ID: %v", err)
			return
		}

		msg := tgbotapi.NewMessage(chatID, message)
		if _, err := m.bot.Send(msg); err != nil {
			log.Printf("Erro ao enviar mensagem: %v", err)
		} else {
			log.Printf("Notificação enviada para produto %d", product.ID)
		}
	}
}

