package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 验证文件存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file not created")
	}

	// 验证 tables 存在
	tables := []string{"sessions", "messages", "feeds", "articles"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestOpen_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested", "dir", "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("nested directory not created")
	}
}

func TestOpen_WALMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var mode string
	db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("expected WAL mode, got %s", mode)
	}
}

func TestOpen_SessionCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Insert session
	_, err = db.Exec("INSERT INTO sessions (id) VALUES (?)", "sess-001")
	if err != nil {
		t.Fatalf("insert session failed: %v", err)
	}

	// Insert message
	_, err = db.Exec("INSERT INTO messages (session_id, role, content) VALUES (?, ?, ?)",
		"sess-001", "user", "hello")
	if err != nil {
		t.Fatalf("insert message failed: %v", err)
	}

	// Query
	var count int
	db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = ?", "sess-001").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 message, got %d", count)
	}

	// Cascade delete
	_, err = db.Exec("DELETE FROM sessions WHERE id = ?", "sess-001")
	if err != nil {
		t.Fatalf("delete session failed: %v", err)
	}

	db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = ?", "sess-001").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", count)
	}
}

func TestOpen_ArticleDedupe(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// First insert
	_, err = db.Exec(`INSERT INTO articles (guid, feed_id, title, link) VALUES (?, ?, ?, ?)`,
		"guid-001", "feed-1", "Title", "https://example.com/1")
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Duplicate GUID - should be ignored
	_, err = db.Exec(`INSERT OR IGNORE INTO articles (guid, feed_id, title, link) VALUES (?, ?, ?, ?)`,
		"guid-001", "feed-1", "Dupe Title", "https://example.com/2")
	if err != nil {
		t.Fatalf("duplicate insert failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 article after dedup, got %d", count)
	}
}
