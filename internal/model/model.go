package model

import "time"

// Optional is a field of a partial update: Set reports whether the client sent
// the field, and a nil Value means it was explicitly set to null.
type Optional[T any] struct {
	Set   bool
	Value *T
}

func Some[T any](v *T) Optional[T] { return Optional[T]{Set: true, Value: v} }

type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

func NewPagination(page, limit, total int) Pagination {
	return Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: (total + limit - 1) / limit,
	}
}

type Page[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// ─── Category ───────────────────────────────────────────────

type Category struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CategoryInput struct {
	Name        string
	Description *string
}

type CategoryUpdate struct {
	Name        *string
	Description Optional[string]
}

// ─── Product ────────────────────────────────────────────────

type Product struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	Description  *string    `json:"description"`
	Price        float64    `json:"price"`
	Stock        int        `json:"stock"`
	SKU          *string    `json:"sku"`
	CategoryID   int        `json:"category_id"`
	CategoryName *string    `json:"category_name,omitempty"`
	DeletedAt    *time.Time `json:"deleted_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ProductInput struct {
	Name        string
	Description *string
	Price       float64
	Stock       int
	CategoryID  int
	SKU         *string
}

type ProductUpdate struct {
	Name        *string
	Description Optional[string]
	Price       *float64
	Stock       *int
	CategoryID  *int
	SKU         Optional[string]
}

// ProductFilter is also serialized to build the list cache key.
type ProductFilter struct {
	Page       int      `json:"page"`
	Limit      int      `json:"limit"`
	Search     string   `json:"search,omitempty"`
	CategoryID int      `json:"category_id,omitempty"`
	MinPrice   *float64 `json:"min_price,omitempty"`
	MaxPrice   *float64 `json:"max_price,omitempty"`
	SortBy     string   `json:"sort_by"`
	SortOrder  string   `json:"sort_order"`
}

// ─── Customer ───────────────────────────────────────────────

type Address struct {
	Street       string  `json:"street,omitempty"`
	Number       string  `json:"number,omitempty"`
	Complement   *string `json:"complement,omitempty"`
	Neighborhood string  `json:"neighborhood,omitempty"`
	City         string  `json:"city,omitempty"`
	State        string  `json:"state,omitempty"`
	ZipCode      string  `json:"zip_code,omitempty"`
}

type Customer struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CPF       string    `json:"cpf"`
	Phone     *string   `json:"phone"`
	Address   *Address  `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CustomerInput struct {
	Name    string
	Email   string
	CPF     string
	Phone   *string
	Address *Address
}

type CustomerUpdate struct {
	Name    *string
	Email   *string
	Phone   Optional[string]
	Address Optional[Address]
}

type CustomerFilter struct {
	Page   int
	Limit  int
	Search string
}

// ─── Order ──────────────────────────────────────────────────

const (
	StatusPending   = "PENDING"
	StatusConfirmed = "CONFIRMED"
	StatusShipped   = "SHIPPED"
	StatusDelivered = "DELIVERED"
	StatusCancelled = "CANCELLED"
)

type Order struct {
	ID            int         `json:"id"`
	CustomerID    int         `json:"customer_id"`
	CustomerName  *string     `json:"customer_name,omitempty"`
	CustomerEmail *string     `json:"customer_email,omitempty"`
	Status        string      `json:"status"`
	Total         float64     `json:"total"`
	Notes         *string     `json:"notes"`
	Items         []OrderItem `json:"items,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID          int       `json:"id"`
	OrderID     int       `json:"order_id"`
	ProductID   int       `json:"product_id"`
	ProductName string    `json:"product_name"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	Subtotal    float64   `json:"subtotal"`
	CreatedAt   time.Time `json:"created_at"`
}

type OrderInput struct {
	CustomerID int
	Items      []OrderItemInput
	Notes      *string
}

type OrderItemInput struct {
	ProductID int
	Quantity  int
}

type OrderFilter struct {
	Page       int
	Limit      int
	Status     string
	CustomerID int
}
