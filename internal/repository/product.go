package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

const (
	productColumns       = "id, name, description, price, stock, sku, category_id, deleted_at, created_at, updated_at"
	productJoinedColumns = "p.id, p.name, p.description, p.price, p.stock, p.sku, p.category_id, p.deleted_at, p.created_at, p.updated_at, c.name"
)

func productFields(p *model.Product) []any {
	return []any{&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.SKU,
		&p.CategoryID, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt}
}

func scanProduct(row pgx.Row) (model.Product, error) {
	var p model.Product
	err := row.Scan(productFields(&p)...)
	return p, err
}

// scanProductWithCategory scans productJoinedColumns.
func scanProductWithCategory(row pgx.Row) (model.Product, error) {
	var p model.Product
	err := row.Scan(append(productFields(&p), &p.CategoryName)...)
	return p, err
}

// Allow-list of sortable columns to prevent SQL injection
var productSortColumns = map[string]string{"name": "name", "price": "price", "created_at": "created_at"}

type ProductRepository struct {
	db database.DBTX
}

func NewProductRepository(db database.DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// FindAll returns a page of active products matching the filters, plus the total count.
func (r *ProductRepository) FindAll(ctx context.Context, f model.ProductFilter) ([]model.Product, int, error) {
	conditions := []string{"p.deleted_at IS NULL"}
	var a args

	if f.Search != "" {
		p := a.add("%" + f.Search + "%")
		conditions = append(conditions, fmt.Sprintf("(p.name ILIKE %[1]s OR p.description ILIKE %[1]s)", p))
	}
	if f.CategoryID != 0 {
		conditions = append(conditions, "p.category_id = "+a.add(f.CategoryID))
	}
	if f.MinPrice != nil {
		conditions = append(conditions, "p.price >= "+a.add(*f.MinPrice))
	}
	if f.MaxPrice != nil {
		conditions = append(conditions, "p.price <= "+a.add(*f.MaxPrice))
	}
	whereClause := where(conditions)

	var total int
	if err := r.db.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM products p "+whereClause, a...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sortColumn, ok := productSortColumns[f.SortBy]
	if !ok {
		sortColumn = "created_at"
	}
	sortOrder := "DESC"
	if f.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	sql := fmt.Sprintf(`SELECT %s
		FROM products p
		LEFT JOIN categories c ON p.category_id = c.id
		%s
		ORDER BY p.%s %s
		LIMIT %s OFFSET %s`,
		productJoinedColumns, whereClause, sortColumn, sortOrder, a.add(f.Limit), a.add(offset(f.Page, f.Limit)))
	rows, err := r.db.Query(ctx, sql, a...)
	products, err := many(rows, err, scanProductWithCategory)
	return products, total, err
}

func (r *ProductRepository) FindByID(ctx context.Context, id int) (*model.Product, error) {
	return one(scanProductWithCategory(r.db.QueryRow(ctx,
		`SELECT `+productJoinedColumns+`
		 FROM products p
		 LEFT JOIN categories c ON p.category_id = c.id
		 WHERE p.id = $1 AND p.deleted_at IS NULL`, id)))
}

func (r *ProductRepository) Create(ctx context.Context, in model.ProductInput) (model.Product, error) {
	return scanProduct(r.db.QueryRow(ctx,
		`INSERT INTO products (name, description, price, stock, sku, category_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+productColumns,
		in.Name, in.Description, in.Price, in.Stock, in.SKU, in.CategoryID))
}

func (r *ProductRepository) Update(ctx context.Context, id int, up model.ProductUpdate) (*model.Product, error) {
	var s setClause
	if up.Name != nil {
		s.set("name", *up.Name)
	}
	if up.Description.Set {
		s.set("description", up.Description.Value)
	}
	if up.Price != nil {
		s.set("price", *up.Price)
	}
	if up.Stock != nil {
		s.set("stock", *up.Stock)
	}
	if up.CategoryID != nil {
		s.set("category_id", *up.CategoryID)
	}
	if up.SKU.Set {
		s.set("sku", up.SKU.Value)
	}
	if s.empty() {
		return r.FindByID(ctx, id)
	}

	sql := `UPDATE products SET ` + s.String() + `
		WHERE id = ` + s.add(id) + ` AND deleted_at IS NULL
		RETURNING ` + productColumns
	return one(scanProduct(r.db.QueryRow(ctx, sql, s.args...)))
}

func (r *ProductRepository) SoftDelete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx,
		"UPDATE products SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	return err
}

// UpdateStock adds delta (negative to subtract) to the product stock. It
// returns nil when the product does not exist or the stock would go negative.
func (r *ProductRepository) UpdateStock(ctx context.Context, db database.DBTX, productID, delta int) (*model.Product, error) {
	return one(scanProduct(db.QueryRow(ctx,
		`UPDATE products SET stock = stock + $1
		 WHERE id = $2 AND deleted_at IS NULL AND stock + $1 >= 0
		 RETURNING `+productColumns,
		delta, productID)))
}
