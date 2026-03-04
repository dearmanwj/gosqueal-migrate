package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"hash/crc64"
	"io/fs"
)

func RunMigrations(db *sql.DB, sqlFiles embed.FS) error {
	createHistoryTable(db)

	return fs.WalkDir(sqlFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			sqlBytes, err := fs.ReadFile(sqlFiles, path)
			if err != nil {
				return err
			}
			return executeMigration(db, d.Name(), sqlBytes)
		}

		return err
	})

}

func executeMigration(db *sql.DB, filename string, sqlBytes []byte) error {
	row := db.QueryRow("SELECT hash, applied FROM migration_histories WHERE name = $1", filename)
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
		_, err := db.Exec("INSERT INTO migration_histories (name, hash, applied) VALUES ($1, $2, false)", filename, incomingChecksumHex)
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
	_, err = db.Exec("UPDATE migration_histories SET applied = true WHERE name = $1", filename)
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
