package service

import (
	"context"
	"slices"
	"testing"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

func TestCustomerService_FindAll(t *testing.T) {
	ctx := context.Background()

	t.Run("returns paginated customers", func(t *testing.T) {
		svc := NewCustomerService(newFakeCustomerStore(model.Customer{ID: 1, Name: "João"}), newFakeCache())

		got, err := svc.FindAll(ctx, model.CustomerFilter{Page: 1, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Data) != 1 || got.Pagination.Total != 1 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("applies search filter", func(t *testing.T) {
		store := newFakeCustomerStore()
		svc := NewCustomerService(store, newFakeCache())

		if _, err := svc.FindAll(ctx, model.CustomerFilter{Page: 1, Limit: 20, Search: "joão"}); err != nil {
			t.Fatal(err)
		}
		if store.lastFilter.Search != "joão" {
			t.Errorf("repository received search %q", store.lastFilter.Search)
		}
	})
}

func TestCustomerService_FindByID(t *testing.T) {
	ctx := context.Background()
	svc := NewCustomerService(newFakeCustomerStore(model.Customer{ID: 1, Name: "João"}), newFakeCache())

	t.Run("returns a customer by id", func(t *testing.T) {
		got, err := svc.FindByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "João" {
			t.Errorf("got name %q", got.Name)
		}
	})

	t.Run("returns NOT_FOUND when customer does not exist", func(t *testing.T) {
		if _, err := svc.FindByID(ctx, 999); errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}

func TestCustomerService_Create(t *testing.T) {
	ctx := context.Background()
	in := model.CustomerInput{Name: "João", Email: "joao@email.com", CPF: "52998224725"}

	t.Run("creates a customer with unique email and CPF", func(t *testing.T) {
		store := newFakeCustomerStore()
		svc := NewCustomerService(store, newFakeCache())

		got, err := svc.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Email != in.Email || len(store.created) != 1 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("returns CONFLICT when email already exists", func(t *testing.T) {
		svc := NewCustomerService(newFakeCustomerStore(model.Customer{ID: 1, Email: in.Email}), newFakeCache())

		if _, err := svc.Create(ctx, in); errCode(err) != "CONFLICT" {
			t.Errorf("got %v, want CONFLICT", err)
		}
	})

	t.Run("returns CONFLICT when CPF already exists", func(t *testing.T) {
		svc := NewCustomerService(newFakeCustomerStore(model.Customer{ID: 1, Email: "other@email.com", CPF: in.CPF}), newFakeCache())

		_, err := svc.Create(ctx, in)
		if errCode(err) != "CONFLICT" || err.Error() != "Customer with this CPF already exists" {
			t.Errorf("got %v, want CPF CONFLICT", err)
		}
	})
}

func TestCustomerService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("updates a customer", func(t *testing.T) {
		c := newFakeCache()
		svc := NewCustomerService(newFakeCustomerStore(model.Customer{ID: 1, Name: "Old", Email: "a@a.com"}), c)

		got, err := svc.Update(ctx, 1, model.CustomerUpdate{Name: ptr("New")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "New" {
			t.Errorf("got name %q", got.Name)
		}
		if !slices.Contains(c.deleted, "customer:1") {
			t.Error("expected customer:1 to be removed from cache")
		}
	})

	t.Run("returns CONFLICT when updating to an existing email", func(t *testing.T) {
		svc := NewCustomerService(newFakeCustomerStore(
			model.Customer{ID: 1, Email: "a@a.com"}, model.Customer{ID: 2, Email: "taken@email.com"}), newFakeCache())

		_, err := svc.Update(ctx, 1, model.CustomerUpdate{Email: ptr("taken@email.com")})
		if errCode(err) != "CONFLICT" {
			t.Errorf("got %v, want CONFLICT", err)
		}
	})

	t.Run("allows keeping the same email", func(t *testing.T) {
		svc := NewCustomerService(newFakeCustomerStore(model.Customer{ID: 1, Email: "same@email.com"}), newFakeCache())

		if _, err := svc.Update(ctx, 1, model.CustomerUpdate{Email: ptr("same@email.com")}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCustomerService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes a customer with no active orders", func(t *testing.T) {
		store := newFakeCustomerStore(model.Customer{ID: 1})
		svc := NewCustomerService(store, newFakeCache())

		if err := svc.Delete(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(store.deleted, 1) {
			t.Error("expected customer to be deleted")
		}
	})

	t.Run("returns BUSINESS_RULE_VIOLATION when customer has active orders", func(t *testing.T) {
		store := newFakeCustomerStore(model.Customer{ID: 1})
		store.activeOrders[1] = 2
		svc := NewCustomerService(store, newFakeCache())

		if err := svc.Delete(ctx, 1); errCode(err) != "BUSINESS_RULE_VIOLATION" {
			t.Errorf("got %v, want BUSINESS_RULE_VIOLATION", err)
		}
	})
}
