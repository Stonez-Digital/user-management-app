package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestUserJSONExposesLowercaseIDForFrontendSelectors(t *testing.T) {
	id := uuid.New()
	user := User{ID: id, Name: "Teacher One", Email: "teacher@example.com", Role: "teacher", Active: true}

	raw, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal user JSON: %v", err)
	}
	if payload["id"] != id.String() {
		t.Fatalf("expected lowercase id %q in user JSON, got %#v", id, payload)
	}
	if _, exists := payload["ID"]; exists {
		t.Fatalf("user JSON must not expose the Go field name ID: %s", raw)
	}
	if _, exists := payload["password_hash"]; exists {
		t.Fatalf("user JSON must not expose password hash: %s", raw)
	}
}
