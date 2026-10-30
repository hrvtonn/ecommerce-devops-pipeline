// Package validator validates and normalizes request payloads. Error messages
// follow the format of the original Joi schemas so API clients see the same
// responses.
package validator

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/apperror"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// messages overrides default messages, keyed by error kind (e.g. "string.min").
type messages map[string]string

type stringRule struct {
	required   bool
	allowEmpty bool
	allowNull  bool
	min        int
	max        int
	length     int
	email      bool
	lowercase  bool
	uppercase  bool
	oneOf      []string
	msgs       messages
}

type numberRule struct {
	required  bool
	integer   bool
	positive  bool
	min       *float64
	max       *float64
	precision int // decimal places the value is rounded to; 0 disables rounding
	msgs      messages
}

func limit(v float64) *float64 { return &v }

// object reads the fields of a JSON object (or query string). Unknown keys are
// ignored and every error is accumulated in errs.
type object struct {
	values  map[string]any
	path    string // prefix of the error field, e.g. "items.0."
	label   string // prefix of the error label, e.g. "items[0]."
	errs    *[]apperror.FieldError
	present int
}

func newObject(body any, errs *[]apperror.FieldError) (*object, bool) {
	values, ok := body.(map[string]any)
	if !ok {
		*errs = append(*errs, apperror.FieldError{Field: "", Message: `"value" must be of type object`})
		return nil, false
	}
	return &object{values: values, errs: errs}, true
}

func queryObject(q url.Values, errs *[]apperror.FieldError) *object {
	values := make(map[string]any, len(q))
	for key := range q {
		values[key] = q.Get(key)
	}
	return &object{values: values, errs: errs}
}

func (o *object) lookup(key string) (any, bool) {
	v, ok := o.values[key]
	if ok {
		o.present++
	}
	return v, ok
}

func (o *object) quote(key string) string { return `"` + o.label + key + `"` }

func (o *object) fail(key, kind string, msgs messages, format string, args ...any) {
	msg, ok := msgs[kind]
	if !ok {
		msg = o.quote(key) + " " + fmt.Sprintf(format, args...)
	}
	*o.errs = append(*o.errs, apperror.FieldError{Field: o.path + key, Message: msg})
}

// requireAny reports an error when none of the known fields was sent.
func (o *object) requireAny() {
	if o.present == 0 {
		*o.errs = append(*o.errs, apperror.FieldError{
			Field:   "",
			Message: "At least one field must be provided for update",
		})
	}
}

// str validates a trimmed string. ok is false when the field is absent or
// invalid; a nil value with ok == true means an explicit null.
func (o *object) str(key string, r stringRule) (value *string, ok bool) {
	raw, found := o.lookup(key)
	if !found {
		if r.required {
			o.fail(key, "any.required", r.msgs, "is required")
		}
		return nil, false
	}
	if raw == nil && r.allowNull {
		return nil, true
	}

	s, isString := raw.(string)
	if !isString {
		o.fail(key, "string.base", r.msgs, "must be a string")
		return nil, false
	}

	s = strings.TrimSpace(s)
	if r.lowercase {
		s = strings.ToLower(s)
	}
	if r.uppercase {
		s = strings.ToUpper(s)
	}

	if s == "" {
		if r.allowEmpty {
			return &s, true
		}
		o.fail(key, "string.empty", r.msgs, "is not allowed to be empty")
		return nil, false
	}

	n := utf8.RuneCountInString(s)
	switch {
	case r.min > 0 && n < r.min:
		o.fail(key, "string.min", r.msgs, "length must be at least %d characters long", r.min)
	case r.max > 0 && n > r.max:
		o.fail(key, "string.max", r.msgs, "length must be less than or equal to %d characters long", r.max)
	case r.length > 0 && n != r.length:
		o.fail(key, "string.length", r.msgs, "length must be %d characters long", r.length)
	case r.email && !emailPattern.MatchString(s):
		o.fail(key, "string.email", r.msgs, "must be a valid email")
	case len(r.oneOf) > 0 && !slices.Contains(r.oneOf, s):
		o.fail(key, "any.only", r.msgs, "must be one of [%s]", strings.Join(r.oneOf, ", "))
	default:
		return &s, true
	}
	return nil, false
}

