package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"bot-produtos/internal/database"
	"bot-produtos/internal/monitor"
	"bot-produtos/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// escapeHTML escapa caracteres especiais do HTML
func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// SetupCommands configura os handlers de comandos do bot
func SetupCommands(bot *tgbotapi.BotAPI, db *database.DB, monitor *monitor.Monitor, registry *scraper.Registry) {
	authorizedChatID, hasAuth := GetAuthorizedChatID()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		text := update.Message.Text
		if text == "" {
			continue
		}

		// Extrair comando (remover @botname se presente e pegar apenas o comando)
		parts := strings.Fields(text)
		if len(parts) == 0 {
			continue
		}
		
		command := strings.ToLower(parts[0])
		// Remover @botname se presente
		if idx := strings.Index(command, "@"); idx > 0 {
			command = command[:idx]
		}

		// Comandos públicos (não precisam de autorização)
		isPublicCommand := command == "/start" || command == "/help"

		// Verificar autorização se configurado (exceto para comandos públicos)
		if !isPublicCommand && hasAuth && update.Message.Chat.ID != authorizedChatID {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Você não está autorizado a usar este bot.")
			bot.Send(msg)
			continue
		}

		switch command {
		case "/start", "/help":
			handleHelp(bot, update.Message.Chat.ID)
		case "/add":
			handleAddProduct(bot, update.Message, db, registry)
		case "/list":
			handleListProducts(bot, update.Message.Chat.ID, db)
		case "/remove":
			handleRemoveProduct(bot, update.Message, db)
		case "/check":
			handleCheckProduct(bot, update.Message, db, monitor, registry)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Comando não reconhecido. Use /help para ver os comandos disponíveis.")
			bot.Send(msg)
		}
	}
}

func handleHelp(bot *tgbotapi.BotAPI, chatID int64) {
	helpText := `🤖 <b>Bot de Monitoramento de Preços</b>

<b>Comandos disponíveis:</b>

<b>/add</b> - Adicionar novo produto para monitorar
Uso: /add &lt;URL&gt; &lt;preço_alvo&gt; OU /add &lt;URL&gt; &lt;desconto%&gt;
Exemplo: /add https://mercadolivre.com.br/produto 3000
Exemplo: /add https://mercadolivre.com.br/produto 15% (para 15% de desconto)

<b>/list</b> - Listar todos os produtos monitorados

<b>/remove &lt;id&gt;</b> - Remover produto do monitoramento
Exemplo: /remove 1

<b>/check &lt;id&gt;</b> - Verificar preço de um produto agora
Exemplo: /check 1

<b>/help</b> - Mostrar esta mensagem de ajuda
`

	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "HTML"
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Erro ao enviar mensagem de ajuda: %v", err)
		// Tentar sem formatação se houver erro
		msg.ParseMode = ""
		bot.Send(msg)
	}
}

