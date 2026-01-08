package scraper

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// MercadoLivreScraper implementa o scraper para Mercado Livre
type MercadoLivreScraper struct {
	client *http.Client
}

// NewMercadoLivreScraper cria uma nova instância do scraper do Mercado Livre
func NewMercadoLivreScraper() *MercadoLivreScraper {
	return &MercadoLivreScraper{}
}

func (m *MercadoLivreScraper) getClient() *http.Client {
	if m.client == nil {
		m.client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return m.client
}

// CanHandle verifica se o scraper pode lidar com a URL fornecida
// URLs do Mercado Livre sempre começam com "https://mercadolivre.com.br/"
func (m *MercadoLivreScraper) CanHandle(url string) bool {
	return strings.HasPrefix(url, "https://mercadolivre.com.br/") || 
		   strings.HasPrefix(url, "http://mercadolivre.com.br/") ||
		   strings.Contains(url, "mercadolivre.com.br")
}

// GetPrice extrai o preço de um produto do Mercado Livre
func (m *MercadoLivreScraper) GetPrice(url string) (float64, error) {
	cleanURL := m.cleanURL(url)

	req, err := http.NewRequest("GET", cleanURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := m.getClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, err
	}

	// Primeiro, tentar buscar especificamente o preço promocional
	// O Mercado Livre geralmente mostra o preço promocional em elementos específicos
	var priceText string
	var promotionalPrice string
	
	// Buscar em elementos que geralmente contêm o preço promocional
	promotionalSelectors := []string{
		".ui-pdp-price__second-line .andes-money-amount__fraction",
		".ui-pdp-price__second-line .andes-money-amount",
		".ui-pdp-price--size-large .andes-money-amount__fraction",
		".andes-money-amount--cents-superscript + .andes-money-amount__fraction",
	}
	
	for _, selector := range promotionalSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if promotionalPrice == "" {
				text := strings.TrimSpace(s.Text())
				if text != "" {
					promotionalPrice = text
				}
			}
		})
		if promotionalPrice != "" {
			break
		}
	}
	
	// Se encontrou preço promocional, usar ele
	if promotionalPrice != "" {
		priceText = promotionalPrice
	} else {
		// Caso contrário, buscar em todos os seletores possíveis
		priceSelectors := []string{
			"[data-testid='price'] .andes-money-amount__fraction",
			".ui-pdp-price__first-line .andes-money-amount__fraction",
			".andes-money-amount__fraction",
			".price-tag-fraction",
			"[data-testid='price']",
		}

		var prices []string // Coletar todos os preços encontrados
		
		for _, selector := range priceSelectors {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				if text != "" {
					prices = append(prices, text)
				}
			})
		}
		
		// Se houver múltiplos preços, pegar o menor (que geralmente é o promocional)
		if len(prices) > 0 {
			if len(prices) > 1 {
				// Se houver múltiplos preços, tentar pegar o menor
				// Isso geralmente indica que o menor é o promocional
				var minPrice float64 = -1
				for _, p := range prices {
					// Limpar e converter temporariamente para comparar
					cleanP := strings.ReplaceAll(p, ".", "")
					cleanP = strings.ReplaceAll(cleanP, ",", ".")
					re := regexp.MustCompile(`[^0-9.]`)
					cleanP = re.ReplaceAllString(cleanP, "")
					if val, err := strconv.ParseFloat(cleanP, 64); err == nil {
						if minPrice < 0 || val < minPrice {
							minPrice = val
							priceText = p
						}
					}
				}
			} else {
				priceText = prices[0]
			}
		}
	}

	// Se não encontrou, tentar buscar em atributos data e meta tags
	if priceText == "" {
		// Buscar em data-testid='price' com conteúdo
		doc.Find("[data-testid='price']").Each(func(i int, s *goquery.Selection) {
			if priceText == "" {
				// Tentar content primeiro
				priceText = s.AttrOr("content", "")
				// Se não tiver content, pegar o texto
				if priceText == "" {
					priceText = strings.TrimSpace(s.Text())
				}
			}
		})
		
		// Buscar em meta tags
		if priceText == "" {
			doc.Find("meta[property='product:price:amount']").Each(func(i int, s *goquery.Selection) {
				if priceText == "" {
					priceText = s.AttrOr("content", "")
				}
			})
		}
	}

	// Tentar buscar no JSON-LD (priorizar preço em "offers" que geralmente tem o preço atual/promocional)
	if priceText == "" {
		doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
			jsonText := s.Text()
			
			// Primeiro tentar buscar em "offers" que geralmente tem o preço promocional
			offersRe := regexp.MustCompile(`"offers"[^}]*"price"\s*:\s*"?([0-9.]+)"?`)
			offersMatches := offersRe.FindStringSubmatch(jsonText)
			if len(offersMatches) > 1 {
				priceText = offersMatches[1]
				return
			}
			
			// Fallback: buscar qualquer "price"
			re := regexp.MustCompile(`"price"\s*:\s*"?([0-9.]+)"?`)
			matches := re.FindStringSubmatch(jsonText)
			if len(matches) > 1 {
				priceText = matches[1]
			}
		})
	}

	if priceText == "" {
		return 0, fmt.Errorf("preço não encontrado na página")
	}

	// Limpar o texto do preço
	priceText = strings.ReplaceAll(priceText, ".", "")
	priceText = strings.ReplaceAll(priceText, ",", ".")
	re := regexp.MustCompile(`[^0-9.]`)
	priceText = re.ReplaceAllString(priceText, "")

	price, err := strconv.ParseFloat(priceText, 64)
	if err != nil {
		return 0, fmt.Errorf("erro ao parsear preço '%s': %v", priceText, err)
	}

	return price, nil
}

