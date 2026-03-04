package gosqueal

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/server"
	gsql "github.com/dolthub/go-mysql-server/sql"
	_ "github.com/go-sql-driver/mysql"
)

var (
	mysqlServerOnce sync.Once
	mysqlTestPort   = 33066
)

func startMySQLTestServer() {
	mysqlServerOnce.Do(func() {
		db := memory.NewDatabase("testdb")
		provider := memory.NewDBProvider(db)
		engine := sqle.NewDefault(provider)

		config := server.Config{
			Protocol: "tcp",
			Address:  fmt.Sprintf("127.0.0.1:%d", mysqlTestPort),
		}

		s, err := server.NewServer(config, engine, gsql.NewContext, memory.NewSessionBuilder(provider), nil)
		if err != nil {
			panic(fmt.Sprintf("failed to create mysql test server: %v", err))
		}

		go func() {
			if err := s.Start(); err != nil && !strings.Contains(err.Error(), "Server closed") {
				panic(fmt.Sprintf("failed to start mysql test server: %v", err))
			}
		}()

		time.Sleep(100 * time.Millisecond)
	})
}

func setupMySQLTestDB(t *testing.T) *sql.DB {
	t.Helper()

	startMySQLTestServer()

	dsn := fmt.Sprintf("root:@tcp(127.0.0.1:%d)/testdb", mysqlTestPort)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to open mysql connection: %v", err)
	}

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS migration_histories")
		db.Exec("DROP TABLE IF EXISTS users")
		db.Exec("DROP TABLE IF EXISTS new_table")
		db.Close()
	})

	return db
}

func TestMySQL_RunOneMigration(t *testing.T) {
	// Given
	db := setupMySQLTestDB(t)

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

func TestMySQL_RunTwoMigrations(t *testing.T) {
	// Given
	db := setupMySQLTestDB(t)

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

func TestMySQL_ShouldFail_WhenReRunChangedMigration(t *testing.T) {
	// Given
	db := setupMySQLTestDB(t)

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

	expectedErrMsg := "does not match applied hash"
	if err != nil && !contains(err.Error(), expectedErrMsg) {
		t.Fatalf("expected error containing %q, got: %v", expectedErrMsg, err)
	}
}

func TestMySQL_IdempotentMigrations(t *testing.T) {
	// Given
	db := setupMySQLTestDB(t)

	// When - run the same migration twice
	err := Run(db, oneFileSet)
	if err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}

	err = Run(db, oneFileSet)
	if err != nil {
		t.Fatalf("second migration run should succeed (idempotent): %v", err)
	}

	// Then - verify the table exists and has correct structure
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM migration_histories").Scan(&count)
	if err != nil {
		t.Fatalf("error querying migration_histories: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 migration record, got %d", count)
	}
}

func TestMySQL_MigrationHistoryTracking(t *testing.T) {
	// Given
	db := setupMySQLTestDB(t)

	// When
	err := Run(db, twoFileSet)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Then - verify migration history is properly tracked
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var _ = fmt.Sprintf("%v", oneFileSet)
var _ = fmt.Sprintf("%v", twoFileSet)
var _ = fmt.Sprintf("%v", oneFileCorruptedSet)
