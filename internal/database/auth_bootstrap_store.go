package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/authbootstrap"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ConnectAuthBootstrap opens a dedicated pool whose credential should have
// EXECUTE on authenticate_access_session and no direct users/sessions access.
func ConnectAuthBootstrap() (*gorm.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("AUTH_BOOTSTRAP_DATABASE_URL"))
	if dsn == "" {
		return nil, fmt.Errorf("AUTH_BOOTSTRAP_DATABASE_URL is required for PostgreSQL authentication")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open auth bootstrap database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get auth bootstrap connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping auth bootstrap database: %w", err)
	}
	return db, nil
}

type PostgresAuthBootstrapStore struct{ db *gorm.DB }

func NewPostgresAuthBootstrapStore(db *gorm.DB) *PostgresAuthBootstrapStore {
	return &PostgresAuthBootstrapStore{db: db}
}

func (s *PostgresAuthBootstrapStore) AuthenticateAccessSession(ctx context.Context, sessionID, userID uuid.UUID) (authbootstrap.Identity, error) {
	if s == nil || s.db == nil || s.db.Dialector.Name() != "postgres" {
		return authbootstrap.Identity{}, fmt.Errorf("PostgreSQL auth bootstrap store is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var result authbootstrap.Identity
	var schoolID sql.NullString
	row := s.db.WithContext(ctx).Raw(
		"SELECT user_id, user_role, school_id FROM public.authenticate_access_session(?, ?)",
		sessionID, userID,
	).Row()
	if err := row.Scan(&result.UserID, &result.Role, &schoolID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authbootstrap.Identity{}, authbootstrap.ErrUnauthenticated
		}
		return authbootstrap.Identity{}, fmt.Errorf("execute auth bootstrap capability: %w", err)
	}
	if schoolID.Valid && schoolID.String != "" {
		parsed, err := uuid.Parse(schoolID.String)
		if err != nil {
			return authbootstrap.Identity{}, fmt.Errorf("auth bootstrap returned invalid school identifier: %w", err)
		}
		result.SchoolID = &parsed
	}
	if result.UserID != userID || strings.TrimSpace(result.Role) == "" {
		return authbootstrap.Identity{}, authbootstrap.ErrUnauthenticated
	}
	return result, nil
}

// LegacyAuthBootstrapStore is restricted to local SQLite development. It is
// deliberately rejected for PostgreSQL so production cannot silently fall back
// to direct table reads when the narrow database capability is unavailable.
type LegacyAuthBootstrapStore struct{ db *gorm.DB }

func NewLegacyAuthBootstrapStore(db *gorm.DB) *LegacyAuthBootstrapStore {
	return &LegacyAuthBootstrapStore{db: db}
}

func (s *LegacyAuthBootstrapStore) AuthenticateAccessSession(ctx context.Context, sessionID, userID uuid.UUID) (authbootstrap.Identity, error) {
	if s == nil || s.db == nil || s.db.Dialector.Name() == "postgres" {
		return authbootstrap.Identity{}, fmt.Errorf("legacy auth bootstrap is available only for non-PostgreSQL development")
	}
	var session models.Session
	err := s.db.WithContext(ctx).
		Select("id", "user_id", "expires_at", "revoked").
		Where("id = ? AND user_id = ? AND revoked = ? AND expires_at > ?", sessionID, userID, false, time.Now()).
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return authbootstrap.Identity{}, authbootstrap.ErrUnauthenticated
	}
	if err != nil {
		return authbootstrap.Identity{}, fmt.Errorf("read development auth session: %w", err)
	}

	var user struct {
		ID       uuid.UUID
		Role     string
		SchoolID *uuid.UUID
		Active   bool
	}
	err = s.db.WithContext(ctx).Table("users").
		Select("id", "role", "school_id", "active").
		Where("id = ?", userID).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !user.Active) {
		return authbootstrap.Identity{}, authbootstrap.ErrUnauthenticated
	}
	if err != nil {
		return authbootstrap.Identity{}, fmt.Errorf("read development auth identity: %w", err)
	}
	return authbootstrap.Identity{UserID: user.ID, Role: user.Role, SchoolID: user.SchoolID}, nil
}
