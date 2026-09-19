package httpx

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
}

func ValidationErrors(err error) []FieldError {
	var ve validator.ValidationErrors
	if !strings.Contains(err.Error(), "validation") && !strings.Contains(err.Error(), "required") {
		return []FieldError{{Field: "body", Rule: "invalid_json"}}
	}
	if errorsAs(err, &ve) {
		out := make([]FieldError, 0, len(ve))
		for _, e := range ve {
			out = append(out, FieldError{Field: e.Field(), Rule: e.Tag()})
		}
		return out
	}
	return []FieldError{{Field: "body", Rule: "invalid"}}
}

func errorsAs(err error, target interface{}) bool {
	switch t := target.(type) {
	case *validator.ValidationErrors:
		ve, ok := err.(validator.ValidationErrors)
		if ok { *t = ve; return true }
	}
	return false
}
