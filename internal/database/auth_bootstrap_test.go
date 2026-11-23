package database

import (
    "context"
    "database/sql"
    "embed"
    "fmt"
    "net/url"
    "os"
    "strings"
    "testing"

    "github.com/google/uuid"
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
    // Statements are separated by semicolon/newline; the function body contains
    // no internal semicolon, so each command is executed independently.
    for _, statement := range strings.Split(script, ";\n") {
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
}

func assertNoAuthRow(t *testing.T, conn *sql.Conn, query string, sessionID,userID uuid.UUID,label string) {
    t.Helper()
    var count int
    if err:=conn.QueryRowContext(context.Background(),query,sessionID,userID).Scan(&count);err!=nil{t.Fatalf("%s: function error: %v",label,err)}
    if count!=0{t.Fatalf("%s: expected no auth row, got %d",label,count)}
}

func quoteIdentifier(value string) string { return "\"" + value + "\"" }
