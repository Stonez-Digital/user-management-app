package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestStudentJSONIncludesOnlySafeLinkedUserIdentity(t *testing.T) {
	schoolID := uuid.New()
	userID := uuid.New()
	student := Student{
		ID: uuid.New(), SchoolID: schoolID, UserID: userID, AdmissionNumber: "KG001",
		User: User{ID: userID, SchoolID: &schoolID, Name: "Jane Doe", Email: "jane@example.com", Role: "student", Active: true, PasswordHash: "must-not-leak"},
	}

	raw, err := json.Marshal(student)
	if err != nil { t.Fatal(err) }
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil { t.Fatal(err) }

	var user map[string]json.RawMessage
	if err := json.Unmarshal(payload["user"], &user); err != nil { t.Fatalf("expected linked user identity in student JSON: %s", raw) }
	var name, email string
	if err := json.Unmarshal(user["name"], &name); err != nil { t.Fatal(err) }
	if err := json.Unmarshal(user["email"], &email); err != nil { t.Fatal(err) }
	if name != "Jane Doe" || email != "jane@example.com" { t.Fatalf("unexpected user identity: name=%q email=%q", name, email) }
	for _, forbidden := range []string{"password_hash", "role", "active", "school_id"} {
		if _, ok := user[forbidden]; ok { t.Fatalf("student JSON leaked user field %q", forbidden) }
	}
}

func TestStudentJSONOmitsUnloadedUser(t *testing.T) {
	raw, err := json.Marshal(Student{ID: uuid.New(), AdmissionNumber: "KG002"})
	if err != nil { t.Fatal(err) }
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil { t.Fatal(err) }
	if _, ok := payload["user"]; ok { t.Fatalf("expected unloaded user relation to be omitted: %s", raw) }
}