// GetOriginalPrice extrai o preço original (antes do desconto) de um produto do Mercado Livre
func (m *MercadoLivreScraper) GetOriginalPrice(url string) (float64, error) {
	cleanURL := m.cleanURL(url)

	req, err := http.NewRequest("GET", cleanURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := m.getClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, err
	}

	// Buscar preço original (geralmente aparece riscado na primeira linha quando há promoção)
	// O preço original geralmente está em elementos com classe relacionada a "previous" ou "original"
	originalPriceSelectors := []string{
		".andes-money-amount--previous-price .andes-money-amount__fraction",
		".andes-money-amount--previous-price",
		".ui-pdp-price__original .andes-money-amount__fraction",
		".ui-pdp-price__original",
		".ui-pdp-price__first-line .andes-money-amount--previous-price .andes-money-amount__fraction",
		".ui-pdp-price__first-line .andes-money-amount--previous-price",
		// Se houver duas linhas de preço, a primeira geralmente é o original
		".ui-pdp-price__first-line .andes-money-amount__fraction",
	}

	var originalPriceText string
	var allPrices []string
	
	// Coletar todos os preços encontrados
	for _, selector := range originalPriceSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				allPrices = append(allPrices, text)
				// Priorizar preços de elementos com "previous" ou "original"
				if strings.Contains(selector, "previous") || strings.Contains(selector, "original") {
					if originalPriceText == "" {
						originalPriceText = text
					}
				}
			}
		})
	}
	
	// Se não encontrou em elementos específicos, mas encontrou múltiplos preços,
	// pegar o maior (que geralmente é o original quando há promoção)
	if originalPriceText == "" && len(allPrices) > 1 {
		var maxPrice float64 = -1
		for _, p := range allPrices {
			cleanP := strings.ReplaceAll(p, ".", "")
			cleanP = strings.ReplaceAll(cleanP, ",", ".")
			re := regexp.MustCompile(`[^0-9.]`)
			cleanP = re.ReplaceAllString(cleanP, "")
			if val, err := strconv.ParseFloat(cleanP, 64); err == nil {
				if maxPrice < 0 || val > maxPrice {
					maxPrice = val
					originalPriceText = p
				}
			}
		}
	}

	// Se não encontrou, tentar buscar no JSON-LD
	if originalPriceText == "" {
		doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
			jsonText := s.Text()
			// Buscar por "priceCurrency" e "highPrice" ou "listPrice"
			re := regexp.MustCompile(`"(listPrice|highPrice|originalPrice)"\s*:\s*"?([0-9.]+)"?`)
			matches := re.FindStringSubmatch(jsonText)
			if len(matches) > 2 {
				originalPriceText = matches[2]
			}
		})
	}

	// Se não encontrou preço original, retornar 0 (não há desconto)
	if originalPriceText == "" {
		return 0, nil
	}

	// Limpar o texto do preço
	originalPriceText = strings.ReplaceAll(originalPriceText, ".", "")
	originalPriceText = strings.ReplaceAll(originalPriceText, ",", ".")
	re := regexp.MustCompile(`[^0-9.]`)
	originalPriceText = re.ReplaceAllString(originalPriceText, "")

	originalPrice, err := strconv.ParseFloat(originalPriceText, 64)
	if err != nil {
		return 0, nil // Se não conseguir parsear, retornar 0 (sem preço original)
	}

	return originalPrice, nil
}

