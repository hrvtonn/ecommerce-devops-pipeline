package service

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

type orderFixture struct {
	orders    *fakeOrderStore
	products  *fakeProductStore
	customers *fakeCustomerStore
	tx        *fakeTx
	cache     *fakeCache
	svc       *OrderService
}

func newOrderFixture(orders ...model.Order) *orderFixture {
	f := &orderFixture{
		orders: newFakeOrderStore(orders...),
		products: newFakeProductStore(
			model.Product{ID: 1, Name: "Smartphone", Price: 2499.99, Stock: 10},
			model.Product{ID: 2, Name: "Fone", Price: 0.1, Stock: 5},
		),
		customers: newFakeCustomerStore(model.Customer{ID: 1, Name: "João", Email: "joao@email.com"}),
		tx:        &fakeTx{},
		cache:     newFakeCache(),
	}
	f.svc = NewOrderService(f.orders, f.products, f.customers, f.tx, f.cache)
	return f
}

func TestOrderService_FindAll(t *testing.T) {
	ctx := context.Background()

	t.Run("returns paginated orders", func(t *testing.T) {
		f := newOrderFixture(model.Order{ID: 1, Status: model.StatusPending, Total: 100})

		got, err := f.svc.FindAll(ctx, model.OrderFilter{Page: 1, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Data) != 1 || got.Pagination.Total != 1 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		f := newOrderFixture()

		if _, err := f.svc.FindAll(ctx, model.OrderFilter{Page: 1, Limit: 20, Status: model.StatusPending}); err != nil {
			t.Fatal(err)
		}
		if f.orders.lastFilter.Status != model.StatusPending {
			t.Errorf("repository received status %q", f.orders.lastFilter.Status)
		}
	})
}

func TestOrderService_FindByID(t *testing.T) {
	ctx := context.Background()

	t.Run("returns order with items", func(t *testing.T) {
		f := newOrderFixture(model.Order{ID: 1, Status: model.StatusPending, Total: 2499.99,
			Items: []model.OrderItem{{ID: 1, ProductName: "Smartphone", Quantity: 1, UnitPrice: 2499.99}}})

		got, err := f.svc.FindByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Items) != 1 || got.Items[0].ProductName != "Smartphone" {
			t.Errorf("got items %+v", got.Items)
		}
		if _, ok := f.cache.data["order:1"]; !ok {
			t.Error("expected order to be cached")
		}
	})

	t.Run("returns NOT_FOUND when order does not exist", func(t *testing.T) {
		f := newOrderFixture()

		if _, err := f.svc.FindByID(ctx, 999); errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}

func TestOrderService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates an order and decrements stock", func(t *testing.T) {
		f := newOrderFixture()
		in := model.OrderInput{CustomerID: 1, Items: []model.OrderItemInput{
			{ProductID: 1, Quantity: 2},
			{ProductID: 2, Quantity: 3},
		}}

		got, err := f.svc.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != 5000.28 {
			t.Errorf("got total %v, want 5000.28", got.Total)
		}
		if len(got.Items) != 2 || got.Items[0].Subtotal != 4999.98 {
			t.Errorf("got items %+v", got.Items)
		}
		if got.CustomerName == nil || *got.CustomerName != "João" {
			t.Error("expected customer name to be filled")
		}
		if f.products.items[1].Stock != 8 || f.products.items[2].Stock != 2 {
			t.Error("expected stock to be decremented")
		}
		if f.tx.calls != 1 {
			t.Errorf("got %d transactions, want 1", f.tx.calls)
		}
		if !slices.Contains(f.cache.invalidated, "products:*") || !slices.Contains(f.cache.deleted, "product:1") {
			t.Error("expected product caches to be invalidated")
		}
	})

	t.Run("returns NOT_FOUND when customer does not exist", func(t *testing.T) {
		f := newOrderFixture()

		_, err := f.svc.Create(ctx, model.OrderInput{CustomerID: 999, Items: []model.OrderItemInput{{ProductID: 1, Quantity: 1}}})
		if errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})

	t.Run("returns NOT_FOUND when product does not exist", func(t *testing.T) {
		f := newOrderFixture()

		_, err := f.svc.Create(ctx, model.OrderInput{CustomerID: 1, Items: []model.OrderItemInput{{ProductID: 999, Quantity: 1}}})
		if errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})

	t.Run("returns BUSINESS_RULE_VIOLATION when stock is insufficient", func(t *testing.T) {
		f := newOrderFixture()

		_, err := f.svc.Create(ctx, model.OrderInput{CustomerID: 1, Items: []model.OrderItemInput{{ProductID: 1, Quantity: 100}}})
		if errCode(err) != "BUSINESS_RULE_VIOLATION" || !strings.Contains(err.Error(), "Insufficient stock") {
			t.Errorf("got %v, want insufficient stock error", err)
		}
		if f.tx.calls != 0 {
			t.Error("no transaction should be started")
		}
	})

	t.Run("fails when stock update is rejected inside the transaction", func(t *testing.T) {
		f := newOrderFixture()
		f.products.failStock = true

		_, err := f.svc.Create(ctx, model.OrderInput{CustomerID: 1, Items: []model.OrderItemInput{{ProductID: 1, Quantity: 1}}})
		if errCode(err) != "BUSINESS_RULE_VIOLATION" || !strings.Contains(err.Error(), "Failed to update stock") {
			t.Errorf("got %v, want failed stock update error", err)
		}
	})
}

