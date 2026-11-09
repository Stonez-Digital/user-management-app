package models

import (
    "encoding/json"
    "testing"

    "github.com/google/uuid"
)

func TestUserJSONExposesLowercaseUUIDForFrontendSelectors(t *testing.T) {
    id := uuid.New()
    user := User{ID: id, Name: "Onoja Monday Ojonugba", Email: "teacher@example.com", Role: "teacher", Active: true}
    raw, err := json.Marshal(user)
    if err != nil { t.Fatal(err) }

    var payload map[string]json.RawMessage
    if err := json.Unmarshal(raw, &payload); err != nil { t.Fatal(err) }

    var got string
    if err := json.Unmarshal(payload["id"], &got); err != nil {
        t.Fatalf("user JSON must expose lowercase id for frontend select values: %s", raw)
    }
    if got != id.String() { t.Fatalf("expected id %q, got %q", id.String(), got) }
    if _, exists := payload["ID"]; exists {
        t.Fatalf("user JSON must not expose Go's default uppercase ID key: %s", raw)
    }
    if _, exists := payload["password_hash"]; exists {
        t.Fatalf("user JSON must not expose password hash: %s", raw)
    }
}