// GetDiscount extrai o percentual de desconto de um produto do Mercado Livre
func (m *MercadoLivreScraper) GetDiscount(url string) (float64, error) {
	cleanURL := m.cleanURL(url)

	req, err := http.NewRequest("GET", cleanURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := m.getClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, err
	}

	// Buscar o campo de desconto diretamente
	// Exemplo: <span class="andes-money-amount__discount ...">17% OFF</span>
	// O desconto está dentro de ui-pdp-price__second-line
	discountSelectors := []string{
		".ui-pdp-price__second-line .andes-money-amount__discount",
		".andes-money-amount__discount",
		".ui-pdp-price__discount",
		"[class*='discount']",
		".ui-pdp-price__discount--with-bg-color",
	}

	var discountText string
	for _, selector := range discountSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if discountText == "" {
				text := strings.TrimSpace(s.Text())
				// Procurar por padrão "XX% OFF" ou apenas "XX%"
				if strings.Contains(text, "%") {
					discountText = text
				}
			}
		})
		if discountText != "" {
			break
		}
	}

	// Se não encontrou, retornar 0 (sem desconto)
	if discountText == "" {
		return 0, nil
	}

	// Extrair o número do texto (ex: "17% OFF" -> 17)
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)
	matches := re.FindStringSubmatch(discountText)
	if len(matches) > 1 {
		discount, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return 0, nil
		}
		return discount, nil
	}

	return 0, nil
}

// GetName extrai o nome de um produto do Mercado Livre
func (m *MercadoLivreScraper) GetName(url string) (string, error) {
	cleanURL := m.cleanURL(url)

	req, err := http.NewRequest("GET", cleanURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := m.getClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Tentar encontrar o nome do produto
	nameSelectors := []string{
		"h1.ui-pdp-title",
		"h1[data-testid='title']",
		".ui-pdp-title",
		"h1",
	}

	var name string
	for _, selector := range nameSelectors {
		doc.Find(selector).First().Each(func(i int, s *goquery.Selection) {
			if name == "" {
				name = strings.TrimSpace(s.Text())
			}
		})
		if name != "" {
			break
		}
	}

	if name == "" {
		// Tentar buscar no JSON-LD
		doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
			jsonText := s.Text()
			re := regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)
			matches := re.FindStringSubmatch(jsonText)
			if len(matches) > 1 {
				name = matches[1]
			}
		})
	}

	if name == "" {
		name = "Produto sem nome"
	}

	return name, nil
}

func (m *MercadoLivreScraper) cleanURL(url string) string {
	parts := strings.Split(url, "#")
	return parts[0]
}