func handleAddProduct(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *database.DB, registry *scraper.Registry) {
	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Formato incorreto.\n\nUso: /add <URL> <preço_alvo> OU /add <URL> <desconto%>\n\nExemplo: /add https://mercadolivre.com.br/produto 3000\nExemplo: /add https://mercadolivre.com.br/produto 15%")
		bot.Send(msg)
		return
	}

	url := parts[1]
	targetStr := parts[2]

	// Verificar se é percentual ou preço
	var targetPrice, targetDiscount float64
	if strings.HasSuffix(targetStr, "%") {
		discountStr := strings.TrimSuffix(targetStr, "%")
		discount, err := strconv.ParseFloat(discountStr, 64)
		if err != nil || discount < 0 || discount > 100 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Desconto inválido. Use um valor entre 0 e 100.")
			bot.Send(msg)
			return
		}
		targetDiscount = discount
	} else {
		price, err := strconv.ParseFloat(targetStr, 64)
		if err != nil || price <= 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Preço inválido. Use um valor numérico positivo.")
			bot.Send(msg)
			return
		}
		targetPrice = price
	}

	// Encontrar scraper apropriado
	scraper := registry.FindScraper(url)
	if scraper == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ URL não suportada. Atualmente suportamos apenas Mercado Livre.")
		bot.Send(msg)
		return
	}

	// Buscar nome do produto
	name, err := scraper.GetName(url)
	if err != nil {
		log.Printf("Erro ao buscar nome do produto: %v", err)
		name = "Produto sem nome"
	}

	// Adicionar ao banco
	err = db.AddProduct(url, name, targetPrice, targetDiscount)
	if err != nil {
		var msg tgbotapi.MessageConfig
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			msg = tgbotapi.NewMessage(message.Chat.ID, "❌ Este produto já está sendo monitorado.")
		} else {
			msg = tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Erro ao adicionar produto: %v", err))
		}
		bot.Send(msg)
		return
	}

	// Buscar preço atual, original e desconto
	currentPrice, err := scraper.GetPrice(url)
	originalPrice, _ := scraper.GetOriginalPrice(url)
	discountPercent, _ := scraper.GetDiscount(url)
	
	priceInfo := ""
	discountInfo := ""
	if err == nil {
		priceInfo = fmt.Sprintf("\nPreço atual: R$ %.2f", currentPrice)
		
		// Atualizar preços no banco
		products, _ := db.GetActiveProducts()
		if len(products) > 0 {
			if discountPercent > 0 || originalPrice > 0 {
				db.UpdateProductPricesWithDiscount(products[len(products)-1].ID, currentPrice, originalPrice, discountPercent)
			} else {
				db.UpdateProductPrice(products[len(products)-1].ID, currentPrice)
			}
		}
		
		// Mostrar desconto do site se disponível
		if discountPercent > 0 {
			if originalPrice > 0 {
				discountInfo = fmt.Sprintf("\n🎉 Produto está em promoção! %.1f%% OFF (de R$ %.2f)", discountPercent, originalPrice)
			} else {
				discountInfo = fmt.Sprintf("\n🎉 Produto está em promoção! %.1f%% OFF", discountPercent)
			}
		} else if originalPrice > 0 && originalPrice > currentPrice {
			// Calcular desconto se não encontrou no site mas tem preço original
			discount := ((originalPrice - currentPrice) / originalPrice) * 100
			discountInfo = fmt.Sprintf("\n🎉 Produto está em promoção! %.1f%% OFF (de R$ %.2f)", discount, originalPrice)
		} else if targetPrice > 0 && currentPrice < targetPrice {
			// Se não tem preço original mas está abaixo do alvo
			discount := ((targetPrice - currentPrice) / targetPrice) * 100
			discountInfo = fmt.Sprintf("\n🎉 Produto já está abaixo do preço alvo! Desconto: %.1f%%", discount)
		} else if targetPrice > 0 && currentPrice > targetPrice {
			// Se o preço atual está acima do alvo, mostrar quanto falta
			diff := currentPrice - targetPrice
			discountInfo = fmt.Sprintf("\n💡 Faltam R$ %.2f para atingir o preço alvo", diff)
		}
	}

	response := fmt.Sprintf(
		"✅ Produto adicionado com sucesso!\n\n"+
			"Nome: %s\n"+
			"URL: %s%s%s",
		name, url, priceInfo, discountInfo,
	)

	if targetPrice > 0 {
		response += fmt.Sprintf("\nPreço alvo: R$ %.2f", targetPrice)
	}
	if targetDiscount > 0 {
		response += fmt.Sprintf("\nDesconto alvo: %.1f%%", targetDiscount)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, response)
	bot.Send(msg)
}

