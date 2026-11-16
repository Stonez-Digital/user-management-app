package service

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSubscriptionBillingRLSInstallerWithRestrictedPostgresRole(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" { t.Skip("TEST_DATABASE_URL is not configured") }
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { t.Fatalf("open PostgreSQL admin connection: %v", err) }
	adminSQL, err := admin.DB()
	if err != nil { t.Fatal(err) }

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := "billing_rls_" + suffix
	groupRole := "billing_api_" + suffix
	loginRole := "billing_login_" + suffix
	password := strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE ROLE " + groupRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS").Error; err != nil { t.Fatalf("create restricted group role: %v", err) }
	if err := admin.Exec("CREATE ROLE " + loginRole + " LOGIN PASSWORD '" + password + "' NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS").Error; err != nil {
		_ = admin.Exec("DROP ROLE " + groupRole).Error
		t.Fatalf("create restricted login role: %v", err)
	}
	if err := admin.Exec("GRANT " + groupRole + " TO " + loginRole).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + schema + " TO " + groupRole).Error; err != nil { t.Fatal(err) }
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		_ = admin.Exec("DROP OWNED BY " + loginRole).Error
		_ = admin.Exec("DROP ROLE IF EXISTS " + loginRole).Error
		_ = admin.Exec("DROP OWNED BY " + groupRole).Error
		_ = admin.Exec("DROP ROLE IF EXISTS " + groupRole).Error
		_ = adminSQL.Close()
	})

	for _, statement := range []string{
		"CREATE TABLE " + schema + ".billing_plans (id uuid PRIMARY KEY, active boolean NOT NULL)",
		"CREATE TABLE " + schema + ".school_subscriptions (id uuid PRIMARY KEY, school_id uuid NOT NULL)",
		"CREATE TABLE " + schema + ".subscription_events (id uuid PRIMARY KEY, school_id uuid NOT NULL, actor_id uuid, event_type text NOT NULL)",
		"CREATE TABLE " + schema + ".subscription_payments (id uuid PRIMARY KEY, school_id uuid NOT NULL)",
	} {
		if err := admin.Exec(statement).Error; err != nil { t.Fatalf("create fixture table: %v", err) }
	}
	schoolA, schoolB, userID := uuid.New(), uuid.New(), uuid.New()
	planActive, planInactive := uuid.New(), uuid.New()
	if err := admin.Exec("INSERT INTO "+schema+".billing_plans (id,active) VALUES (?,?),(?,?)", planActive, true, planInactive, false).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec("INSERT INTO "+schema+".school_subscriptions (id,school_id) VALUES (?,?),(?,?)", uuid.New(), schoolA, uuid.New(), schoolB).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec("INSERT INTO "+schema+".subscription_events (id,school_id,actor_id,event_type) VALUES (?, ?, ?, ?), (?, ?, NULL, ?)", uuid.New(), schoolA, userID, "created", uuid.New(), schoolB, "created").Error; err != nil { t.Fatal(err) }
	if err := admin.Exec("INSERT INTO "+schema+".subscription_payments (id,school_id) VALUES (?,?),(?,?)", uuid.New(), schoolA, uuid.New(), schoolB).Error; err != nil { t.Fatal(err) }

	raw, err := os.ReadFile("../../scripts/security/install_subscription_billing_rls.sql")
	if err != nil { t.Fatalf("read billing RLS installer: %v", err) }
	script := string(raw)
	script = strings.ReplaceAll(script, "stonez_api_runtime", groupRole)
	script = strings.ReplaceAll(script, "public.", schema+".")
	script = strings.ReplaceAll(script, "'public'", "'"+schema+"'")
	pgxConfig, err := pgx.ParseConfig(dsn)
	if err != nil { t.Fatalf("parse PostgreSQL test DSN for SQL installer: %v", err) }
	pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	scriptConn, err := pgx.ConnectConfig(context.Background(), pgxConfig)
	if err != nil { t.Fatalf("open PostgreSQL simple-protocol connection: %v", err) }
	if _, err := scriptConn.Exec(context.Background(), script); err != nil {
		_ = scriptConn.Close(context.Background())
		t.Fatalf("execute billing RLS installer in isolated schema: %v", err)
	}
	if err := scriptConn.Close(context.Background()); err != nil { t.Fatalf("close PostgreSQL SQL installer connection: %v", err) }

	parsed, err := url.Parse(dsn)
	if err != nil { t.Fatal(err) }
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	parsed.User = url.UserPassword(loginRole, password)
	restricted, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil { t.Fatalf("connect as restricted billing role: %v", err) }
	restrictedSQL, err := restricted.DB()
	if err != nil { t.Fatal(err) }
	restrictedSQL.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = restrictedSQL.Close() })

	assertVisible := func(table string, schoolID uuid.UUID, want int64) {
		t.Helper()
		err := tenantTransaction(restricted, schoolID, userID, func(tx *gorm.DB) error {
			var count int64
			if err := tx.Table(schema+"."+table).Count(&count).Error; err != nil { return err }
			if count != want { t.Errorf("%s visible rows=%d, want %d", table, count, want) }
			return nil
		})
		if err != nil { t.Errorf("%s tenant read: %v", table, err) }
	}
	assertVisible("school_subscriptions", schoolA, 1)
	assertVisible("subscription_events", schoolA, 1)
	assertVisible("subscription_payments", schoolA, 1)

	if err := tenantTransaction(restricted, schoolA, userID, func(tx *gorm.DB) error {
		var totalPlans int64
		if err := tx.Table(schema+".billing_plans").Count(&totalPlans).Error; err != nil { return err }
		if totalPlans != 1 { t.Errorf("active plan catalogue rows=%d, want 1", totalPlans) }
		crossSchoolWrites := []struct {
			name string
			write func(*gorm.DB) error
		}{
			{name: "subscription", write: func(sp *gorm.DB) error {
				return sp.Table(schema+".school_subscriptions").Create(map[string]interface{}{"id":uuid.New(),"school_id":schoolB}).Error
			}},
			{name: "payment", write: func(sp *gorm.DB) error {
				return sp.Table(schema+".subscription_payments").Create(map[string]interface{}{"id":uuid.New(),"school_id":schoolB}).Error
			}},
			{name: "event", write: func(sp *gorm.DB) error {
				return sp.Table(schema+".subscription_events").Create(map[string]interface{}{"id":uuid.New(),"school_id":schoolB,"event_type":"forbidden"}).Error
			}},
		}
		for _, attempt := range crossSchoolWrites {
			err := tx.Transaction(func(savepoint *gorm.DB) error { return attempt.write(savepoint) })
			if err == nil { t.Errorf("cross-school %s insert unexpectedly succeeded", attempt.name) }
		}
		if err := tx.Table(schema+".school_subscriptions").Where("school_id = ?", schoolA).Update("school_id", schoolA).Error; err != nil {
			t.Errorf("same-school subscription update failed: %v", err)
		}
		if err := tx.Table(schema+".subscription_payments").Where("school_id = ?", schoolA).Update("school_id", schoolA).Error; err != nil {
			t.Errorf("same-school payment update failed: %v", err)
		}
		if err := tx.Table(schema+".school_subscriptions").Create(map[string]interface{}{"id":uuid.New(),"school_id":schoolA}).Error; err != nil {
			t.Errorf("same-school subscription insert failed: %v", err)
		}
		if err := tx.Table(schema+".subscription_payments").Create(map[string]interface{}{"id":uuid.New(),"school_id":schoolA}).Error; err != nil {
			t.Errorf("same-school payment insert failed: %v", err)
		}
		if err := tx.Table(schema+".subscription_events").Create(map[string]interface{}{"id":uuid.New(),"school_id":schoolA,"actor_id":userID,"event_type":"allowed"}).Error; err != nil {
			t.Errorf("same-school event insert failed: %v", err)
		}
		return nil
	}); err != nil { t.Fatalf("restricted role tenant transaction: %v", err) }

	var withoutContext int64
	if err := restricted.Table(schema+".school_subscriptions").Count(&withoutContext).Error; err != nil { t.Fatal(err) }
	if withoutContext != 0 { t.Fatalf("missing tenant context exposed %d subscription rows", withoutContext) }
}
