package gosqueal

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to open postgres connection: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestPostgres_RunOneMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration test in short mode")
	}

	// Given
	db := setupPostgresTestDB(t)

	// When
	err := Run(db, oneFileSet)

	// Then
	if err != nil {
		t.Fatalf("error running migrations: %v", err)
	}

	checkTableQuery := "INSERT INTO users (id, name) VALUES (1, 'john')"
	_, err = db.Exec(checkTableQuery)
	if err != nil {
		t.Fatalf("error inserting into users table: %v", err)
	}
}

func TestPostgres_RunTwoMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration test in short mode")
	}

	// Given
	db := setupPostgresTestDB(t)

	// When
	err := Run(db, twoFileSet)

	// Then
	if err != nil {
		t.Fatalf("error running migrations: %v", err)
	}

	checkTableQuery1 := "INSERT INTO users (id, name) VALUES (1, 'john')"
	_, err = db.Exec(checkTableQuery1)
	if err != nil {
		t.Fatalf("error inserting into users table: %v", err)
	}

	checkTableQuery2 := "INSERT INTO new_table (col1, col2) VALUES (1, 2)"
	_, err = db.Exec(checkTableQuery2)
	if err != nil {
		t.Fatalf("error inserting into new_table: %v", err)
	}
}

func TestPostgres_ShouldFail_WhenReRunChangedMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration test in short mode")
	}

	// Given
	db := setupPostgresTestDB(t)

	// When
	err := Run(db, oneFileSet)
	if err != nil {
		t.Fatalf("first migration should succeed: %v", err)
	}

	err = Run(db, oneFileCorruptedSet)

	// Then
	if err == nil {
		t.Fatalf("second migration should have failed due to checksum mismatch")
	}
}

func TestPostgres_IdempotentMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration test in short mode")
	}

	// Given
	db := setupPostgresTestDB(t)

	// When
	err := Run(db, oneFileSet)
	if err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}

	err = Run(db, oneFileSet)
	if err != nil {
		t.Fatalf("second migration run should succeed (idempotent): %v", err)
	}

	// Then
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM migration_histories").Scan(&count)
	if err != nil {
		t.Fatalf("error querying migration_histories: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 migration record, got %d", count)
	}
}

func TestPostgres_MigrationHistoryTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration test in short mode")
	}

	// Given
	db := setupPostgresTestDB(t)

	// When
	err := Run(db, twoFileSet)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Then
	rows, err := db.Query("SELECT name, applied FROM migration_histories ORDER BY name")
	if err != nil {
		t.Fatalf("error querying migration_histories: %v", err)
	}
	defer rows.Close()

	var migrations []struct {
		name    string
		applied bool
	}

	for rows.Next() {
		var m struct {
			name    string
			applied bool
		}
		if err := rows.Scan(&m.name, &m.applied); err != nil {
			t.Fatalf("error scanning row: %v", err)
		}
		migrations = append(migrations, m)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	for _, m := range migrations {
		if !m.applied {
			t.Errorf("migration %s should be marked as applied", m.name)
		}
	}
}
