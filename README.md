# Gosqueal Migrate

This library provides the bare minimum for you to get started with schema migrations in golang. No unnecessary dependencies/bloat.

Key features:
- It integrates directly with the native `database/sql` package and is db/driver agnostic
- Checksum validation ensures integrity and immutability of processed migrations
- Tiny footprint, pure native go

## Installation

```bash
go get github.com/dearmanwj/gosqueal-migrate
```

## Usage

1. Create a directory for your SQL migration files:

```
migrations/
├── 001_create_users.sql
├── 002_add_email_column.sql
└── 003_create_orders.sql
```

2. Embed the migrations and run them against your database:

```go
package main

import (
	"database/sql"
	"embed"
	"log"

	gosqueal "github.com/dearmanwj/gosqueal-migrate"
	_ "github.com/lib/pq" // or your preferred driver
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	db, err := sql.Open("postgres", "postgres://localhost/mydb?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := gosqueal.Run(db, migrations); err != nil {
		log.Fatal(err)
	}

	log.Println("Migrations applied successfully")
}
```

## How it works

- Migrations are applied in filesystem order (alphabetical)
- A `migration_histories` table tracks applied migrations and their checksums
- Re-running is safe: already-applied migrations are skipped
- If a migration file is modified after being applied, an error is returned to prevent inconsistencies

## Tested drivers

| Database   | Driver                          | Status |
|------------|--------------------------------|--------|
| PostgreSQL | `github.com/lib/pq`            | ✅      |
| MySQL      | `github.com/go-sql-driver/mysql` | ✅      |
| SQLite     | `modernc.org/sqlite`           | ✅      |

Other `database/sql` compatible drivers should work but are not explicitly tested.
