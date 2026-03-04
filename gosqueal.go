package gosqueal

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"hash/crc64"
	"io/fs"
	"reflect"
	"strings"
)

type dialect int

const (
	dialectMySQL dialect = iota
	dialectPostgres
	dialectSQLite
)

func detectDialect(db *sql.DB) dialect {
	driverType := reflect.TypeOf(db.Driver()).String()
	switch {
	case strings.Contains(driverType, "pq") || strings.Contains(driverType, "pgx") || strings.Contains(driverType, "postgres"):
		return dialectPostgres
	case strings.Contains(driverType, "mysql"):
		return dialectMySQL
	default:
		return dialectSQLite
	}
}

func placeholder(d dialect, n int) string {
	if d == dialectPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// Run applies all SQL migrations from the embedded filesystem to the database.
// It creates a migration_histories table to track applied migrations and their checksums.
func Run(db *sql.DB, sqlFiles embed.FS) error {
	createHistoryTable(db)

	dialect := detectDialect(db)

	return fs.WalkDir(sqlFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			sqlBytes, err := fs.ReadFile(sqlFiles, path)
			if err != nil {
				return err
			}
			return executeMigration(db, dialect, d.Name(), sqlBytes)
		}

		return err
	})

}

func executeMigration(db *sql.DB, d dialect, filename string, sqlBytes []byte) error {
	query := fmt.Sprintf("SELECT hash, applied FROM migration_histories WHERE name = %s", placeholder(d, 1))
	row := db.QueryRow(query, filename)
	var storedHash string
	var applied bool
	err := row.Scan(&storedHash, &applied)

	newMigration := false
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			newMigration = true
		} else {
			return fmt.Errorf("could not read migration histories table for migration %v, error: %w", filename, err)
		}
	}

	table := crc64.MakeTable(crc64.ECMA)
	incomingChecksum := crc64.Checksum(sqlBytes, table)
	incomingChecksumHex := fmt.Sprintf("%016x", incomingChecksum)

	if newMigration {
		query := fmt.Sprintf("INSERT INTO migration_histories (name, hash, applied) VALUES (%s, %s, false)", placeholder(d, 1), placeholder(d, 2))
		_, err := db.Exec(query, filename, incomingChecksumHex)
		if err != nil {
			return fmt.Errorf("could not insert migration into history table %w", err)
		}
	} else {
		if incomingChecksumHex != storedHash {
			return fmt.Errorf("migration file %v does not match applied hash", filename)
		}
		if applied {
			return nil
		}
	}

	_, err = db.Exec(string(sqlBytes))
	if err != nil {
		return fmt.Errorf("could not apply migration %v, sql error %w", filename, err)
	}
	query = fmt.Sprintf("UPDATE migration_histories SET applied = true WHERE name = %s", placeholder(d, 1))
	_, err = db.Exec(query, filename)
	if err != nil {
		return fmt.Errorf("error updating row in histories after successful migration %w", err)
	}
	return nil
}

func createHistoryTable(db *sql.DB) error {
	/*
		See if table histories exists
		If not create table with columns: filename, hash, applied
	*/
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migration_histories (
	name VARCHAR(50) NOT NULL, 
	hash VARCHAR(50) NOT NULL, 
	applied BOOLEAN NOT NULL)`)
	return err
}
