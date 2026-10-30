package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

const categoryColumns = "id, name, description, created_at, updated_at"

func scanCategory(row pgx.Row) (model.Category, error) {
	var c model.Category
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

type CategoryRepository struct {
	db database.DBTX
}

func NewCategoryRepository(db database.DBTX) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) FindAll(ctx context.Context) ([]model.Category, error) {
	rows, err := r.db.Query(ctx, "SELECT "+categoryColumns+" FROM categories ORDER BY name ASC")
	return many(rows, err, scanCategory)
}

func (r *CategoryRepository) FindByID(ctx context.Context, id int) (*model.Category, error) {
	return one(scanCategory(r.db.QueryRow(ctx,
		"SELECT "+categoryColumns+" FROM categories WHERE id = $1", id)))
}

func (r *CategoryRepository) FindByName(ctx context.Context, name string) (*model.Category, error) {
	return one(scanCategory(r.db.QueryRow(ctx,
		"SELECT "+categoryColumns+" FROM categories WHERE LOWER(name) = LOWER($1)", name)))
}

func (r *CategoryRepository) Create(ctx context.Context, in model.CategoryInput) (model.Category, error) {
	return scanCategory(r.db.QueryRow(ctx,
		`INSERT INTO categories (name, description)
		 VALUES ($1, $2)
		 RETURNING `+categoryColumns,
		in.Name, in.Description))
}

func (r *CategoryRepository) Update(ctx context.Context, id int, up model.CategoryUpdate) (*model.Category, error) {
	var s setClause
	if up.Name != nil {
		s.set("name", *up.Name)
	}
	if up.Description.Set {
		s.set("description", up.Description.Value)
	}
	if s.empty() {
		return r.FindByID(ctx, id)
	}

	sql := `UPDATE categories SET ` + s.String() + `
		WHERE id = ` + s.add(id) + `
		RETURNING ` + categoryColumns
	return one(scanCategory(r.db.QueryRow(ctx, sql, s.args...)))
}

func (r *CategoryRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, "DELETE FROM categories WHERE id = $1", id)
	return err
}

func (r *CategoryRepository) CountProducts(ctx context.Context, categoryID int) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM products WHERE category_id = $1 AND deleted_at IS NULL",
		categoryID).Scan(&count)
	return count, err
}
