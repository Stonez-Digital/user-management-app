package database

import (
    "context"
    "database/sql"
    "errors"
    "embed"
    "fmt"
    "net/url"
    "os"
    "strings"
    "testing"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/authbootstrap"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

//go:embed auth_bootstrap.sql
var authBootstrapSQL embed.FS

func TestAuthBootstrapCapabilityPostgres(t *testing.T) {
    dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
    if dsn == "" { t.Skip("TEST_DATABASE_URL is not set") }
    parsed, err := url.Parse(dsn)
    if err != nil { t.Fatalf("parse TEST_DATABASE_URL: %v", err) }
    host := strings.ToLower(parsed.Hostname())
    if host != "localhost" && host != "127.0.0.1" && host != "::1" && host != "postgres" {
        t.Fatalf("refusing auth-bootstrap test against non-local PostgreSQL host %q", host)
    }

    gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil { t.Fatalf("connect disposable PostgreSQL: %v", err) }
    sqlDB, err := gdb.DB()
    if err != nil { t.Fatal(err) }
    defer sqlDB.Close()
    conn, err := sqlDB.Conn(context.Background())
    if err != nil { t.Fatal(err) }
    defer conn.Close()

    suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
    schema, owner, runtime := "auth_bootstrap_"+suffix, "auth_fn_owner_"+suffix, "auth_runtime_"+suffix
    qs, qo, qr := quoteIdentifier(schema), quoteIdentifier(owner), quoteIdentifier(runtime)
    exec := func(query string, args ...any) {
        t.Helper()
        if _, e := conn.ExecContext(context.Background(), query, args...); e != nil {
            t.Fatalf("SQL failed: %v\n%s", e, query)
        }
    }

    exec("CREATE SCHEMA "+qs)
    defer func() {
        _, _ = conn.ExecContext(context.Background(), "RESET ROLE")
        _, _ = conn.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+qs+" CASCADE")
        _, _ = conn.ExecContext(context.Background(), "DROP ROLE IF EXISTS "+qr)
        _, _ = conn.ExecContext(context.Background(), "DROP ROLE IF EXISTS "+qo)
    }()
    exec("CREATE ROLE "+qo+" NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION")
    exec("CREATE ROLE "+qr+" NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION")
    exec(fmt.Sprintf("CREATE TABLE %s.users (id uuid PRIMARY KEY, active boolean NOT NULL, role text NOT NULL, school_id uuid, password_hash text NOT NULL)", qs))
    exec(fmt.Sprintf("CREATE TABLE %s.sessions (id uuid PRIMARY KEY, user_id uuid NOT NULL, expires_at timestamptz NOT NULL, revoked boolean NOT NULL DEFAULT false, refresh_token_hash text)", qs))

    raw, err := authBootstrapSQL.ReadFile("auth_bootstrap.sql")
    if err != nil { t.Fatal(err) }
    script := string(raw)
    script = strings.ReplaceAll(script, "stonez_auth_function_owner", owner)
    script = strings.ReplaceAll(script, "stonez_auth_runtime", runtime)
    script = strings.ReplaceAll(script, "public", schema)
    // Split only on SQL statement terminators outside dollar-quoted bodies.
    for _, statement := range splitSQLStatements(script) {
        statement = strings.TrimSpace(statement)
        if statement == "" || strings.HasPrefix(statement, "--") { continue }
        exec(statement)
    }

    userID, sessionID, schoolID := uuid.New(), uuid.New(), uuid.New()
    exec(fmt.Sprintf("INSERT INTO %s.users(id,active,role,school_id,password_hash) VALUES ($1,true,'school_admin',$2,'must-not-be-readable')", qs), userID, schoolID)
    exec(fmt.Sprintf("INSERT INTO %s.sessions(id,user_id,expires_at,revoked,refresh_token_hash) VALUES ($1,$2,now()+interval '1 hour',false,'refresh-secret')", qs), sessionID, userID)

    exec("SET ROLE "+qr)
    var gotUser, gotSchool uuid.UUID
    var gotRole string
    query := fmt.Sprintf("SELECT user_id,user_role,school_id FROM %s.authenticate_access_session($1,$2)", qs)
    countQuery := fmt.Sprintf("SELECT count(*) FROM %s.authenticate_access_session($1,$2)", qs)
    if err := conn.QueryRowContext(context.Background(), query, sessionID, userID).Scan(&gotUser,&gotRole,&gotSchool); err != nil { t.Fatalf("valid session rejected: %v", err) }
    if gotUser != userID || gotSchool != schoolID || gotRole != "school_admin" { t.Fatalf("unexpected bootstrap result: %s %s %s", gotUser,gotRole,gotSchool) }

    // Exercise the real Go adapter against the same disposable SQL capability,
    // using a single pooled connection with the restricted runtime role active.
    adapterDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil { t.Fatalf("open adapter PostgreSQL pool: %v", err) }
    adapterSQLDB, err := adapterDB.DB()
    if err != nil { t.Fatal(err) }
    adapterSQLDB.SetMaxOpenConns(1)
    adapterSQLDB.SetMaxIdleConns(1)
    defer adapterSQLDB.Close()
    if err := adapterDB.Exec("SET ROLE "+qr).Error; err != nil { t.Fatalf("set adapter runtime role: %v", err) }
    adapter := newPostgresAuthBootstrapStoreForSchema(adapterDB, schema)
    identity, err := adapter.AuthenticateAccessSession(context.Background(), sessionID, userID)
    if err != nil { t.Fatalf("Go adapter rejected valid SQL capability: %v", err) }
    if identity.UserID != userID || identity.Role != "school_admin" || identity.SchoolID == nil || *identity.SchoolID != schoolID {
        t.Fatalf("Go adapter returned unexpected identity: user=%s role=%q school=%v", identity.UserID, identity.Role, identity.SchoolID)
    }
    if _, err := adapter.AuthenticateAccessSession(context.Background(), uuid.New(), userID); !errors.Is(err, authbootstrap.ErrUnauthenticated) {
        t.Fatalf("Go adapter should reject an unknown session as unauthenticated, got %v", err)
    }

    if _, err := conn.ExecContext(context.Background(), "SELECT password_hash FROM "+qs+".users"); err == nil { t.Fatal("runtime unexpectedly read password_hash / users table") }
    if _, err := conn.ExecContext(context.Background(), "SELECT refresh_token_hash FROM "+qs+".sessions"); err == nil { t.Fatal("runtime unexpectedly read refresh_token_hash / sessions table") }

    exec("RESET ROLE")
    newSchool := uuid.New()
    exec(fmt.Sprintf("UPDATE %s.users SET role='teacher',school_id=$2 WHERE id=$1", qs), userID,newSchool)
    exec("SET ROLE "+qr)
    if err := conn.QueryRowContext(context.Background(), query, sessionID,userID).Scan(&gotUser,&gotRole,&gotSchool); err != nil { t.Fatalf("updated user rejected: %v",err) }
    if gotRole != "teacher" || gotSchool != newSchool { t.Fatalf("bootstrap did not observe current authority: role=%q school=%s",gotRole,newSchool) }

    exec("RESET ROLE")
    exec(fmt.Sprintf("UPDATE %s.sessions SET revoked=true WHERE id=$1",qs),sessionID)
    exec("SET ROLE "+qr)
    assertNoAuthRow(t,conn,countQuery,sessionID,userID,"revoked session")

    exec("RESET ROLE")
    exec(fmt.Sprintf("UPDATE %s.sessions SET revoked=false,expires_at=now()-interval '1 second' WHERE id=$1",qs),sessionID)
    exec("SET ROLE "+qr)
    assertNoAuthRow(t,conn,countQuery,sessionID,userID,"expired session")

    exec("RESET ROLE")
    exec(fmt.Sprintf("UPDATE %s.sessions SET expires_at=now()+interval '1 hour' WHERE id=$1",qs),sessionID)
    exec("SET ROLE "+qr)
    assertNoAuthRow(t,conn,countQuery,sessionID,uuid.New(),"session/user mismatch")

    exec("RESET ROLE")
    exec(fmt.Sprintf("UPDATE %s.users SET active=false WHERE id=$1",qs),userID)
    exec("SET ROLE "+qr)
    assertNoAuthRow(t,conn,countQuery,sessionID,userID,"inactive user")

    exec("RESET ROLE")
    var ownerLogin,ownerBypass,runtimeLogin,runtimeBypass,runtimeMember bool
    if err:=conn.QueryRowContext(context.Background(),"SELECT rolcanlogin,rolbypassrls FROM pg_roles WHERE rolname=$1",owner).Scan(&ownerLogin,&ownerBypass);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),"SELECT rolcanlogin,rolbypassrls FROM pg_roles WHERE rolname=$1",runtime).Scan(&runtimeLogin,&runtimeBypass);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),"SELECT pg_has_role($1,$2,'MEMBER')",runtime,owner).Scan(&runtimeMember);err!=nil{t.Fatal(err)}
    if ownerLogin||ownerBypass||runtimeLogin||runtimeBypass||runtimeMember{t.Fatalf("unsafe role attributes/membership: owner(login=%t bypass=%t) runtime(login=%t bypass=%t member=%t)",ownerLogin,ownerBypass,runtimeLogin,runtimeBypass,runtimeMember)}

    // The function owner is not a table owner and must see no rows when the
    // function has not installed its transaction-local lookup context.
    var ownerOwnsUsers, ownerOwnsSessions bool
    if err:=conn.QueryRowContext(context.Background(),
        "SELECT c.relowner=(SELECT oid FROM pg_roles WHERE rolname=$1) FROM pg_class c WHERE c.oid=$2::regclass",
        owner,schema+".users").Scan(&ownerOwnsUsers);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),
        "SELECT c.relowner=(SELECT oid FROM pg_roles WHERE rolname=$1) FROM pg_class c WHERE c.oid=$2::regclass",
        owner,schema+".sessions").Scan(&ownerOwnsSessions);err!=nil{t.Fatal(err)}
    if ownerOwnsUsers||ownerOwnsSessions{t.Fatal("auth function owner unexpectedly owns a protected table")}
    var usersPolicy, sessionsPolicy string
    if err:=conn.QueryRowContext(context.Background(),
        "SELECT qual FROM pg_policies WHERE schemaname=$1 AND tablename='users' AND policyname='auth_bootstrap_function_owner_read'",schema).Scan(&usersPolicy);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),
        "SELECT qual FROM pg_policies WHERE schemaname=$1 AND tablename='sessions' AND policyname='auth_bootstrap_function_owner_read'",schema).Scan(&sessionsPolicy);err!=nil{t.Fatal(err)}
    if !strings.Contains(usersPolicy,"current_setting")||!strings.Contains(sessionsPolicy,"current_setting")||strings.Trim(strings.TrimSpace(strings.ToLower(usersPolicy)),"() ")=="true"||strings.Trim(strings.TrimSpace(strings.ToLower(sessionsPolicy)),"() ")=="true"{
        t.Fatalf("auth owner RLS policies are not lookup-scoped: users=%q sessions=%q",usersPolicy,sessionsPolicy)
    }
    exec("SET ROLE "+qo)
    var ownerVisibleUsers, ownerVisibleSessions int
    if err:=conn.QueryRowContext(context.Background(),"SELECT count(id) FROM "+qs+".users").Scan(&ownerVisibleUsers);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),"SELECT count(id) FROM "+qs+".sessions").Scan(&ownerVisibleSessions);err!=nil{t.Fatal(err)}
    if ownerVisibleUsers!=0||ownerVisibleSessions!=0{t.Fatalf("function owner sees rows without lookup context: users=%d sessions=%d",ownerVisibleUsers,ownerVisibleSessions)}
    exec("RESET ROLE")

    exec("UPDATE "+qs+".users SET active=true WHERE id=$1",userID)
    exec("ALTER TABLE "+qs+".users FORCE ROW LEVEL SECURITY")
    exec("ALTER TABLE "+qs+".sessions FORCE ROW LEVEL SECURITY")
    var forceUsers,forceSessions,ownerPassword,runtimeTable,runtimeExecute bool
    if err:=conn.QueryRowContext(context.Background(),"SELECT relforcerowsecurity FROM pg_class WHERE oid=$1::regclass",schema+".users").Scan(&forceUsers);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),"SELECT relforcerowsecurity FROM pg_class WHERE oid=$1::regclass",schema+".sessions").Scan(&forceSessions);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),"SELECT has_column_privilege($1,$2,'password_hash','SELECT')",owner,schema+".users").Scan(&ownerPassword);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),"SELECT has_table_privilege($1,$2,'SELECT')",runtime,schema+".users").Scan(&runtimeTable);err!=nil{t.Fatal(err)}
    if err:=conn.QueryRowContext(context.Background(),"SELECT has_function_privilege($1,$2,'EXECUTE')",runtime,schema+".authenticate_access_session(uuid,uuid)").Scan(&runtimeExecute);err!=nil{t.Fatal(err)}
    if !forceUsers||!forceSessions||ownerPassword||runtimeTable||!runtimeExecute{t.Fatalf("RLS/privilege invariant failed: force_users=%t force_sessions=%t owner_password=%t runtime_table=%t runtime_execute=%t",forceUsers,forceSessions,ownerPassword,runtimeTable,runtimeExecute)}

    exec("SET ROLE "+qr)
    exec("SET search_path=pg_catalog")
    if err:=conn.QueryRowContext(context.Background(),query,sessionID,userID).Scan(&gotUser,&gotRole,&gotSchool);err!=nil{t.Fatalf("fixed search_path failed under caller search_path: %v",err)}
    if gotUser!=userID||gotRole!="teacher"||gotSchool!=newSchool{t.Fatal("caller search_path changed bootstrap result")}

    // Re-run the Go adapter after FORCE ROW LEVEL SECURITY is enabled on both
    // protected tables, proving the real runtime call path survives forced RLS.
    identity, err = adapter.AuthenticateAccessSession(context.Background(), sessionID, userID)
    if err != nil { t.Fatalf("Go adapter failed under FORCE ROW LEVEL SECURITY: %v", err) }
    if identity.UserID != userID || identity.Role != "teacher" || identity.SchoolID == nil || *identity.SchoolID != newSchool {
        t.Fatalf("Go adapter returned unexpected forced-RLS identity: user=%s role=%q school=%v", identity.UserID, identity.Role, identity.SchoolID)
    }
}

