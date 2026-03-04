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
var oneFileSet embed.FS

func TestRunOneMigration(t *testing.T) {
	// Given
	db := setupTestDB(t)

	// When
	err := RunMigrations(db, oneFileSet)

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

//go:embed db/twofiles/*.sql
var twoFileSet embed.FS

func TestRunTwoMigrations(t *testing.T) {
	// Given
	db := setupTestDB(t)

	// When
	err := RunMigrations(db, twoFileSet)

	// Then
	if err != nil {
		t.Fatalf("error running migrations %v", err)
	}
	checkTableQuery1 := "INSERT INTO users (id, name) VALUES (1, 'john')"
	_, err = db.Exec(checkTableQuery1)

	if err != nil {
		log.Printf("error running test %v", err)
		t.Fail()
	}

	checkTableQuery2 := "INSERT INTO new_table (col1, col2) VALUES (1, 2)"
	_, err = db.Exec(checkTableQuery2)

	if err != nil {
		log.Printf("error running test %v", err)
		t.Fail()
	}
}

//go:embed db/onefile-corrupted/*.sql
var oneFileCorruptedSet embed.FS

func TestShouldFail_WhenReRunChangedMigration(t *testing.T) {
	// Given
	db := setupTestDB(t)

	// When
	_ = RunMigrations(db, oneFileSet)
	err := RunMigrations(db, oneFileCorruptedSet)

	// Then
	if err == nil {
		t.Fatalf("second migration should have failed")
	}
}
