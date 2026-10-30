package service

import (
	"context"
	"slices"
	"testing"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

func TestCategoryService_FindAll(t *testing.T) {
	ctx := context.Background()

	t.Run("returns all categories", func(t *testing.T) {
		store := newFakeCategoryStore(model.Category{ID: 1, Name: "Eletrônicos"}, model.Category{ID: 2, Name: "Livros"})
		c := newFakeCache()
		svc := NewCategoryService(store, c)

		got, err := svc.FindAll(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d categories, want 2", len(got))
		}
		if _, ok := c.data["categories:list"]; !ok {
			t.Error("expected result to be cached")
		}
	})

	t.Run("returns cached data when available", func(t *testing.T) {
		store := newFakeCategoryStore()
		c := newFakeCache()
		c.Set(ctx, "categories:list", []model.Category{{ID: 1, Name: "Cached"}}, 0)
		svc := NewCategoryService(store, c)

		got, err := svc.FindAll(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "Cached" {
			t.Errorf("got %+v, want cached category", got)
		}
		if store.findAllCalls != 0 {
			t.Error("repository should not be queried on cache hit")
		}
	})
}

func TestCategoryService_FindByID(t *testing.T) {
	ctx := context.Background()
	svc := NewCategoryService(newFakeCategoryStore(model.Category{ID: 1, Name: "Eletrônicos"}), newFakeCache())

	t.Run("returns a category by id", func(t *testing.T) {
		got, err := svc.FindByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Eletrônicos" {
			t.Errorf("got name %q", got.Name)
		}
	})

	t.Run("returns NOT_FOUND when category does not exist", func(t *testing.T) {
		_, err := svc.FindByID(ctx, 999)
		if errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}

func TestCategoryService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a new category", func(t *testing.T) {
		c := newFakeCache()
		svc := NewCategoryService(newFakeCategoryStore(), c)

		got, err := svc.Create(ctx, model.CategoryInput{Name: "Nova", Description: ptr("Desc")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Nova" {
			t.Errorf("got name %q", got.Name)
		}
		if !slices.Contains(c.invalidated, "categories:*") {
			t.Error("expected categories:* to be invalidated")
		}
	})

	t.Run("returns CONFLICT when name already exists", func(t *testing.T) {
		svc := NewCategoryService(newFakeCategoryStore(model.Category{ID: 1, Name: "Existente"}), newFakeCache())

		_, err := svc.Create(ctx, model.CategoryInput{Name: "existente"})
		if errCode(err) != "CONFLICT" {
			t.Errorf("got %v, want CONFLICT", err)
		}
	})
}

func TestCategoryService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("updates an existing category", func(t *testing.T) {
		c := newFakeCache()
		svc := NewCategoryService(newFakeCategoryStore(model.Category{ID: 1, Name: "Old"}), c)

		got, err := svc.Update(ctx, 1, model.CategoryUpdate{Name: ptr("Updated")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Updated" {
			t.Errorf("got name %q", got.Name)
		}
		if !slices.Contains(c.deleted, "category:1") {
			t.Error("expected category:1 to be removed from cache")
		}
	})

	t.Run("returns CONFLICT when updating to an existing name", func(t *testing.T) {
		svc := NewCategoryService(newFakeCategoryStore(
			model.Category{ID: 1, Name: "Old"}, model.Category{ID: 2, Name: "Taken"}), newFakeCache())

		_, err := svc.Update(ctx, 1, model.CategoryUpdate{Name: ptr("Taken")})
		if errCode(err) != "CONFLICT" {
			t.Errorf("got %v, want CONFLICT", err)
		}
	})

	t.Run("allows keeping the same name", func(t *testing.T) {
		svc := NewCategoryService(newFakeCategoryStore(model.Category{ID: 1, Name: "Same"}), newFakeCache())

		if _, err := svc.Update(ctx, 1, model.CategoryUpdate{Name: ptr("Same")}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCategoryService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes a category with no products", func(t *testing.T) {
		store := newFakeCategoryStore(model.Category{ID: 1, Name: "Empty"})
		svc := NewCategoryService(store, newFakeCache())

		if err := svc.Delete(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(store.deleted, 1) {
			t.Error("expected category to be deleted")
		}
	})

	t.Run("returns BUSINESS_RULE_VIOLATION when category has active products", func(t *testing.T) {
		store := newFakeCategoryStore(model.Category{ID: 1, Name: "Full"})
		store.productCounts[1] = 5
		svc := NewCategoryService(store, newFakeCache())

		err := svc.Delete(ctx, 1)
		if errCode(err) != "BUSINESS_RULE_VIOLATION" {
			t.Errorf("got %v, want BUSINESS_RULE_VIOLATION", err)
		}
		if len(store.deleted) != 0 {
			t.Error("category must not be deleted")
		}
	})

	t.Run("returns NOT_FOUND when category does not exist", func(t *testing.T) {
		svc := NewCategoryService(newFakeCategoryStore(), newFakeCache())

		if err := svc.Delete(ctx, 999); errCode(err) != "NOT_FOUND" {
			t.Errorf("got %v, want NOT_FOUND", err)
		}
	})
}
