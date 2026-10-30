package validator

import (
	"net/url"
	"strings"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/model"
)

var orderStatuses = []string{
	model.StatusPending, model.StatusConfirmed, model.StatusShipped, model.StatusDelivered, model.StatusCancelled,
}

// pagination reads the page/limit query parameters shared by list endpoints.
func pagination(o *object) (page, size int) {
	page, size = 1, 20
	if v, ok := o.integer("page", numberRule{min: limit(1)}); ok {
		page = v
	}
	if v, ok := o.integer("limit", numberRule{min: limit(1), max: limit(100)}); ok {
		size = v
	}
	return page, size
}

// ─── Category ───────────────────────────────────────────────

func CreateCategory(body any) (model.CategoryInput, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return model.CategoryInput{}, result(errs)
	}

	var in model.CategoryInput
	if v, ok := o.str("name", stringRule{required: true, min: 2, max: 100,
		msgs: messages{"string.min": "Category name must have at least 2 characters"}}); ok {
		in.Name = *v
	}
	in.Description = ptr("")
	if v, ok := o.str("description", stringRule{max: 500, allowEmpty: true, allowNull: true}); ok {
		in.Description = v
	}
	return in, result(errs)
}

func UpdateCategory(body any) (model.CategoryUpdate, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return model.CategoryUpdate{}, result(errs)
	}

	var up model.CategoryUpdate
	if v, ok := o.str("name", stringRule{min: 2, max: 100}); ok {
		up.Name = v
	}
	if v, ok := o.str("description", stringRule{max: 500, allowEmpty: true, allowNull: true}); ok {
		up.Description = model.Some(v)
	}
	o.requireAny()
	return up, result(errs)
}

// ─── Product ────────────────────────────────────────────────

var (
	priceRule = numberRule{positive: true, precision: 2,
		msgs: messages{"number.positive": "Price must be greater than zero"}}
	stockRule = numberRule{min: limit(0),
		msgs: messages{"number.min": "Stock cannot be negative"}}
)

func CreateProduct(body any) (model.ProductInput, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return model.ProductInput{}, result(errs)
	}

	var in model.ProductInput
	if v, ok := o.str("name", stringRule{required: true, min: 2, max: 255,
		msgs: messages{"string.min": "Product name must have at least 2 characters"}}); ok {
		in.Name = *v
	}
	in.Description = ptr("")
	if v, ok := o.str("description", stringRule{max: 2000, allowEmpty: true, allowNull: true}); ok {
		in.Description = v
	}
	price := priceRule
	price.required = true
	in.Price, _ = o.num("price", price)
	stock := stockRule
	stock.required = true
	in.Stock, _ = o.integer("stock", stock)
	in.CategoryID, _ = o.integer("category_id", numberRule{required: true, positive: true,
		msgs: messages{"any.required": "Category is required"}})
	if v, ok := o.str("sku", stringRule{max: 100, allowEmpty: true, allowNull: true}); ok {
		in.SKU = v
	}
	return in, result(errs)
}

func UpdateProduct(body any) (model.ProductUpdate, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return model.ProductUpdate{}, result(errs)
	}

	var up model.ProductUpdate
	if v, ok := o.str("name", stringRule{min: 2, max: 255}); ok {
		up.Name = v
	}
	if v, ok := o.str("description", stringRule{max: 2000, allowEmpty: true, allowNull: true}); ok {
		up.Description = model.Some(v)
	}
	if v, ok := o.num("price", priceRule); ok {
		up.Price = &v
	}
	if v, ok := o.integer("stock", stockRule); ok {
		up.Stock = &v
	}
	if v, ok := o.integer("category_id", numberRule{positive: true}); ok {
		up.CategoryID = &v
	}
	if v, ok := o.str("sku", stringRule{max: 100, allowEmpty: true, allowNull: true}); ok {
		up.SKU = model.Some(v)
	}
	o.requireAny()
	return up, result(errs)
}

func ProductQuery(q url.Values) (model.ProductFilter, error) {
	var errs []apperror.FieldError
	o := queryObject(q, &errs)

	f := model.ProductFilter{SortBy: "created_at", SortOrder: "desc"}
	f.Page, f.Limit = pagination(o)
	if v, ok := o.str("search", stringRule{max: 255, allowEmpty: true}); ok {
		f.Search = *v
	}
	f.CategoryID, _ = o.integer("category_id", numberRule{positive: true})
	if v, ok := o.num("min_price", numberRule{min: limit(0)}); ok {
		f.MinPrice = &v
	}
	if v, ok := o.num("max_price", numberRule{min: limit(0)}); ok {
		f.MaxPrice = &v
	}
	if v, ok := o.str("sort_by", stringRule{oneOf: []string{"name", "price", "created_at"}}); ok {
		f.SortBy = *v
	}
	if v, ok := o.str("sort_order", stringRule{oneOf: []string{"asc", "desc"}}); ok {
		f.SortOrder = *v
	}
	return f, result(errs)
}

// ─── Customer ───────────────────────────────────────────────

