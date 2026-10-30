package service

import (
	"context"
	"fmt"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/cache"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

type CategoryService struct {
	store CategoryStore
	cache Cache
}

func NewCategoryService(store CategoryStore, cache Cache) *CategoryService {
	return &CategoryService{store: store, cache: cache}
}

func (s *CategoryService) FindAll(ctx context.Context) ([]model.Category, error) {
	const cacheKey = "categories:list"
	var cached []model.Category
	if s.cache.Get(ctx, cacheKey, &cached) {
		return cached, nil
	}

	categories, err := s.store.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	s.cache.Set(ctx, cacheKey, categories, cache.TTL("categories_list"))
	return categories, nil
}

func (s *CategoryService) FindByID(ctx context.Context, id int) (*model.Category, error) {
	cacheKey := fmt.Sprintf("category:%d", id)
	var cached model.Category
	if s.cache.Get(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	category, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, apperror.NotFound("Category", id)
	}

	s.cache.Set(ctx, cacheKey, category, cache.TTL("category"))
	return category, nil
}

func (s *CategoryService) Create(ctx context.Context, in model.CategoryInput) (*model.Category, error) {
	existing, err := s.store.FindByName(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperror.Conflict(fmt.Sprintf("Category with name %q already exists", in.Name))
	}

	category, err := s.store.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	s.cache.InvalidatePattern(ctx, "categories:*")
	return &category, nil
}

func (s *CategoryService) Update(ctx context.Context, id int, up model.CategoryUpdate) (*model.Category, error) {
	if _, err := s.FindByID(ctx, id); err != nil {
		return nil, err
	}

	if up.Name != nil {
		existing, err := s.store.FindByName(ctx, *up.Name)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != id {
			return nil, apperror.Conflict(fmt.Sprintf("Category with name %q already exists", *up.Name))
		}
	}

	updated, err := s.store.Update(ctx, id, up)
	if err != nil {
		return nil, err
	}
	s.cache.Del(ctx, fmt.Sprintf("category:%d", id))
	s.cache.InvalidatePattern(ctx, "categories:*")
	if updated == nil {
		return nil, apperror.NotFound("Category", id)
	}
	return updated, nil
}

func (s *CategoryService) Delete(ctx context.Context, id int) error {
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	productCount, err := s.store.CountProducts(ctx, id)
	if err != nil {
		return err
	}
	if productCount > 0 {
		return apperror.Business(fmt.Sprintf(
			"Cannot delete category: it has %d active product(s) associated", productCount))
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.cache.Del(ctx, fmt.Sprintf("category:%d", id))
	s.cache.InvalidatePattern(ctx, "categories:*")
	return nil
}
