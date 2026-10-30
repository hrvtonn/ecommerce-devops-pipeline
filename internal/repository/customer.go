package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

const customerColumns = "id, name, email, cpf, phone, address, created_at, updated_at"

func scanCustomer(row pgx.Row) (model.Customer, error) {
	var c model.Customer
	err := row.Scan(&c.ID, &c.Name, &c.Email, &c.CPF, &c.Phone, &c.Address, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

type CustomerRepository struct {
	db database.DBTX
}

func NewCustomerRepository(db database.DBTX) *CustomerRepository {
	return &CustomerRepository{db: db}
}

// FindAll returns a page of customers matching the filters, plus the total count.
func (r *CustomerRepository) FindAll(ctx context.Context, f model.CustomerFilter) ([]model.Customer, int, error) {
	var conditions []string
	var a args

	if f.Search != "" {
		p := a.add("%" + f.Search + "%")
		conditions = append(conditions, fmt.Sprintf("(name ILIKE %[1]s OR email ILIKE %[1]s)", p))
	}
	whereClause := where(conditions)

	var total int
	if err := r.db.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM customers "+whereClause, a...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM customers %s
		ORDER BY created_at DESC
		LIMIT %s OFFSET %s`,
		customerColumns, whereClause, a.add(f.Limit), a.add(offset(f.Page, f.Limit)))
	rows, err := r.db.Query(ctx, sql, a...)
	customers, err := many(rows, err, scanCustomer)
	return customers, total, err
}

func (r *CustomerRepository) FindByID(ctx context.Context, id int) (*model.Customer, error) {
	return one(scanCustomer(r.db.QueryRow(ctx,
		"SELECT "+customerColumns+" FROM customers WHERE id = $1", id)))
}

func (r *CustomerRepository) FindByEmail(ctx context.Context, email string) (*model.Customer, error) {
	return one(scanCustomer(r.db.QueryRow(ctx,
		"SELECT "+customerColumns+" FROM customers WHERE email = $1", email)))
}

func (r *CustomerRepository) FindByCPF(ctx context.Context, cpf string) (*model.Customer, error) {
	return one(scanCustomer(r.db.QueryRow(ctx,
		"SELECT "+customerColumns+" FROM customers WHERE cpf = $1", cpf)))
}

func (r *CustomerRepository) Create(ctx context.Context, in model.CustomerInput) (model.Customer, error) {
	return scanCustomer(r.db.QueryRow(ctx,
		`INSERT INTO customers (name, email, cpf, phone, address)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+customerColumns,
		in.Name, in.Email, in.CPF, in.Phone, in.Address))
}

func (r *CustomerRepository) Update(ctx context.Context, id int, up model.CustomerUpdate) (*model.Customer, error) {
	var s setClause
	if up.Name != nil {
		s.set("name", *up.Name)
	}
	if up.Email != nil {
		s.set("email", *up.Email)
	}
	if up.Phone.Set {
		s.set("phone", up.Phone.Value)
	}
	if up.Address.Set {
		s.set("address", up.Address.Value)
	}
	if s.empty() {
		return r.FindByID(ctx, id)
	}

	sql := `UPDATE customers SET ` + s.String() + `
		WHERE id = ` + s.add(id) + `
		RETURNING ` + customerColumns
	return one(scanCustomer(r.db.QueryRow(ctx, sql, s.args...)))
}

func (r *CustomerRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, "DELETE FROM customers WHERE id = $1", id)
	return err
}

func (r *CustomerRepository) CountActiveOrders(ctx context.Context, customerID int) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM orders
		 WHERE customer_id = $1 AND status NOT IN ('DELIVERED', 'CANCELLED')`,
		customerID).Scan(&count)
	return count, err
}
