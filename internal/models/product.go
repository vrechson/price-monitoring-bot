package models

import "time"

// Product representa um produto sendo monitorado
type Product struct {
	ID             int64
	URL            string
	Name           string
	CurrentPrice   float64
	OriginalPrice  float64 // Preço original (antes do desconto)
	Discount       float64 // Percentual de desconto atual do site (0-100)
	TargetPrice    float64
	TargetDiscount float64 // Percentual de desconto desejado (0-100)
	LastChecked    time.Time
	Active         bool
	CreatedAt      time.Time
}

