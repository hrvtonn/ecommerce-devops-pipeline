package service

import (
	"context"
	"fmt"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/cache"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

type CustomerService struct {
	store CustomerStore
	cache Cache
}

func NewCustomerService(store CustomerStore, cache Cache) *CustomerService {
	return &CustomerService{store: store, cache: cache}
}

func (s *CustomerService) FindAll(ctx context.Context, f model.CustomerFilter) (*model.Page[model.Customer], error) {
	customers, total, err := s.store.FindAll(ctx, f)
	if err != nil {
		return nil, err
	}
	return &model.Page[model.Customer]{
		Data:       customers,
		Pagination: model.NewPagination(f.Page, f.Limit, total),
	}, nil
}

func (s *CustomerService) FindByID(ctx context.Context, id int) (*model.Customer, error) {
	cacheKey := fmt.Sprintf("customer:%d", id)
	var cached model.Customer
	if s.cache.Get(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	customer, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, apperror.NotFound("Customer", id)
	}

	s.cache.Set(ctx, cacheKey, customer, cache.TTL("customer"))
	return customer, nil
}

func (s *CustomerService) Create(ctx context.Context, in model.CustomerInput) (*model.Customer, error) {
	// Check for duplicate email
	existing, err := s.store.FindByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperror.Conflict(fmt.Sprintf("Customer with email %q already exists", in.Email))
	}

	// Check for duplicate CPF
	existing, err = s.store.FindByCPF(ctx, in.CPF)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperror.Conflict("Customer with this CPF already exists")
	}

	customer, err := s.store.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return &customer, nil
}

func (s *CustomerService) Update(ctx context.Context, id int, up model.CustomerUpdate) (*model.Customer, error) {
	customer, err := s.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if up.Email != nil && *up.Email != customer.Email {
		existing, err := s.store.FindByEmail(ctx, *up.Email)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, apperror.Conflict(fmt.Sprintf("Customer with email %q already exists", *up.Email))
		}
	}

	updated, err := s.store.Update(ctx, id, up)
	if err != nil {
		return nil, err
	}
	s.cache.Del(ctx, fmt.Sprintf("customer:%d", id))
	if updated == nil {
		return nil, apperror.NotFound("Customer", id)
	}
	return updated, nil
}

func (s *CustomerService) Delete(ctx context.Context, id int) error {
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	activeOrders, err := s.store.CountActiveOrders(ctx, id)
	if err != nil {
		return err
	}
	if activeOrders > 0 {
		return apperror.Business(fmt.Sprintf(
			"Cannot delete customer: there are %d active order(s) associated", activeOrders))
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.cache.Del(ctx, fmt.Sprintf("customer:%d", id))
	return nil
}