func assertNoAuthRow(t *testing.T, conn *sql.Conn, query string, sessionID,userID uuid.UUID,label string) {
    t.Helper()
    var count int
    if err:=conn.QueryRowContext(context.Background(),query,sessionID,userID).Scan(&count);err!=nil{t.Fatalf("%s: function error: %v",label,err)}
    if count!=0{t.Fatalf("%s: expected no auth row, got %d",label,count)}
}

func quoteIdentifier(value string) string { return "\"" + value + "\"" }


// splitSQLStatements separates SQL commands without splitting semicolons inside
// dollar-quoted function bodies.
func splitSQLStatements(script string) []string {
    var statements []string
    start := 0
    var dollarTag string
    for i := 0; i < len(script); {
        if dollarTag == "" && script[i] == '$' {
            end := strings.IndexByte(script[i+1:], '$')
            if end >= 0 {
                tag := script[i : i+end+2]
                if strings.Trim(tag[1:len(tag)-1], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") == "" {
                    dollarTag = tag
                    i += len(tag)
                    continue
                }
            }
        }
        if dollarTag != "" && strings.HasPrefix(script[i:], dollarTag) {
            i += len(dollarTag)
            dollarTag = ""
            continue
        }
        if dollarTag == "" && script[i] == ';' {
            statements = append(statements, script[start:i])
            start = i + 1
        }
        i++
    }
    if tail := strings.TrimSpace(script[start:]); tail != "" {
        statements = append(statements, tail)
    }
    return statements
}
