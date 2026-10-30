package httpapi

import (
	"net/http"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/service"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/validator"
)

// ─── Categories ─────────────────────────────────────────────

type categoryHandler struct{ svc *service.CategoryService }

func (h categoryHandler) findAll(w http.ResponseWriter, r *http.Request) error {
	categories, err := h.svc.FindAll(r.Context())
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, categories)
	return nil
}

func (h categoryHandler) findByID(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	category, err := h.svc.FindByID(r.Context(), id)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, category)
	return nil
}

func (h categoryHandler) create(w http.ResponseWriter, r *http.Request) error {
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	in, err := validator.CreateCategory(body)
	if err != nil {
		return err
	}
	category, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	writeData(w, http.StatusCreated, category)
	return nil
}

func (h categoryHandler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	up, err := validator.UpdateCategory(body)
	if err != nil {
		return err
	}
	category, err := h.svc.Update(r.Context(), id, up)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, category)
	return nil
}

func (h categoryHandler) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ─── Products ───────────────────────────────────────────────

type productHandler struct{ svc *service.ProductService }

func (h productHandler) findAll(w http.ResponseWriter, r *http.Request) error {
	filter, err := validator.ProductQuery(r.URL.Query())
	if err != nil {
		return err
	}
	page, err := h.svc.FindAll(r.Context(), filter)
	if err != nil {
		return err
	}
	writePage(w, page)
	return nil
}

func (h productHandler) findByID(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	product, err := h.svc.FindByID(r.Context(), id)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, product)
	return nil
}

func (h productHandler) create(w http.ResponseWriter, r *http.Request) error {
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	in, err := validator.CreateProduct(body)
	if err != nil {
		return err
	}
	product, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	writeData(w, http.StatusCreated, product)
	return nil
}

func (h productHandler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	up, err := validator.UpdateProduct(body)
	if err != nil {
		return err
	}
	product, err := h.svc.Update(r.Context(), id, up)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, product)
	return nil
}

func (h productHandler) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ─── Customers ──────────────────────────────────────────────

type customerHandler struct{ svc *service.CustomerService }

func (h customerHandler) findAll(w http.ResponseWriter, r *http.Request) error {
	filter, err := validator.CustomerQuery(r.URL.Query())
	if err != nil {
		return err
	}
	page, err := h.svc.FindAll(r.Context(), filter)
	if err != nil {
		return err
	}
	writePage(w, page)
	return nil
}

func (h customerHandler) findByID(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	customer, err := h.svc.FindByID(r.Context(), id)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, customer)
	return nil
}

func (h customerHandler) create(w http.ResponseWriter, r *http.Request) error {
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	in, err := validator.CreateCustomer(body)
	if err != nil {
		return err
	}
	customer, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	writeData(w, http.StatusCreated, customer)
	return nil
}

func (h customerHandler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	up, err := validator.UpdateCustomer(body)
	if err != nil {
		return err
	}
	customer, err := h.svc.Update(r.Context(), id, up)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, customer)
	return nil
}

func (h customerHandler) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ─── Orders ─────────────────────────────────────────────────

type orderHandler struct{ svc *service.OrderService }

func (h orderHandler) findAll(w http.ResponseWriter, r *http.Request) error {
	filter, err := validator.OrderQuery(r.URL.Query())
	if err != nil {
		return err
	}
	page, err := h.svc.FindAll(r.Context(), filter)
	if err != nil {
		return err
	}
	writePage(w, page)
	return nil
}

func (h orderHandler) findByID(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	order, err := h.svc.FindByID(r.Context(), id)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, order)
	return nil
}

func (h orderHandler) create(w http.ResponseWriter, r *http.Request) error {
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	in, err := validator.CreateOrder(body)
	if err != nil {
		return err
	}
	order, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	writeData(w, http.StatusCreated, order)
	return nil
}

func (h orderHandler) updateStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	body, err := decodeBody(w, r)
	if err != nil {
		return err
	}
	status, err := validator.UpdateOrderStatus(body)
	if err != nil {
		return err
	}
	order, err := h.svc.UpdateStatus(r.Context(), id, status)
	if err != nil {
		return err
	}
	writeData(w, http.StatusOK, order)
	return nil
}
