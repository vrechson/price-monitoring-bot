package scraper

// Scraper define a interface para scrapers de diferentes lojas
type Scraper interface {
	GetPrice(url string) (float64, error)
	GetOriginalPrice(url string) (float64, error) // Preço original (antes do desconto)
	GetDiscount(url string) (float64, error)      // Percentual de desconto (0-100)
	GetName(url string) (string, error)
	CanHandle(url string) bool
}

// Registry mantém um registro de todos os scrapers disponíveis
type Registry struct {
	scrapers []Scraper
}

// NewRegistry cria um novo registro de scrapers
func NewRegistry() *Registry {
	return &Registry{
		scrapers: []Scraper{
			NewMercadoLivreScraper(),
		},
	}
}

// FindScraper encontra o scraper apropriado para uma URL
func (r *Registry) FindScraper(url string) Scraper {
	for _, scraper := range r.scrapers {
		if scraper.CanHandle(url) {
			return scraper
		}
	}
	return nil
}

