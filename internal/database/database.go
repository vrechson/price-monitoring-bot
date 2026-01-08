package database

import (
	"database/sql"
	"log"

	"bot-produtos/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// DB encapsula a conexão com o banco de dados
type DB struct {
	conn *sql.DB
}

// New cria uma nova instância do banco de dados
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}

	if err := db.init(); err != nil {
		conn.Close()
		return nil, err
	}

	log.Println("Banco de dados inicializado com sucesso")
	return db, nil
}

// Close fecha a conexão com o banco de dados
func (db *DB) Close() error {
	return db.conn.Close()
}

// init cria as tabelas necessárias
func (db *DB) init() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL UNIQUE,
		name TEXT,
		current_price REAL,
		original_price REAL,
		discount REAL,
		target_price REAL,
		target_discount REAL,
		last_checked DATETIME,
		active BOOLEAN DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.conn.Exec(createTableSQL); err != nil {
		return err
	}
	
	// Tentar adicionar colunas se não existirem (migração)
	// SQLite não suporta IF NOT EXISTS em ALTER TABLE, então ignoramos o erro
	_, _ = db.conn.Exec("ALTER TABLE products ADD COLUMN original_price REAL")
	_, _ = db.conn.Exec("ALTER TABLE products ADD COLUMN discount REAL")
	
	return nil
}

// AddProduct adiciona um novo produto ao banco de dados
func (db *DB) AddProduct(url, name string, targetPrice, targetDiscount float64) error {
	_, err := db.conn.Exec(
		"INSERT INTO products (url, name, current_price, original_price, target_price, target_discount, active) VALUES (?, ?, 0, 0, ?, ?, 1)",
		url, name, targetPrice, targetDiscount,
	)
	return err
}

// GetActiveProducts retorna todos os produtos ativos
func (db *DB) GetActiveProducts() ([]models.Product, error) {
	rows, err := db.conn.Query("SELECT id, url, name, current_price, original_price, discount, target_price, target_discount, last_checked, active, created_at FROM products WHERE active = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		var lastChecked sql.NullTime
		var originalPrice sql.NullFloat64
		var discount sql.NullFloat64
		err := rows.Scan(&p.ID, &p.URL, &p.Name, &p.CurrentPrice, &originalPrice, &discount, &p.TargetPrice, &p.TargetDiscount, &lastChecked, &p.Active, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		if lastChecked.Valid {
			p.LastChecked = lastChecked.Time
		}
		if originalPrice.Valid {
			p.OriginalPrice = originalPrice.Float64
		}
		if discount.Valid {
			p.Discount = discount.Float64
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

// UpdateProductPrice atualiza o preço atual de um produto
func (db *DB) UpdateProductPrice(id int64, price float64) error {
	_, err := db.conn.Exec(
		"UPDATE products SET current_price = ?, last_checked = CURRENT_TIMESTAMP WHERE id = ?",
		price, id,
	)
	return err
}

// UpdateProductPrices atualiza o preço atual e original de um produto
func (db *DB) UpdateProductPrices(id int64, currentPrice, originalPrice float64) error {
	_, err := db.conn.Exec(
		"UPDATE products SET current_price = ?, original_price = ?, last_checked = CURRENT_TIMESTAMP WHERE id = ?",
		currentPrice, originalPrice, id,
	)
	return err
}

// UpdateProductPricesWithDiscount atualiza o preço atual, original e desconto de um produto
func (db *DB) UpdateProductPricesWithDiscount(id int64, currentPrice, originalPrice, discount float64) error {
	_, err := db.conn.Exec(
		"UPDATE products SET current_price = ?, original_price = ?, discount = ?, last_checked = CURRENT_TIMESTAMP WHERE id = ?",
		currentPrice, originalPrice, discount, id,
	)
	return err
}

// DeactivateProduct desativa um produto
func (db *DB) DeactivateProduct(id int64) error {
	_, err := db.conn.Exec("UPDATE products SET active = 0 WHERE id = ?", id)
	return err
}

// GetProductByID retorna um produto pelo ID
func (db *DB) GetProductByID(id int64) (*models.Product, error) {
	var p models.Product
	var lastChecked sql.NullTime
	var originalPrice sql.NullFloat64
	var discount sql.NullFloat64
	err := db.conn.QueryRow(
		"SELECT id, url, name, current_price, original_price, discount, target_price, target_discount, last_checked, active, created_at FROM products WHERE id = ?",
		id,
	).Scan(&p.ID, &p.URL, &p.Name, &p.CurrentPrice, &originalPrice, &discount, &p.TargetPrice, &p.TargetDiscount, &lastChecked, &p.Active, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	if lastChecked.Valid {
		p.LastChecked = lastChecked.Time
	}
	if originalPrice.Valid {
		p.OriginalPrice = originalPrice.Float64
	}
	if discount.Valid {
		p.Discount = discount.Float64
	}
	return &p, nil
}

// ListProducts retorna todos os produtos (ativos e inativos)
func (db *DB) ListProducts() ([]models.Product, error) {
	rows, err := db.conn.Query("SELECT id, url, name, current_price, original_price, discount, target_price, target_discount, last_checked, active, created_at FROM products ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		var lastChecked sql.NullTime
		var originalPrice sql.NullFloat64
		var discount sql.NullFloat64
		err := rows.Scan(&p.ID, &p.URL, &p.Name, &p.CurrentPrice, &originalPrice, &discount, &p.TargetPrice, &p.TargetDiscount, &lastChecked, &p.Active, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		if lastChecked.Valid {
			p.LastChecked = lastChecked.Time
		}
		if originalPrice.Valid {
			p.OriginalPrice = originalPrice.Float64
		}
		if discount.Valid {
			p.Discount = discount.Float64
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

