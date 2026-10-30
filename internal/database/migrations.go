package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS categories (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL UNIQUE,
		description VARCHAR(500) DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS products (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		description TEXT DEFAULT '',
		price DECIMAL(10, 2) NOT NULL CHECK (price > 0),
		stock INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
		sku VARCHAR(100) UNIQUE,
		category_id INTEGER NOT NULL REFERENCES categories(id),
		deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS customers (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL UNIQUE,
		cpf VARCHAR(11) NOT NULL UNIQUE,
		phone VARCHAR(20),
		address JSONB DEFAULT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		customer_id INTEGER NOT NULL REFERENCES customers(id),
		status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
			CHECK (status IN ('PENDING', 'CONFIRMED', 'SHIPPED', 'DELIVERED', 'CANCELLED')),
		total DECIMAL(12, 2) NOT NULL DEFAULT 0,
		notes TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS order_items (
		id SERIAL PRIMARY KEY,
		order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
		product_id INTEGER NOT NULL REFERENCES products(id),
		product_name VARCHAR(255) NOT NULL,
		quantity INTEGER NOT NULL CHECK (quantity > 0),
		unit_price DECIMAL(10, 2) NOT NULL,
		subtotal DECIMAL(12, 2) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	// Indexes for performance
	`CREATE INDEX IF NOT EXISTS idx_products_category_id ON products(category_id);
	CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);
	CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
	CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
	CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
	CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
	CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email);
	CREATE INDEX IF NOT EXISTS idx_customers_cpf ON customers(cpf)`,

	// Function for auto-updating updated_at
	`CREATE OR REPLACE FUNCTION update_updated_at_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = NOW();
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql`,
}

// Migrate creates the schema (idempotently) within a single transaction.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for _, stmt := range schema {
			if _, err := tx.Exec(ctx, stmt); err != nil {
				return err
			}
		}

		// Triggers for updated_at
		for _, table := range []string{"categories", "products", "customers", "orders"} {
			stmt := fmt.Sprintf(`DROP TRIGGER IF EXISTS set_updated_at ON %[1]s;
				CREATE TRIGGER set_updated_at
					BEFORE UPDATE ON %[1]s
					FOR EACH ROW
					EXECUTE FUNCTION update_updated_at_column()`, table)
			if _, err := tx.Exec(ctx, stmt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("Migration failed", "error", err)
		return err
	}

	slog.Info("Database migrations completed successfully")
	return nil
}
