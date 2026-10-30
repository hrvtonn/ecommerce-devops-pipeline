package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

const (
	orderColumns       = "id, customer_id, status, total, notes, created_at, updated_at"
	orderJoinedColumns = "o.id, o.customer_id, o.status, o.total, o.notes, o.created_at, o.updated_at, c.name, c.email"
	orderItemColumns   = "id, order_id, product_id, product_name, quantity, unit_price, subtotal, created_at"
)

func orderFields(o *model.Order) []any {
	return []any{&o.ID, &o.CustomerID, &o.Status, &o.Total, &o.Notes, &o.CreatedAt, &o.UpdatedAt}
}

func scanOrder(row pgx.Row) (model.Order, error) {
	var o model.Order
	err := row.Scan(orderFields(&o)...)
	return o, err
}

// scanOrderWithCustomer scans orderJoinedColumns.
func scanOrderWithCustomer(row pgx.Row) (model.Order, error) {
	var o model.Order
	err := row.Scan(append(orderFields(&o), &o.CustomerName, &o.CustomerEmail)...)
	return o, err
}

func scanOrderItem(row pgx.Row) (model.OrderItem, error) {
	var i model.OrderItem
	err := row.Scan(&i.ID, &i.OrderID, &i.ProductID, &i.ProductName, &i.Quantity, &i.UnitPrice, &i.Subtotal, &i.CreatedAt)
	return i, err
}

type OrderRepository struct {
	db database.DBTX
}

func NewOrderRepository(db database.DBTX) *OrderRepository {
	return &OrderRepository{db: db}
}

// FindAll returns a page of orders matching the filters, plus the total count.
func (r *OrderRepository) FindAll(ctx context.Context, f model.OrderFilter) ([]model.Order, int, error) {
	var conditions []string
	var a args

	if f.Status != "" {
		conditions = append(conditions, "o.status = "+a.add(f.Status))
	}
	if f.CustomerID != 0 {
		conditions = append(conditions, "o.customer_id = "+a.add(f.CustomerID))
	}
	whereClause := where(conditions)

	var total int
	if err := r.db.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM orders o "+whereClause, a...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sql := fmt.Sprintf(`SELECT %s
		FROM orders o
		LEFT JOIN customers c ON o.customer_id = c.id
		%s
		ORDER BY o.created_at DESC
		LIMIT %s OFFSET %s`,
		orderJoinedColumns, whereClause, a.add(f.Limit), a.add(offset(f.Page, f.Limit)))
	rows, err := r.db.Query(ctx, sql, a...)
	orders, err := many(rows, err, scanOrderWithCustomer)
	return orders, total, err
}

func (r *OrderRepository) FindByID(ctx context.Context, id int) (*model.Order, error) {
	return one(scanOrderWithCustomer(r.db.QueryRow(ctx,
		`SELECT `+orderJoinedColumns+`
		 FROM orders o
		 LEFT JOIN customers c ON o.customer_id = c.id
		 WHERE o.id = $1`, id)))
}

func (r *OrderRepository) FindItemsByOrderID(ctx context.Context, orderID int) ([]model.OrderItem, error) {
	rows, err := r.db.Query(ctx,
		"SELECT "+orderItemColumns+" FROM order_items WHERE order_id = $1 ORDER BY id", orderID)
	return many(rows, err, scanOrderItem)
}

// Create inserts an order; db is expected to be a transaction.
func (r *OrderRepository) Create(ctx context.Context, db database.DBTX, customerID int, total float64, notes *string) (model.Order, error) {
	return scanOrder(db.QueryRow(ctx,
		`INSERT INTO orders (customer_id, total, notes)
		 VALUES ($1, $2, $3)
		 RETURNING `+orderColumns,
		customerID, total, notes))
}

// CreateItem inserts an order item; db is expected to be a transaction.
func (r *OrderRepository) CreateItem(ctx context.Context, db database.DBTX, item model.OrderItem) (model.OrderItem, error) {
	return scanOrderItem(db.QueryRow(ctx,
		`INSERT INTO order_items (order_id, product_id, product_name, quantity, unit_price, subtotal)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+orderItemColumns,
		item.OrderID, item.ProductID, item.ProductName, item.Quantity, item.UnitPrice, item.Subtotal))
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, db database.DBTX, id int, status string) error {
	_, err := db.Exec(ctx, "UPDATE orders SET status = $1 WHERE id = $2", status, id)
	return err
}
