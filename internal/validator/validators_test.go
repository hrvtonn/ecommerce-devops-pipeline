package validator

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
)

// body decodes a JSON literal the same way the HTTP layer does.
func body(t *testing.T, s string) any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func details(t *testing.T, err error) []apperror.FieldError {
	t.Helper()
	var vErr *apperror.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	return vErr.Details
}

func hasDetail(list []apperror.FieldError, field, message string) bool {
	for _, d := range list {
		if d.Field == field && d.Message == message {
			return true
		}
	}
	return false
}

func TestIsValidCPF(t *testing.T) {
	tests := []struct {
		cpf  string
		want bool
	}{
		{"52998224725", true},
		{"529.982.247-25", true},
		{"11111111111", false},
		{"00000000000", false},
		{"52998224726", false},
		{"12345678900", false},
		{"123", false},
		{"123456789012", false},
	}
	for _, tt := range tests {
		if got := IsValidCPF(tt.cpf); got != tt.want {
			t.Errorf("IsValidCPF(%q) = %v, want %v", tt.cpf, got, tt.want)
		}
	}
}

func TestCreateProduct(t *testing.T) {
	t.Run("validates a valid product", func(t *testing.T) {
		in, err := CreateProduct(body(t, `{"name": "  Test Product ", "price": 99.999, "stock": 10, "category_id": 1, "extra": true}`))
		if err != nil {
			t.Fatal(err)
		}
		if in.Name != "Test Product" || in.Price != 100 || in.Stock != 10 || in.CategoryID != 1 {
			t.Errorf("got %+v", in)
		}
		if in.Description == nil || *in.Description != "" || in.SKU != nil {
			t.Error("expected default description '' and null sku")
		}
	})

	tests := []struct {
		name, json, field, message string
	}{
		{"requires name", `{"price": 10, "stock": 1, "category_id": 1}`, "name", `"name" is required`},
		{"rejects short name", `{"name": "A", "price": 10, "stock": 1, "category_id": 1}`, "name", "Product name must have at least 2 characters"},
		{"requires positive price", `{"name": "Test", "price": -10, "stock": 1, "category_id": 1}`, "price", "Price must be greater than zero"},
		{"rejects zero price", `{"name": "Test", "price": 0, "stock": 1, "category_id": 1}`, "price", "Price must be greater than zero"},
		{"rejects negative stock", `{"name": "Test", "price": 10, "stock": -1, "category_id": 1}`, "stock", "Stock cannot be negative"},
		{"rejects fractional stock", `{"name": "Test", "price": 10, "stock": 1.5, "category_id": 1}`, "stock", `"stock" must be an integer`},
		{"requires category_id", `{"name": "Test", "price": 10, "stock": 1}`, "category_id", "Category is required"},
		{"rejects non-numeric price", `{"name": "Test", "price": "abc", "stock": 1, "category_id": 1}`, "price", `"price" must be a number`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateProduct(body(t, tt.json))
			if d := details(t, err); !hasDetail(d, tt.field, tt.message) {
				t.Errorf("got %+v, want {%s %s}", d, tt.field, tt.message)
			}
		})
	}

	t.Run("reports every invalid field", func(t *testing.T) {
		_, err := CreateProduct(body(t, `{}`))
		if d := details(t, err); len(d) != 4 {
			t.Errorf("got %d errors, want 4: %+v", len(d), d)
		}
	})

	t.Run("rejects a non-object body", func(t *testing.T) {
		_, err := CreateProduct(body(t, `[1, 2]`))
		if d := details(t, err); !hasDetail(d, "", `"value" must be of type object`) {
			t.Errorf("got %+v", d)
		}
	})
}

func TestUpdateProduct(t *testing.T) {
	t.Run("requires at least one field", func(t *testing.T) {
		_, err := UpdateProduct(body(t, `{"unknown": 1}`))
		if d := details(t, err); !hasDetail(d, "", "At least one field must be provided for update") {
			t.Errorf("got %+v", d)
		}
	})

	t.Run("allows partial updates", func(t *testing.T) {
		up, err := UpdateProduct(body(t, `{"price": 49.99, "sku": null}`))
		if err != nil {
			t.Fatal(err)
		}
		if up.Price == nil || *up.Price != 49.99 || up.Name != nil {
			t.Errorf("got %+v", up)
		}
		if !up.SKU.Set || up.SKU.Value != nil {
			t.Error("expected sku to be explicitly set to null")
		}
		if up.Description.Set {
			t.Error("description must not be set")
		}
	})

	t.Run("rejects invalid price", func(t *testing.T) {
		_, err := UpdateProduct(body(t, `{"price": -5}`))
		if d := details(t, err); !hasDetail(d, "price", "Price must be greater than zero") {
			t.Errorf("got %+v", d)
		}
	})
}