// num validates a number, accepting numeric strings as well.
func (o *object) num(key string, r numberRule) (float64, bool) {
	raw, found := o.lookup(key)
	if !found {
		if r.required {
			o.fail(key, "any.required", r.msgs, "is required")
		}
		return 0, false
	}

	f, isNumber := toNumber(raw)
	switch {
	case !isNumber:
		o.fail(key, "number.base", r.msgs, "must be a number")
	case r.integer && f != math.Trunc(f):
		o.fail(key, "number.integer", r.msgs, "must be an integer")
	case r.positive && f <= 0:
		o.fail(key, "number.positive", r.msgs, "must be a positive number")
	case r.min != nil && f < *r.min:
		o.fail(key, "number.min", r.msgs, "must be greater than or equal to %s", formatNumber(*r.min))
	case r.max != nil && f > *r.max:
		o.fail(key, "number.max", r.msgs, "must be less than or equal to %s", formatNumber(*r.max))
	default:
		if r.precision > 0 {
			scale := math.Pow10(r.precision)
			f = math.Round(f*scale) / scale
		}
		return f, true
	}
	return 0, false
}

func (o *object) integer(key string, r numberRule) (int, bool) {
	r.integer = true
	if r.max == nil {
		r.max = limit(math.MaxInt32)
	}
	f, ok := o.num(key, r)
	return int(f), ok
}

// nested validates a nested object. A nil result with ok == true means an
// explicit null.
func (o *object) nested(key string, allowNull bool) (*object, bool) {
	raw, found := o.lookup(key)
	if !found {
		return nil, false
	}
	if raw == nil && allowNull {
		return nil, true
	}

	values, isObject := raw.(map[string]any)
	if !isObject {
		o.fail(key, "object.base", nil, "must be of type object")
		return nil, false
	}
	return &object{values: values, path: o.path + key + ".", label: o.label + key + ".", errs: o.errs}, true
}

// array validates a required, non-empty array of objects.
func (o *object) array(key string, msgs messages) ([]*object, bool) {
	raw, found := o.lookup(key)
	if !found {
		o.fail(key, "any.required", msgs, "is required")
		return nil, false
	}

	list, isArray := raw.([]any)
	switch {
	case !isArray:
		o.fail(key, "array.base", msgs, "must be an array")
		return nil, false
	case len(list) < 1:
		o.fail(key, "array.min", msgs, "must contain at least 1 items")
		return nil, false
	}

	items := make([]*object, 0, len(list))
	valid := true
	for i, el := range list {
		values, isObject := el.(map[string]any)
		if !isObject {
			*o.errs = append(*o.errs, apperror.FieldError{
				Field:   fmt.Sprintf("%s%s.%d", o.path, key, i),
				Message: fmt.Sprintf(`"%s%s[%d]" must be of type object`, o.label, key, i),
			})
			valid = false
			continue
		}
		items = append(items, &object{
			values: values,
			path:   fmt.Sprintf("%s%s.%d.", o.path, key, i),
			label:  fmt.Sprintf("%s%s[%d].", o.label, key, i),
			errs:   o.errs,
		})
	}
	return items, valid
}

func toNumber(raw any) (float64, bool) {
	var (
		f   float64
		err error
	)
	switch v := raw.(type) {
	case json.Number:
		f, err = v.Float64()
	case float64:
		f = v
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		f, err = strconv.ParseFloat(s, 64)
	default:
		return 0, false
	}
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}

func formatNumber(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func result(errs []apperror.FieldError) error {
	if len(errs) == 0 {
		return nil
	}
	return &apperror.ValidationError{Details: errs}
}

func ptr[T any](v T) *T { return &v }