// IsValidCPF validates a Brazilian CPF number (length and check digits).
func IsValidCPF(cpf string) bool {
	cleaned := digitsOnly(cpf)
	if len(cleaned) != 11 {
		return false
	}

	// Reject known invalid sequences (all same digit)
	if strings.Count(cleaned, cleaned[:1]) == 11 {
		return false
	}

	// Validate check digits
	for _, n := range []int{9, 10} {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(cleaned[i]-'0') * (n + 1 - i)
		}
		remainder := (sum * 10) % 11
		if remainder == 10 {
			remainder = 0
		}
		if remainder != int(cleaned[n]-'0') {
			return false
		}
	}
	return true
}

func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func address(o *object) (*model.Address, bool) {
	sub, ok := o.nested("address", true)
	if !ok || sub == nil {
		return nil, ok
	}

	var a model.Address
	fields := []struct {
		key  string
		dest *string
		rule stringRule
	}{
		{"street", &a.Street, stringRule{max: 255}},
		{"number", &a.Number, stringRule{max: 20}},
		{"neighborhood", &a.Neighborhood, stringRule{max: 100}},
		{"city", &a.City, stringRule{max: 100}},
		{"state", &a.State, stringRule{length: 2, uppercase: true}},
		{"zip_code", &a.ZipCode, stringRule{max: 10}},
	}
	for _, f := range fields {
		if v, ok := sub.str(f.key, f.rule); ok {
			*f.dest = *v
		}
	}
	if v, ok := sub.str("complement", stringRule{max: 100, allowEmpty: true, allowNull: true}); ok {
		a.Complement = v
	}
	return &a, true
}

func CreateCustomer(body any) (model.CustomerInput, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return model.CustomerInput{}, result(errs)
	}

	var in model.CustomerInput
	if v, ok := o.str("name", stringRule{required: true, min: 2, max: 255}); ok {
		in.Name = *v
	}
	if v, ok := o.str("email", stringRule{required: true, email: true, lowercase: true}); ok {
		in.Email = *v
	}
	if v, ok := o.str("cpf", stringRule{required: true}); ok {
		in.CPF = digitsOnly(*v)
		if !IsValidCPF(in.CPF) {
			o.fail("cpf", "any.invalid", messages{"any.invalid": "Invalid CPF number"}, "")
		}
	}
	if v, ok := o.str("phone", stringRule{max: 20, allowEmpty: true, allowNull: true}); ok {
		in.Phone = v
	}
	in.Address, _ = address(o)
	return in, result(errs)
}

func UpdateCustomer(body any) (model.CustomerUpdate, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return model.CustomerUpdate{}, result(errs)
	}

	var up model.CustomerUpdate
	if v, ok := o.str("name", stringRule{min: 2, max: 255}); ok {
		up.Name = v
	}
	if v, ok := o.str("email", stringRule{email: true, lowercase: true}); ok {
		up.Email = v
	}
	if v, ok := o.str("phone", stringRule{max: 20, allowEmpty: true, allowNull: true}); ok {
		up.Phone = model.Some(v)
	}
	if v, ok := address(o); ok {
		up.Address = model.Some(v)
	}
	o.requireAny()
	return up, result(errs)
}

func CustomerQuery(q url.Values) (model.CustomerFilter, error) {
	var errs []apperror.FieldError
	o := queryObject(q, &errs)

	var f model.CustomerFilter
	f.Page, f.Limit = pagination(o)
	if v, ok := o.str("search", stringRule{max: 255, allowEmpty: true}); ok {
		f.Search = *v
	}
	return f, result(errs)
}

// ─── Order ──────────────────────────────────────────────────

func CreateOrder(body any) (model.OrderInput, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return model.OrderInput{}, result(errs)
	}

	var in model.OrderInput
	in.CustomerID, _ = o.integer("customer_id", numberRule{required: true, positive: true,
		msgs: messages{"any.required": "Customer is required"}})

	items, _ := o.array("items", messages{"array.min": "Order must have at least one item"})
	for _, item := range items {
		var it model.OrderItemInput
		it.ProductID, _ = item.integer("product_id", numberRule{required: true, positive: true})
		it.Quantity, _ = item.integer("quantity", numberRule{required: true, min: limit(1),
			msgs: messages{"number.min": "Quantity must be at least 1"}})
		in.Items = append(in.Items, it)
	}

	if v, ok := o.str("notes", stringRule{max: 500, allowEmpty: true, allowNull: true}); ok {
		in.Notes = v
	}
	return in, result(errs)
}

func UpdateOrderStatus(body any) (string, error) {
	var errs []apperror.FieldError
	o, ok := newObject(body, &errs)
	if !ok {
		return "", result(errs)
	}

	var status string
	if v, ok := o.str("status", stringRule{required: true,
		oneOf: orderStatuses[1:],
		msgs:  messages{"any.only": "Status must be one of: CONFIRMED, SHIPPED, DELIVERED, CANCELLED"}}); ok {
		status = *v
	}
	return status, result(errs)
}

func OrderQuery(q url.Values) (model.OrderFilter, error) {
	var errs []apperror.FieldError
	o := queryObject(q, &errs)

	var f model.OrderFilter
	f.Page, f.Limit = pagination(o)
	if v, ok := o.str("status", stringRule{oneOf: orderStatuses}); ok {
		f.Status = *v
	}
	f.CustomerID, _ = o.integer("customer_id", numberRule{positive: true})
	return f, result(errs)
}