func TestProductQuery(t *testing.T) {
	t.Run("applies defaults", func(t *testing.T) {
		f, err := ProductQuery(url.Values{})
		if err != nil {
			t.Fatal(err)
		}
		if f.Page != 1 || f.Limit != 20 || f.SortBy != "created_at" || f.SortOrder != "desc" {
			t.Errorf("got %+v", f)
		}
	})

	t.Run("parses filters", func(t *testing.T) {
		f, err := ProductQuery(url.Values{"page": {"2"}, "min_price": {"10.5"}, "sort_by": {"price"}, "category_id": {"3"}})
		if err != nil {
			t.Fatal(err)
		}
		if f.Page != 2 || *f.MinPrice != 10.5 || f.SortBy != "price" || f.CategoryID != 3 {
			t.Errorf("got %+v", f)
		}
	})

	t.Run("rejects invalid values", func(t *testing.T) {
		_, err := ProductQuery(url.Values{"limit": {"500"}, "sort_by": {"stock"}})
		d := details(t, err)
		if !hasDetail(d, "limit", `"limit" must be less than or equal to 100`) ||
			!hasDetail(d, "sort_by", `"sort_by" must be one of [name, price, created_at]`) {
			t.Errorf("got %+v", d)
		}
	})
}

func TestCategoryValidators(t *testing.T) {
	t.Run("rejects short category name", func(t *testing.T) {
		_, err := CreateCategory(body(t, `{"name": "A"}`))
		if d := details(t, err); !hasDetail(d, "name", "Category name must have at least 2 characters") {
			t.Errorf("got %+v", d)
		}
	})

	t.Run("allows clearing the description", func(t *testing.T) {
		up, err := UpdateCategory(body(t, `{"description": null}`))
		if err != nil {
			t.Fatal(err)
		}
		if !up.Description.Set || up.Description.Value != nil {
			t.Errorf("got %+v", up)
		}
	})
}

func TestCreateCustomer(t *testing.T) {
	t.Run("normalizes a valid customer", func(t *testing.T) {
		in, err := CreateCustomer(body(t, `{
			"name": "João Silva", "email": " JOAO@Email.com ", "cpf": "529.982.247-25",
			"address": {"street": "Rua das Flores", "state": "sp", "complement": null}
		}`))
		if err != nil {
			t.Fatal(err)
		}
		if in.Email != "joao@email.com" || in.CPF != "52998224725" {
			t.Errorf("got %+v", in)
		}
		if in.Address == nil || in.Address.State != "SP" || in.Address.Complement != nil {
			t.Errorf("got address %+v", in.Address)
		}
	})

	tests := []struct {
		name, json, field, message string
	}{
		{"rejects invalid CPF", `{"name": "João", "email": "j@e.com", "cpf": "11111111111"}`, "cpf", "Invalid CPF number"},
		{"rejects invalid email", `{"name": "João", "email": "not-an-email", "cpf": "52998224725"}`, "email", `"email" must be a valid email`},
		{"validates nested address", `{"name": "João", "email": "j@e.com", "cpf": "52998224725", "address": {"state": "SPX"}}`,
			"address.state", `"address.state" length must be 2 characters long`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateCustomer(body(t, tt.json))
			if d := details(t, err); !hasDetail(d, tt.field, tt.message) {
				t.Errorf("got %+v, want {%s %s}", d, tt.field, tt.message)
			}
		})
	}
}

func TestOrderValidators(t *testing.T) {
	t.Run("validates a valid order", func(t *testing.T) {
		in, err := CreateOrder(body(t, `{"customer_id": 1, "items": [{"product_id": 2, "quantity": "3"}], "notes": " rápido "}`))
		if err != nil {
			t.Fatal(err)
		}
		if in.CustomerID != 1 || len(in.Items) != 1 || in.Items[0].Quantity != 3 || *in.Notes != "rápido" {
			t.Errorf("got %+v", in)
		}
	})

	tests := []struct {
		name, json, field, message string
	}{
		{"requires customer", `{"items": [{"product_id": 1, "quantity": 1}]}`, "customer_id", "Customer is required"},
		{"requires at least one item", `{"customer_id": 1, "items": []}`, "items", "Order must have at least one item"},
		{"validates item quantity", `{"customer_id": 1, "items": [{"product_id": 1, "quantity": 0}]}`, "items.0.quantity", "Quantity must be at least 1"},
		{"validates item product", `{"customer_id": 1, "items": [{"quantity": 1}]}`, "items.0.product_id", `"items[0].product_id" is required`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateOrder(body(t, tt.json))
			if d := details(t, err); !hasDetail(d, tt.field, tt.message) {
				t.Errorf("got %+v, want {%s %s}", d, tt.field, tt.message)
			}
		})
	}

	t.Run("rejects unknown status", func(t *testing.T) {
		_, err := UpdateOrderStatus(body(t, `{"status": "PENDING"}`))
		if d := details(t, err); !hasDetail(d, "status", "Status must be one of: CONFIRMED, SHIPPED, DELIVERED, CANCELLED") {
			t.Errorf("got %+v", d)
		}
	})
}
