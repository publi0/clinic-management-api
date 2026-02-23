package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel/attribute"
)

func OpenPostgres(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := otelsql.Open(
		"pgx",
		databaseURL,
		otelsql.WithAttributes(attribute.String("db.system", "postgresql")),
	)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}
