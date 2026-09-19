package httpx

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
}

func ValidationErrors(err error) []FieldError {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		out := make([]FieldError, 0, len(ve))
		for _, e := range ve {
			out = append(out, FieldError{Field: e.Field(), Rule: e.Tag()})
		}
		return out
	}
	return []FieldError{{Field: "body", Rule: "invalid"}}
}

