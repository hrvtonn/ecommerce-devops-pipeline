package service

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/cache"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

// Valid status transitions
var statusTransitions = map[string][]string{
	model.StatusPending:   {model.StatusConfirmed, model.StatusCancelled},
	model.StatusConfirmed: {model.StatusShipped, model.StatusCancelled},
	model.StatusShipped:   {model.StatusDelivered},
	model.StatusDelivered: {},
	model.StatusCancelled: {},
}

type OrderService struct {
	store     OrderStore
	products  ProductStore
	customers CustomerStore
	tx        Transactor
	cache     Cache
}

func NewOrderService(store OrderStore, products ProductStore, customers CustomerStore, tx Transactor, cache Cache) *OrderService {
	return &OrderService{store: store, products: products, customers: customers, tx: tx, cache: cache}
}

func (s *OrderService) FindAll(ctx context.Context, f model.OrderFilter) (*model.Page[model.Order], error) {
	orders, total, err := s.store.FindAll(ctx, f)
	if err != nil {
		return nil, err
	}
	return &model.Page[model.Order]{
		Data:       orders,
		Pagination: model.NewPagination(f.Page, f.Limit, total),
	}, nil
}

func (s *OrderService) FindByID(ctx context.Context, id int) (*model.Order, error) {
	cacheKey := fmt.Sprintf("order:%d", id)
	var cached model.Order
	if s.cache.Get(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	order, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, apperror.NotFound("Order", id)
	}

	order.Items, err = s.store.FindItemsByOrderID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cache.Set(ctx, cacheKey, order, cache.TTL("order"))
	return order, nil
}

func (s *OrderService) Create(ctx context.Context, in model.OrderInput) (*model.Order, error) {
	// Validate customer exists
	customer, err := s.customers.FindByID(ctx, in.CustomerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, apperror.NotFound("Customer", in.CustomerID)
	}

	// Validate products and calculate total
	details := make([]model.OrderItem, 0, len(in.Items))
	total := 0.0
	for _, item := range in.Items {
		product, err := s.products.FindByID(ctx, item.ProductID)
		if err != nil {
			return nil, err
		}
		if product == nil {
			return nil, apperror.NotFound("Product", item.ProductID)
		}

		if product.Stock < item.Quantity {
			return nil, apperror.Business(fmt.Sprintf(
				"Insufficient stock for product %q: requested %d, available %d",
				product.Name, item.Quantity, product.Stock))
		}

		subtotal := roundMoney(product.Price * float64(item.Quantity))
		details = append(details, model.OrderItem{
			ProductID:   item.ProductID,
			ProductName: product.Name,
			Quantity:    item.Quantity,
			UnitPrice:   product.Price,
			Subtotal:    subtotal,
		})
		total += subtotal
	}

	// Execute within a transaction: create order, items, and decrement stock
	var order model.Order
	err = s.tx.InTx(ctx, func(db database.DBTX) error {
		var err error
		order, err = s.store.Create(ctx, db, in.CustomerID, roundMoney(total), in.Notes)
		if err != nil {
			return err
		}

		for _, item := range details {
			item.OrderID = order.ID
			created, err := s.store.CreateItem(ctx, db, item)
			if err != nil {
				return err
			}
			order.Items = append(order.Items, created)

			// Decrement stock
			updated, err := s.products.UpdateStock(ctx, db, item.ProductID, -item.Quantity)
			if err != nil {
				return err
			}
			if updated == nil {
				return apperror.Business(fmt.Sprintf(
					"Failed to update stock for product %q. Insufficient stock.", item.ProductName))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Invalidate product caches since stock changed
	s.invalidateProducts(ctx, details)

	order.CustomerName = &customer.Name
	order.CustomerEmail = &customer.Email
	return &order, nil
}

func (s *OrderService) UpdateStatus(ctx context.Context, id int, newStatus string) (*model.Order, error) {
	order, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, apperror.NotFound("Order", id)
	}

	allowed := statusTransitions[order.Status]
	if !slices.Contains(allowed, newStatus) {
		allowedText := "none"
		if len(allowed) > 0 {
			allowedText = strings.Join(allowed, ", ")
		}
		return nil, apperror.Business(fmt.Sprintf(
			"Cannot transition order status from %q to %q. Allowed transitions: %s",
			order.Status, newStatus, allowedText))
	}

	// If cancelling, restore stock
	var items []model.OrderItem
	if newStatus == model.StatusCancelled {
		if items, err = s.store.FindItemsByOrderID(ctx, id); err != nil {
			return nil, err
		}
	}

	err = s.tx.InTx(ctx, func(db database.DBTX) error {
		if err := s.store.UpdateStatus(ctx, db, id, newStatus); err != nil {
			return err
		}
		for _, item := range items {
			if _, err := s.products.UpdateStock(ctx, db, item.ProductID, item.Quantity); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(items) > 0 {
		// Invalidate product caches since stock was restored
		s.invalidateProducts(ctx, items)
	}
	s.cache.Del(ctx, fmt.Sprintf("order:%d", id))

	updated, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, apperror.NotFound("Order", id)
	}
	return updated, nil
}

func (s *OrderService) invalidateProducts(ctx context.Context, items []model.OrderItem) {
	s.cache.InvalidatePattern(ctx, "products:*")
	for _, item := range items {
		s.cache.Del(ctx, fmt.Sprintf("product:%d", item.ProductID))
	}
}

func roundMoney(v float64) float64 { return math.Round(v*100) / 100 }
