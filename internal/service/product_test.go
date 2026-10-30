package service

import (
	"context"
	"slices"
	"testing"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

func TestProductService_FindAll(t *testing.T) {
	ctx := context.Background()

	t.Run("returns paginated products", func(t *testing.T) {
		store := newFakeProductStore(model.Product{ID: 1, Name: "Product 1", Price: 100})
		svc := NewProductService(store, newFakeCategoryStore(), newFakeCache())

		got, err := svc.FindAll(ctx, model.ProductFilter{Page: 1, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Data) != 1 {
			t.Errorf("got %d products, want 1", len(got.Data))
		}
		want := model.Pagination{Page: 1, Limit: 20, Total: 1, TotalPages: 1}
		if got.Pagination != want {
			t.Errorf("got pagination %+v, want %+v", got.Pagination, want)
		}
	})

	t.Run("applies filters correctly", func(t *testing.T) {
		store := newFakeProductStore()
		svc := NewProductService(store, newFakeCategoryStore(), newFakeCache())

		filter := model.ProductFilter{Page: 1, Limit: 10, CategoryID: 1, Search: "phone"}
		if _, err := svc.FindAll(ctx, filter); err != nil {
			t.Fatal(err)
		}
		if store.lastFilter != filter {
			t.Errorf("repository received %+v, want %+v", store.lastFilter, filter)
		}
	})
}

func TestProductService_FindByID(t *testing.T) {
	ctx := context.Background()

	t.Run("returns a product by id", func(t *testing.T) {
		svc := NewProductService(newFakeProductStore(model.Product{ID: 1, Name: "Test"}), newFakeCategoryStore(), newFakeCache())

		got, err := svc.FindByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Test" {
			t.Errorf("got name %q", got.Name)
		}
	})

	t.Run("returns NOT_FOUND when product does not exist", func(t *testing.T) {
		svc := NewProductService(newFakeProductStore(), newFakeCategoryStore(), newFakeCache())

		if _, err := svc.FindByID(ctx, 999); errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})

	t.Run("returns cached product when available", func(t *testing.T) {
		c := newFakeCache()
		c.Set(ctx, "product:1", model.Product{ID: 1, Name: "Cached"}, 0)
		svc := NewProductService(newFakeProductStore(), newFakeCategoryStore(), c)

		got, err := svc.FindByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Cached" {
			t.Errorf("got name %q, want cached product", got.Name)
		}
	})
}

func TestProductService_Create(t *testing.T) {
	ctx := context.Background()
	in := model.ProductInput{Name: "New", Price: 99.99, Stock: 10, CategoryID: 1}

	t.Run("creates a product with valid category", func(t *testing.T) {
		c := newFakeCache()
		svc := NewProductService(newFakeProductStore(), newFakeCategoryStore(model.Category{ID: 1}), c)

		got, err := svc.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "New" {
			t.Errorf("got name %q", got.Name)
		}
		if !slices.Contains(c.invalidated, "products:*") {
			t.Error("expected products:* to be invalidated")
		}
	})

	t.Run("returns NOT_FOUND when category does not exist", func(t *testing.T) {
		svc := NewProductService(newFakeProductStore(), newFakeCategoryStore(), newFakeCache())

		if _, err := svc.Create(ctx, in); errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}

func TestProductService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("updates an existing product", func(t *testing.T) {
		c := newFakeCache()
		svc := NewProductService(newFakeProductStore(model.Product{ID: 1, Name: "Old"}), newFakeCategoryStore(), c)

		got, err := svc.Update(ctx, 1, model.ProductUpdate{Name: ptr("Updated")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Updated" {
			t.Errorf("got name %q", got.Name)
		}
		if !slices.Contains(c.deleted, "product:1") || !slices.Contains(c.invalidated, "products:list:*") {
			t.Error("expected product caches to be invalidated")
		}
	})

	t.Run("validates new category on update", func(t *testing.T) {
		svc := NewProductService(newFakeProductStore(model.Product{ID: 1, Name: "P"}), newFakeCategoryStore(), newFakeCache())

		_, err := svc.Update(ctx, 1, model.ProductUpdate{CategoryID: ptr(999)})
		if errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}

func TestProductService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("soft deletes a product", func(t *testing.T) {
		store := newFakeProductStore(model.Product{ID: 1, Name: "P"})
		svc := NewProductService(store, newFakeCategoryStore(), newFakeCache())

		if err := svc.Delete(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(store.softDeleted, 1) {
			t.Error("expected product to be soft deleted")
		}
	})

	t.Run("returns NOT_FOUND when product does not exist", func(t *testing.T) {
		svc := NewProductService(newFakeProductStore(), newFakeCategoryStore(), newFakeCache())

		if err := svc.Delete(ctx, 999); errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}