func handleListProducts(bot *tgbotapi.BotAPI, chatID int64, db *database.DB) {
	products, err := db.GetActiveProducts()
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Erro ao listar produtos: %v", err))
		bot.Send(msg)
		return
	}

	if len(products) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📋 Nenhum produto sendo monitorado no momento.")
		bot.Send(msg)
		return
	}

	var response strings.Builder
	response.WriteString("📋 <b>Produtos em Monitoramento:</b>\n\n")

	for _, p := range products {
		// Escapar HTML no nome do produto
		productName := escapeHTML(p.Name)
		
		response.WriteString(fmt.Sprintf("🆔 <b>ID: %d</b>\n", p.ID))
		response.WriteString(fmt.Sprintf("📦 %s\n", productName))

		if p.CurrentPrice > 0 {
			response.WriteString(fmt.Sprintf("💰 <b>Preço atual: R$ %.2f</b>\n", p.CurrentPrice))
			
			// Mostrar desconto do banco se disponível
			if p.Discount > 0 {
				if p.OriginalPrice > 0 {
					response.WriteString(fmt.Sprintf("🎉 <b>%.1f%% OFF</b> (de R$ %.2f)\n", p.Discount, p.OriginalPrice))
				} else {
					response.WriteString(fmt.Sprintf("🎉 <b>%.1f%% OFF</b>\n", p.Discount))
				}
			} else if p.OriginalPrice > 0 && p.OriginalPrice > p.CurrentPrice {
				// Calcular desconto se não tem no banco mas tem preço original
				discount := ((p.OriginalPrice - p.CurrentPrice) / p.OriginalPrice) * 100
				response.WriteString(fmt.Sprintf("🎉 <b>%.1f%% OFF</b> (de R$ %.2f)\n", discount, p.OriginalPrice))
			}
		} else {
			response.WriteString("💰 <b>Preço atual: Não verificado ainda</b>\n")
		}

		if p.TargetPrice > 0 {
			diff := p.CurrentPrice - p.TargetPrice
			if p.CurrentPrice > 0 && diff > 0 {
				// Calcular desconto em relação ao preço alvo
				discount := (diff / p.TargetPrice) * 100
				response.WriteString(fmt.Sprintf("🎯 Preço alvo: R$ %.2f (faltam R$ %.2f - %.1f%% acima)\n", p.TargetPrice, diff, discount))
			} else if p.CurrentPrice > 0 && diff <= 0 {
				// Produto está em promoção! Meta atingida
				response.WriteString(fmt.Sprintf("🎯 Preço alvo: R$ %.2f ✅ <b>META ATINGIDA!</b>\n", p.TargetPrice))
			} else {
				response.WriteString(fmt.Sprintf("🎯 Preço alvo: R$ %.2f\n", p.TargetPrice))
			}
		}

		if p.TargetDiscount > 0 {
			response.WriteString(fmt.Sprintf("🎯 Desconto alvo: %.1f%%\n", p.TargetDiscount))
		}

		if !p.LastChecked.IsZero() {
			response.WriteString(fmt.Sprintf("🕐 Última verificação: %s\n", p.LastChecked.Format("02/01/2006 15:04")))
		} else {
			response.WriteString("🕐 Última verificação: Nunca\n")
		}

		response.WriteString(fmt.Sprintf("🔗 %s\n\n", p.URL))
	}

	msg := tgbotapi.NewMessage(chatID, response.String())
	msg.ParseMode = "HTML"
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Erro ao enviar lista de produtos com HTML: %v", err)
		// Tentar enviar sem formatação se houver erro
		msg.ParseMode = ""
		if _, err2 := bot.Send(msg); err2 != nil {
			log.Printf("Erro ao enviar lista sem formatação: %v", err2)
		}
	}
}

func handleRemoveProduct(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *database.DB) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Formato incorreto.\n\nUso: /remove <id>\n\nExemplo: /remove 1")
		bot.Send(msg)
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ ID inválido.")
		bot.Send(msg)
		return
	}

	product, err := db.GetProductByID(id)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Produto não encontrado.")
		bot.Send(msg)
		return
	}

	err = db.DeactivateProduct(id)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Erro ao remover produto: %v", err))
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ Produto removido: %s", product.Name))
	bot.Send(msg)
}

