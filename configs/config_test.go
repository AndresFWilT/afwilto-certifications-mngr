package configs

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("STORAGE_DIR", "")
	t.Setenv("ALLOW_ORIGINS", "")
	t.Setenv("MAX_UPLOAD_BYTES", "")
	t.Setenv("MIGRATE_ON_START", "")

	cfg := Load()

	if cfg.Port != "8082" {
		t.Errorf("Port = %q, want 8082", cfg.Port)
	}
	if cfg.StorageDir != "./data/uploads" {
		t.Errorf("StorageDir = %q, want ./data/uploads", cfg.StorageDir)
	}
	if cfg.MaxUploadBytes != 10485760 {
		t.Errorf("MaxUploadBytes = %d, want 10485760", cfg.MaxUploadBytes)
	}
	if !cfg.MigrateOnStart {
		t.Error("MigrateOnStart = false, want true by default")
	}
	wantOrigins := []string{"http://localhost:3000", "http://localhost:3006"}
	if len(cfg.AllowOrigins) != len(wantOrigins) {
		t.Fatalf("AllowOrigins = %v, want %v", cfg.AllowOrigins, wantOrigins)
	}
	for i, o := range wantOrigins {
		if cfg.AllowOrigins[i] != o {
			t.Errorf("AllowOrigins[%d] = %q, want %q", i, cfg.AllowOrigins[i], o)
		}
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
	t.Setenv("STORAGE_DIR", "/data/uploads")
	t.Setenv("ALLOW_ORIGINS", "http://a.test, http://b.test")
	t.Setenv("MAX_UPLOAD_BYTES", "1024")
	t.Setenv("MIGRATE_ON_START", "false")

	cfg := Load()

	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want 9000", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://u:p@h:5432/db" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.StorageDir != "/data/uploads" {
		t.Errorf("StorageDir = %q", cfg.StorageDir)
	}
	if cfg.MaxUploadBytes != 1024 {
		t.Errorf("MaxUploadBytes = %d, want 1024", cfg.MaxUploadBytes)
	}
	if cfg.MigrateOnStart {
		t.Error("MigrateOnStart = true, want false")
	}
	if len(cfg.AllowOrigins) != 2 || cfg.AllowOrigins[0] != "http://a.test" || cfg.AllowOrigins[1] != "http://b.test" {
		t.Errorf("AllowOrigins = %v, want trimmed pair", cfg.AllowOrigins)
	}
}
