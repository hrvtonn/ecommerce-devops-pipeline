package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

// ─── Cache ──────────────────────────────────────────────────

type fakeCache struct {
	data        map[string][]byte
	deleted     []string
	invalidated []string
}

func newFakeCache() *fakeCache { return &fakeCache{data: map[string][]byte{}} }

func (c *fakeCache) Get(_ context.Context, key string, dest any) bool {
	raw, ok := c.data[key]
	return ok && json.Unmarshal(raw, dest) == nil
}

func (c *fakeCache) Set(_ context.Context, key string, value any, _ time.Duration) {
	c.data[key], _ = json.Marshal(value)
}

func (c *fakeCache) Del(_ context.Context, key string) {
	delete(c.data, key)
	c.deleted = append(c.deleted, key)
}

func (c *fakeCache) InvalidatePattern(_ context.Context, pattern string) {
	prefix := strings.TrimSuffix(pattern, "*")
	for key := range c.data {
		if strings.HasPrefix(key, prefix) {
			delete(c.data, key)
		}
	}
	c.invalidated = append(c.invalidated, pattern)
}

// ─── Transactor ─────────────────────────────────────────────

type fakeTx struct{ calls int }

func (t *fakeTx) InTx(_ context.Context, fn func(database.DBTX) error) error {
	t.calls++
	return fn(nil)
}

// ─── Categories ─────────────────────────────────────────────

type fakeCategoryStore struct {
	items         map[int]model.Category
	productCounts map[int]int
	findAllCalls  int
	deleted       []int
}

func newFakeCategoryStore(categories ...model.Category) *fakeCategoryStore {
	s := &fakeCategoryStore{items: map[int]model.Category{}, productCounts: map[int]int{}}
	for _, c := range categories {
		s.items[c.ID] = c
	}
	return s
}

func (s *fakeCategoryStore) FindAll(context.Context) ([]model.Category, error) {
	s.findAllCalls++
	list := []model.Category{}
	for _, c := range s.items {
		list = append(list, c)
	}
	return list, nil
}

func (s *fakeCategoryStore) FindByID(_ context.Context, id int) (*model.Category, error) {
	if c, ok := s.items[id]; ok {
		return &c, nil
	}
	return nil, nil
}

func (s *fakeCategoryStore) FindByName(_ context.Context, name string) (*model.Category, error) {
	for _, c := range s.items {
		if strings.EqualFold(c.Name, name) {
			return &c, nil
		}
	}
	return nil, nil
}

func (s *fakeCategoryStore) Create(_ context.Context, in model.CategoryInput) (model.Category, error) {
	c := model.Category{ID: len(s.items) + 1, Name: in.Name, Description: in.Description}
	s.items[c.ID] = c
	return c, nil
}

func (s *fakeCategoryStore) Update(_ context.Context, id int, up model.CategoryUpdate) (*model.Category, error) {
	c, ok := s.items[id]
	if !ok {
		return nil, nil
	}
	if up.Name != nil {
		c.Name = *up.Name
	}
	if up.Description.Set {
		c.Description = up.Description.Value
	}
	s.items[id] = c
	return &c, nil
}

func (s *fakeCategoryStore) Delete(_ context.Context, id int) error {
	delete(s.items, id)
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *fakeCategoryStore) CountProducts(_ context.Context, id int) (int, error) {
	return s.productCounts[id], nil
}

// ─── Products ───────────────────────────────────────────────

type fakeProductStore struct {
	items       map[int]model.Product
	lastFilter  model.ProductFilter
	softDeleted []int
	failStock   bool
}

func newFakeProductStore(products ...model.Product) *fakeProductStore {
	s := &fakeProductStore{items: map[int]model.Product{}}
	for _, p := range products {
		s.items[p.ID] = p
	}
	return s
}

func (s *fakeProductStore) FindAll(_ context.Context, f model.ProductFilter) ([]model.Product, int, error) {
	s.lastFilter = f
	list := []model.Product{}
	for _, p := range s.items {
		if f.CategoryID == 0 || p.CategoryID == f.CategoryID {
			list = append(list, p)
		}
	}
	return list, len(list), nil
}

func (s *fakeProductStore) FindByID(_ context.Context, id int) (*model.Product, error) {
	if p, ok := s.items[id]; ok && p.DeletedAt == nil {
		return &p, nil
	}
	return nil, nil
}

func (s *fakeProductStore) Create(_ context.Context, in model.ProductInput) (model.Product, error) {
	p := model.Product{ID: len(s.items) + 1, Name: in.Name, Description: in.Description,
		Price: in.Price, Stock: in.Stock, CategoryID: in.CategoryID, SKU: in.SKU}
	s.items[p.ID] = p
	return p, nil
}

func (s *fakeProductStore) Update(_ context.Context, id int, up model.ProductUpdate) (*model.Product, error) {
	p, ok := s.items[id]
	if !ok {
		return nil, nil
	}
	if up.Name != nil {
		p.Name = *up.Name
	}
	if up.Price != nil {
		p.Price = *up.Price
	}
	if up.Stock != nil {
		p.Stock = *up.Stock
	}
	if up.CategoryID != nil {
		p.CategoryID = *up.CategoryID
	}
	s.items[id] = p
	return &p, nil
}

func (s *fakeProductStore) SoftDelete(_ context.Context, id int) error {
	p := s.items[id]
	now := time.Now()
	p.DeletedAt = &now
	s.items[id] = p
	s.softDeleted = append(s.softDeleted, id)
	return nil
}