func TestOrderService_UpdateStatus(t *testing.T) {
	ctx := context.Background()

	transitions := []struct{ from, to string }{
		{model.StatusPending, model.StatusConfirmed},
		{model.StatusConfirmed, model.StatusShipped},
		{model.StatusShipped, model.StatusDelivered},
	}
	for _, tt := range transitions {
		t.Run("transitions from "+tt.from+" to "+tt.to, func(t *testing.T) {
			f := newOrderFixture(model.Order{ID: 1, Status: tt.from})

			got, err := f.svc.UpdateStatus(ctx, 1, tt.to)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.to {
				t.Errorf("got status %q, want %q", got.Status, tt.to)
			}
			if !slices.Contains(f.cache.deleted, "order:1") {
				t.Error("expected order:1 to be removed from cache")
			}
		})
	}

	t.Run("rejects invalid transition from DELIVERED", func(t *testing.T) {
		f := newOrderFixture(model.Order{ID: 1, Status: model.StatusDelivered})

		_, err := f.svc.UpdateStatus(ctx, 1, model.StatusCancelled)
		if errCode(err) != "BUSINESS_RULE_VIOLATION" || !strings.HasSuffix(err.Error(), "Allowed transitions: none") {
			t.Errorf("got %v, want BUSINESS_RULE_VIOLATION", err)
		}
	})

	t.Run("rejects shipping a pending order", func(t *testing.T) {
		f := newOrderFixture(model.Order{ID: 1, Status: model.StatusPending})

		_, err := f.svc.UpdateStatus(ctx, 1, model.StatusShipped)
		want := `Cannot transition order status from "PENDING" to "SHIPPED". Allowed transitions: CONFIRMED, CANCELLED`
		if err == nil || err.Error() != want {
			t.Errorf("got %v, want %q", err, want)
		}
	})

	t.Run("restores stock when cancelling an order", func(t *testing.T) {
		f := newOrderFixture(model.Order{ID: 1, Status: model.StatusPending,
			Items: []model.OrderItem{{ProductID: 1, Quantity: 2}, {ProductID: 2, Quantity: 1}}})

		got, err := f.svc.UpdateStatus(ctx, 1, model.StatusCancelled)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != model.StatusCancelled {
			t.Errorf("got status %q", got.Status)
		}
		if f.products.items[1].Stock != 12 || f.products.items[2].Stock != 6 {
			t.Error("expected stock to be restored")
		}
		if !slices.Contains(f.cache.invalidated, "products:*") {
			t.Error("expected product caches to be invalidated")
		}
	})

	t.Run("returns NOT_FOUND when order does not exist", func(t *testing.T) {
		f := newOrderFixture()

		if _, err := f.svc.UpdateStatus(ctx, 999, model.StatusConfirmed); errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}
