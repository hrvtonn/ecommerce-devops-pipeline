// Package service holds the business rules. It depends on small interfaces so
// it can be unit-tested with in-memory fakes.
package service

import (
	"context"
	"time"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

type Cache interface {
	Get(ctx context.Context, key string, dest any) bool
	Set(ctx context.Context, key string, value any, ttl time.Duration)
	Del(ctx context.Context, key string)
	InvalidatePattern(ctx context.Context, pattern string)
}

type Transactor interface {
	InTx(ctx context.Context, fn func(database.DBTX) error) error
}

type CategoryStore interface {
	FindAll(ctx context.Context) ([]model.Category, error)
	FindByID(ctx context.Context, id int) (*model.Category, error)
	FindByName(ctx context.Context, name string) (*model.Category, error)
	Create(ctx context.Context, in model.CategoryInput) (model.Category, error)
	Update(ctx context.Context, id int, up model.CategoryUpdate) (*model.Category, error)
	Delete(ctx context.Context, id int) error
	CountProducts(ctx context.Context, categoryID int) (int, error)
}

type ProductStore interface {
	FindAll(ctx context.Context, f model.ProductFilter) ([]model.Product, int, error)
	FindByID(ctx context.Context, id int) (*model.Product, error)
	Create(ctx context.Context, in model.ProductInput) (model.Product, error)
	Update(ctx context.Context, id int, up model.ProductUpdate) (*model.Product, error)
	SoftDelete(ctx context.Context, id int) error
	UpdateStock(ctx context.Context, db database.DBTX, productID, delta int) (*model.Product, error)
}

type CustomerStore interface {
	FindAll(ctx context.Context, f model.CustomerFilter) ([]model.Customer, int, error)
	FindByID(ctx context.Context, id int) (*model.Customer, error)
	FindByEmail(ctx context.Context, email string) (*model.Customer, error)
	FindByCPF(ctx context.Context, cpf string) (*model.Customer, error)
	Create(ctx context.Context, in model.CustomerInput) (model.Customer, error)
	Update(ctx context.Context, id int, up model.CustomerUpdate) (*model.Customer, error)
	Delete(ctx context.Context, id int) error
	CountActiveOrders(ctx context.Context, customerID int) (int, error)
}

type OrderStore interface {
	FindAll(ctx context.Context, f model.OrderFilter) ([]model.Order, int, error)
	FindByID(ctx context.Context, id int) (*model.Order, error)
	FindItemsByOrderID(ctx context.Context, orderID int) ([]model.OrderItem, error)
	Create(ctx context.Context, db database.DBTX, customerID int, total float64, notes *string) (model.Order, error)
	CreateItem(ctx context.Context, db database.DBTX, item model.OrderItem) (model.OrderItem, error)
	UpdateStatus(ctx context.Context, db database.DBTX, id int, status string) error
}