func (s *fakeProductStore) UpdateStock(_ context.Context, _ database.DBTX, productID, delta int) (*model.Product, error) {
	p, ok := s.items[productID]
	if !ok || s.failStock || p.Stock+delta < 0 {
		return nil, nil
	}
	p.Stock += delta
	s.items[productID] = p
	return &p, nil
}

// ─── Customers ──────────────────────────────────────────────

type fakeCustomerStore struct {
	items        map[int]model.Customer
	activeOrders map[int]int
	lastFilter   model.CustomerFilter
	created      []model.CustomerInput
	deleted      []int
}

func newFakeCustomerStore(customers ...model.Customer) *fakeCustomerStore {
	s := &fakeCustomerStore{items: map[int]model.Customer{}, activeOrders: map[int]int{}}
	for _, c := range customers {
		s.items[c.ID] = c
	}
	return s
}

func (s *fakeCustomerStore) FindAll(_ context.Context, f model.CustomerFilter) ([]model.Customer, int, error) {
	s.lastFilter = f
	list := []model.Customer{}
	for _, c := range s.items {
		if f.Search == "" || strings.Contains(c.Name, f.Search) || strings.Contains(c.Email, f.Search) {
			list = append(list, c)
		}
	}
	return list, len(list), nil
}

func (s *fakeCustomerStore) FindByID(_ context.Context, id int) (*model.Customer, error) {
	if c, ok := s.items[id]; ok {
		return &c, nil
	}
	return nil, nil
}

func (s *fakeCustomerStore) find(match func(model.Customer) bool) *model.Customer {
	for _, c := range s.items {
		if match(c) {
			return &c
		}
	}
	return nil
}

func (s *fakeCustomerStore) FindByEmail(_ context.Context, email string) (*model.Customer, error) {
	return s.find(func(c model.Customer) bool { return c.Email == email }), nil
}

func (s *fakeCustomerStore) FindByCPF(_ context.Context, cpf string) (*model.Customer, error) {
	return s.find(func(c model.Customer) bool { return c.CPF == cpf }), nil
}

func (s *fakeCustomerStore) Create(_ context.Context, in model.CustomerInput) (model.Customer, error) {
	s.created = append(s.created, in)
	c := model.Customer{ID: len(s.items) + 1, Name: in.Name, Email: in.Email, CPF: in.CPF,
		Phone: in.Phone, Address: in.Address}
	s.items[c.ID] = c
	return c, nil
}

func (s *fakeCustomerStore) Update(_ context.Context, id int, up model.CustomerUpdate) (*model.Customer, error) {
	c, ok := s.items[id]
	if !ok {
		return nil, nil
	}
	if up.Name != nil {
		c.Name = *up.Name
	}
	if up.Email != nil {
		c.Email = *up.Email
	}
	s.items[id] = c
	return &c, nil
}

func (s *fakeCustomerStore) Delete(_ context.Context, id int) error {
	delete(s.items, id)
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *fakeCustomerStore) CountActiveOrders(_ context.Context, id int) (int, error) {
	return s.activeOrders[id], nil
}

// ─── Orders ─────────────────────────────────────────────────

type fakeOrderStore struct {
	orders     map[int]model.Order
	items      map[int][]model.OrderItem
	lastFilter model.OrderFilter
}

func newFakeOrderStore(orders ...model.Order) *fakeOrderStore {
	s := &fakeOrderStore{orders: map[int]model.Order{}, items: map[int][]model.OrderItem{}}
	for _, o := range orders {
		s.items[o.ID] = o.Items
		o.Items = nil
		s.orders[o.ID] = o
	}
	return s
}

func (s *fakeOrderStore) FindAll(_ context.Context, f model.OrderFilter) ([]model.Order, int, error) {
	s.lastFilter = f
	list := []model.Order{}
	for _, o := range s.orders {
		if f.Status == "" || o.Status == f.Status {
			list = append(list, o)
		}
	}
	return list, len(list), nil
}

func (s *fakeOrderStore) FindByID(_ context.Context, id int) (*model.Order, error) {
	if o, ok := s.orders[id]; ok {
		return &o, nil
	}
	return nil, nil
}

func (s *fakeOrderStore) FindItemsByOrderID(_ context.Context, id int) ([]model.OrderItem, error) {
	return s.items[id], nil
}

func (s *fakeOrderStore) Create(_ context.Context, _ database.DBTX, customerID int, total float64, notes *string) (model.Order, error) {
	o := model.Order{ID: len(s.orders) + 1, CustomerID: customerID, Status: model.StatusPending, Total: total, Notes: notes}
	s.orders[o.ID] = o
	return o, nil
}

func (s *fakeOrderStore) CreateItem(_ context.Context, _ database.DBTX, item model.OrderItem) (model.OrderItem, error) {
	item.ID = len(s.items[item.OrderID]) + 1
	s.items[item.OrderID] = append(s.items[item.OrderID], item)
	return item, nil
}

func (s *fakeOrderStore) UpdateStatus(_ context.Context, _ database.DBTX, id int, status string) error {
	o := s.orders[id]
	o.Status = status
	s.orders[id] = o
	return nil
}

// ─── Assertions ─────────────────────────────────────────────

// errCode returns the code of an *apperror.Error, or "" for any other error.
func errCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func ptr[T any](v T) *T { return &v }
