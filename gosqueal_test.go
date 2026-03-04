package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:memdb_%s?mode=memory&cache=shared", t.Name())
	// dsn := "app.db"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	return db
}

//go:embed db/onefile/*.sql
var files embed.FS

func TestRunMigrations(t *testing.T) {
	// Given
	db := setupTestDB(t)

	// When
	err := RunMigrations(db, files)

	// Then
	if err != nil {
		t.Fatalf("error running migrations %v", err)
	}
	checkTableQuery := "INSERT INTO users (id, name) VALUES (1, 'john')"
	_, err = db.Exec(checkTableQuery)

	if err != nil {
		log.Printf("error running test %v", err)
		t.Fail()
	}
}
