package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/cache"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

type ProductService struct {
	store      ProductStore
	categories CategoryStore
	cache      Cache
}

func NewProductService(store ProductStore, categories CategoryStore, cache Cache) *ProductService {
	return &ProductService{store: store, categories: categories, cache: cache}
}

func (s *ProductService) FindAll(ctx context.Context, f model.ProductFilter) (*model.Page[model.Product], error) {
	filterJSON, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	hash := md5.Sum(filterJSON)
	cacheKey := "products:list:" + hex.EncodeToString(hash[:])

	var cached model.Page[model.Product]
	if s.cache.Get(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	products, total, err := s.store.FindAll(ctx, f)
	if err != nil {
		return nil, err
	}
	result := &model.Page[model.Product]{
		Data:       products,
		Pagination: model.NewPagination(f.Page, f.Limit, total),
	}

	s.cache.Set(ctx, cacheKey, result, cache.TTL("products_list"))
	return result, nil
}

func (s *ProductService) FindByID(ctx context.Context, id int) (*model.Product, error) {
	cacheKey := fmt.Sprintf("product:%d", id)
	var cached model.Product
	if s.cache.Get(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	product, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, apperror.NotFound("Product", id)
	}

	s.cache.Set(ctx, cacheKey, product, cache.TTL("product"))
	return product, nil
}

func (s *ProductService) Create(ctx context.Context, in model.ProductInput) (*model.Product, error) {
	if err := s.ensureCategory(ctx, in.CategoryID); err != nil {
		return nil, err
	}

	product, err := s.store.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	s.cache.InvalidatePattern(ctx, "products:*")
	return &product, nil
}

func (s *ProductService) Update(ctx context.Context, id int, up model.ProductUpdate) (*model.Product, error) {
	if _, err := s.FindByID(ctx, id); err != nil {
		return nil, err
	}

	if up.CategoryID != nil {
		if err := s.ensureCategory(ctx, *up.CategoryID); err != nil {
			return nil, err
		}
	}

	updated, err := s.store.Update(ctx, id, up)
	if err != nil {
		return nil, err
	}
	s.cache.Del(ctx, fmt.Sprintf("product:%d", id))
	s.cache.InvalidatePattern(ctx, "products:list:*")
	if updated == nil {
		return nil, apperror.NotFound("Product", id)
	}
	return updated, nil
}

func (s *ProductService) Delete(ctx context.Context, id int) error {
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	if err := s.store.SoftDelete(ctx, id); err != nil {
		return err
	}
	s.cache.Del(ctx, fmt.Sprintf("product:%d", id))
	s.cache.InvalidatePattern(ctx, "products:list:*")
	return nil
}

func (s *ProductService) ensureCategory(ctx context.Context, categoryID int) error {
	category, err := s.categories.FindByID(ctx, categoryID)
	if err != nil {
		return err
	}
	if category == nil {
		return apperror.NotFound("Category", categoryID)
	}
	return nil
}