func handleCheckProduct(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *database.DB, monitor *monitor.Monitor, registry *scraper.Registry) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Formato incorreto.\n\nUso: /check <id>\n\nExemplo: /check 1")
		bot.Send(msg)
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ ID inválido.")
		bot.Send(msg)
		return
	}

	product, err := db.GetProductByID(id)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Produto não encontrado.")
		bot.Send(msg)
		return
	}

	// Enviar mensagem de "verificando"
	waitMsg := tgbotapi.NewMessage(message.Chat.ID, "⏳ Verificando preço...")
	sentMsg, err := bot.Send(waitMsg)
	var sentMessageID int = 0
	if err == nil {
		sentMessageID = sentMsg.MessageID
	}

	// Verificar produto (isso atualiza o preço no banco)
	newPrice, err := monitor.CheckProduct(*product)
	if err != nil {
		errorText := fmt.Sprintf("❌ Erro ao verificar preço: %v", err)
		if sentMessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(message.Chat.ID, sentMessageID, errorText)
			bot.Send(editMsg)
		} else {
			errorMsg := tgbotapi.NewMessage(message.Chat.ID, errorText)
			bot.Send(errorMsg)
		}
		return
	}

	// Buscar produto atualizado do banco
	updatedProduct, err := db.GetProductByID(id)
	if err != nil {
		errorText := fmt.Sprintf("❌ Erro ao buscar produto atualizado: %v", err)
		if sentMessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(message.Chat.ID, sentMessageID, errorText)
			bot.Send(editMsg)
		} else {
			errorMsg := tgbotapi.NewMessage(message.Chat.ID, errorText)
			bot.Send(errorMsg)
		}
		return
	}
	
	// Usar o preço retornado se o banco ainda não foi atualizado
	if updatedProduct.CurrentPrice == 0 && newPrice > 0 {
		updatedProduct.CurrentPrice = newPrice
	}

	// Montar resposta (usar HTML)
	productName := escapeHTML(updatedProduct.Name)
	response := fmt.Sprintf(
		"📊 <b>Produto: %s</b>\n\n"+
			"Preço atual: R$ %.2f\n"+
			"Preço anterior: R$ %.2f\n"+
			"Link: %s",
		productName,
		updatedProduct.CurrentPrice,
		product.CurrentPrice,
		updatedProduct.URL,
	)

	// Mostrar desconto do banco se disponível
	if updatedProduct.Discount > 0 {
		if updatedProduct.OriginalPrice > 0 {
			response += fmt.Sprintf("\n\n🎉 <b>%.1f%% OFF</b> (de R$ %.2f)", updatedProduct.Discount, updatedProduct.OriginalPrice)
		} else {
			response += fmt.Sprintf("\n\n🎉 <b>%.1f%% OFF</b>", updatedProduct.Discount)
		}
	} else if updatedProduct.OriginalPrice > 0 && updatedProduct.OriginalPrice > updatedProduct.CurrentPrice {
		// Calcular desconto se não tem no banco mas tem preço original
		discount := ((updatedProduct.OriginalPrice - updatedProduct.CurrentPrice) / updatedProduct.OriginalPrice) * 100
		response += fmt.Sprintf("\n\n🎉 <b>%.1f%% OFF</b> (de R$ %.2f)", discount, updatedProduct.OriginalPrice)
	} else if updatedProduct.CurrentPrice < product.CurrentPrice && product.CurrentPrice > 0 {
		// Se não tem preço original mas o preço diminuiu
		discount := ((product.CurrentPrice - updatedProduct.CurrentPrice) / product.CurrentPrice) * 100
		response += fmt.Sprintf("\n\n🎉 Desconto de %.1f%%!", discount)
	}
	
	// Mostrar desconto em relação ao preço alvo se estiver em promoção
	if updatedProduct.TargetPrice > 0 && updatedProduct.CurrentPrice > 0 {
		if updatedProduct.CurrentPrice <= updatedProduct.TargetPrice {
			discount := ((updatedProduct.TargetPrice - updatedProduct.CurrentPrice) / updatedProduct.TargetPrice) * 100
			response += fmt.Sprintf("\n\n✅ Produto está abaixo do preço alvo! %.1f%% OFF", discount)
		}
	}

	// Tentar editar a mensagem de "verificando" se foi enviada
	if sentMessageID != 0 {
		editMsg := tgbotapi.NewEditMessageText(message.Chat.ID, sentMessageID, response)
		editMsg.ParseMode = "HTML"
		if _, err := bot.Send(editMsg); err != nil {
			log.Printf("Erro ao editar mensagem (tentando enviar nova): %v", err)
			// Se falhar ao editar, enviar nova mensagem
			newMsg := tgbotapi.NewMessage(message.Chat.ID, response)
			newMsg.ParseMode = "HTML"
			if _, err2 := bot.Send(newMsg); err2 != nil {
				// Se falhar com HTML, tentar sem formatação
				newMsg.ParseMode = ""
				bot.Send(newMsg)
			}
		}
	} else {
		// Se não conseguiu enviar a mensagem inicial, enviar resposta diretamente
		newMsg := tgbotapi.NewMessage(message.Chat.ID, response)
		newMsg.ParseMode = "HTML"
		if _, err := bot.Send(newMsg); err != nil {
			log.Printf("Erro ao enviar mensagem de resposta: %v", err)
			// Tentar sem formatação
			newMsg.ParseMode = ""
			bot.Send(newMsg)
		}
	}
}

