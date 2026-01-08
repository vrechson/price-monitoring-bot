# Bot de Monitoramento de Preços - Telegram

Bot de Telegram escrito em Go para monitorar preços de produtos em lojas online. Atualmente suporta Mercado Livre, mas foi projetado para ser facilmente extensível para outras lojas.

## Funcionalidades

- ✅ Monitoramento automático de preços em intervalos configuráveis
- ✅ Notificações via Telegram quando produtos atingem preço alvo ou desconto desejado
- ✅ Suporte para monitorar por preço alvo ou percentual de desconto
- ✅ Banco de dados SQLite para persistência
- ✅ Comandos do Telegram para gerenciar produtos
- ✅ Arquitetura extensível para adicionar novos scrapers

## Requisitos

- Go 1.21 ou superior
- Token do bot do Telegram
- Chat ID do Telegram

## Instalação

1. Clone o repositório:
```bash
git clone <seu-repositorio>
cd bot-produtos
```

2. Instale as dependências:
```bash
go mod download
```

3. Configure o arquivo `.env`:
```bash
cp env.sample .env
```

Edite o arquivo `.env` e adicione suas credenciais:
```
TELEGRAM_BOT_TOKEN=seu_token_aqui
TELEGRAM_CHAT_ID=seu_chat_id_aqui
CHECK_INTERVAL_MINUTES=30
```

**Nota:** O arquivo `env.sample` contém exemplos e instruções detalhadas sobre cada variável de ambiente.

### Como obter o Token do Bot

1. Abra o Telegram e procure por `@BotFather`
2. Envie `/newbot` e siga as instruções
3. Copie o token fornecido

### Como obter o Chat ID

1. Abra o Telegram e procure por `@userinfobot`
2. Envie qualquer mensagem
3. O bot retornará seu Chat ID

## Uso

### Executar o bot

```bash
go run cmd/bot/main.go
```

Ou compile e execute:
```bash
go build -o bot-produtos cmd/bot/main.go
./bot-produtos
```

### Comandos do Telegram

- `/start` ou `/help` - Mostra a lista de comandos disponíveis
- `/add <URL> <preço_alvo>` - Adiciona um produto para monitorar por preço
  - Exemplo: `/add https://mercadolivre.com.br/produto 3000`
- `/add <URL> <desconto%>` - Adiciona um produto para monitorar por desconto
  - Exemplo: `/add https://mercadolivre.com.br/produto 15%`
- `/list` - Lista todos os produtos monitorados
- `/remove <id>` - Remove um produto do monitoramento
  - Exemplo: `/remove 1`
- `/check <id>` - Verifica o preço de um produto imediatamente
  - Exemplo: `/check 1`

## Exemplos

### Monitorar por preço alvo

```
/add https://www.mercadolivre.com.br/lava-e-seca-11kg-titanium-slim-conectada-midea-cinza-escuro/p/MLB50097091 3000
```

O bot notificará quando o preço estiver em R$ 3.000,00 ou abaixo.

### Monitorar por desconto

```
/add https://www.mercadolivre.com.br/midea-healthguard-mf201w110wbgk-01-corrente-eletrica-127v-titanium/p/MLB50097091 20%
```

O bot notificará quando houver um desconto de 20% ou mais.

## Estrutura do Projeto

```
bot-produtos/
├── cmd/
│   └── bot/
│       └── main.go              # Ponto de entrada da aplicação
├── internal/
│   ├── bot/
│   │   ├── bot.go                # Inicialização do bot do Telegram
│   │   └── handlers.go           # Handlers de comandos do bot
│   ├── database/
│   │   └── database.go           # Operações com banco de dados SQLite
│   ├── models/
│   │   └── product.go            # Modelo de dados Product
│   ├── monitor/
│   │   └── monitor.go            # Sistema de monitoramento periódico
│   └── scraper/
│       ├── scraper.go            # Interface e registry de scrapers
│       └── mercadolivre.go       # Scraper do Mercado Livre
├── config/
│   └── config.go                 # Configurações da aplicação
├── go.mod                         # Dependências do projeto
├── env.sample                     # Exemplo de configuração
└── README.md                      # Este arquivo
```

## Extendendo para Outras Lojas

Para adicionar suporte a uma nova loja, você precisa:

1. Criar um novo arquivo em `internal/scraper/` (ex: `novaloja.go`) que implementa a interface `Scraper`:
```go
package scraper

type NovaLojaScraper struct {
    client *http.Client
}

func NewNovaLojaScraper() *NovaLojaScraper {
    return &NovaLojaScraper{}
}

func (n *NovaLojaScraper) CanHandle(url string) bool {
    return strings.Contains(url, "novaloja.com.br")
}

func (n *NovaLojaScraper) GetPrice(url string) (float64, error) {
    // Implementar lógica de scraping
}

func (n *NovaLojaScraper) GetName(url string) (string, error) {
    // Implementar lógica de scraping
}
```

2. Registrar o scraper no `internal/scraper/scraper.go`:
```go
func NewRegistry() *Registry {
    return &Registry{
        scrapers: []Scraper{
            NewMercadoLivreScraper(),
            NewNovaLojaScraper(),  // Adicionar aqui
        },
    }
}
```

## Banco de Dados

O bot usa SQLite para armazenar os produtos monitorados. O arquivo `products.db` é criado automaticamente na primeira execução.

### Estrutura da Tabela

- `id` - ID único do produto
- `url` - URL do produto (único)
- `name` - Nome do produto
- `current_price` - Preço atual
- `target_price` - Preço alvo (0 se não usado)
- `target_discount` - Desconto alvo em % (0 se não usado)
- `last_checked` - Data/hora da última verificação
- `active` - Se o produto está ativo (1) ou não (0)
- `created_at` - Data/hora de criação

## Notas

- O bot verifica os preços em intervalos configuráveis (padrão: 30 minutos)
- Notificações são enviadas apenas quando há mudança de preço que atende aos critérios
- O bot respeita um delay de 2 segundos entre requisições para não sobrecarregar os servidores
- Certifique-se de não fazer muitas requisições para evitar bloqueios

## Licença

Este projeto é de código aberto e está disponível para uso pessoal.

