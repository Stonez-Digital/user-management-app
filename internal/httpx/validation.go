package httpx

import (
	"encoding/json"
	"errors"
	"io"

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
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" { field = "body" }
		return []FieldError{{Field: field, Rule: "invalid_type"}}
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return []FieldError{{Field: "body", Rule: "malformed_json"}}
	}
	if errors.Is(err, io.EOF) {
		return []FieldError{{Field: "body", Rule: "required"}}
	}
	return []FieldError{{Field: "body", Rule: "invalid"}}
}

