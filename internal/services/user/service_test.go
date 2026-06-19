package user

import (
	"database/sql"
	"testing"

	"raggo/internal/config"
	"raggo/internal/database"
)

func makeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func makeService(db *sql.DB, secret string) *Service {
	return &Service{
		DB: db,
		Cfg: &config.Config{
			SecretKey:     secret,
			AdminUsername: "admin",
			AdminPassword: "adminpass",
		},
	}
}

func TestSeedAdmin_CreatesAdmin(t *testing.T) {
	db := makeTestDB(t)
	svc := makeService(db, "secret")

	if err := svc.SeedAdmin(); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}

	u, err := database.GetUserByUsername(db, "admin")
	if err != nil {
		t.Fatalf("admin not found: %v", err)
	}
	if u.Role != "admin" {
		t.Errorf("expected role admin, got %s", u.Role)
	}
}

func TestSeedAdmin_Idempotent(t *testing.T) {
	db := makeTestDB(t)
	svc := makeService(db, "secret")

	if err := svc.SeedAdmin(); err != nil {
		t.Fatalf("first SeedAdmin: %v", err)
	}
	if err := svc.SeedAdmin(); err != nil {
		t.Fatalf("second SeedAdmin: %v", err)
	}
}

func TestLogin_ValidCredentials(t *testing.T) {
	db := makeTestDB(t)
	svc := makeService(db, "secret")

	if err := svc.SeedAdmin(); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}

	access, refresh, err := svc.Login("admin", "adminpass")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if access == "" || refresh == "" {
		t.Error("expected non-empty tokens")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	db := makeTestDB(t)
	svc := makeService(db, "secret")

	if err := svc.SeedAdmin(); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}

	_, _, err := svc.Login("admin", "wrongpassword")
	if err == nil {
		t.Error("expected error for invalid password")
	}
}

func TestRefreshToken_Valid(t *testing.T) {
	db := makeTestDB(t)
	svc := makeService(db, "secret")

	if err := svc.SeedAdmin(); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}

	_, refresh, err := svc.Login("admin", "adminpass")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	newAccess, err := svc.RefreshToken(refresh)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if newAccess == "" {
		t.Error("expected non-empty new access token")
	}
}

func TestRefreshToken_AccessTokenRejected(t *testing.T) {
	db := makeTestDB(t)
	svc := makeService(db, "secret")

	if err := svc.SeedAdmin(); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}

	access, _, err := svc.Login("admin", "adminpass")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Passing access token where refresh expected
	_, err = svc.RefreshToken(access)
	if err == nil {
		t.Error("expected error when using access token as refresh")
	}
}
