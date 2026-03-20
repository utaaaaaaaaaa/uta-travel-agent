#!/bin/bash
# Run database migrations

set -e

MIGRATIONS_DIR="./internal/storage/postgres/migrations"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASS="${DB_PASS:-postgres}"
DB_NAME="${DB_NAME:-uta_travel}"

echo "Running migrations..."
echo "Database: $DB_HOST:$DB_PORT/$DB_NAME"

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo "Error: psql is not installed"
    exit 1
fi

# Run all .up.sql files in order
for migration in $(ls -1 $MIGRATIONS_DIR/*.up.sql 2>/dev/null | sort -V); do
    filename=$(basename "$migration")
    echo "Applying: $filename"

    PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$migration" || {
        echo "Error applying migration: $filename"
        exit 1
    }

    echo "Applied: $filename"
done

echo "All migrations completed successfully!"