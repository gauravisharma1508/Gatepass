package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	"gatepass/db/migrations"
)

// Migrate runs a goose command ("up", "down", "reset", "status") with the owner connection URL.
func Migrate(ctx context.Context, url, cmd string, out io.Writer) error {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return err
	}
	switch cmd {
	case "up":
		res, err := p.Up(ctx)
		for _, r := range res {
			fmt.Fprintf(out, "applied %s\n", r.Source.Path)
		}
		return err
	case "down":
		_, err := p.Down(ctx)
		return err
	case "reset":
		_, err := p.DownTo(ctx, 0)
		return err
	case "status":
		st, err := p.Status(ctx)
		for _, s := range st {
			fmt.Fprintf(out, "%-8s %s\n", s.State, s.Source.Path)
		}
		return err
	}
	return fmt.Errorf("unknown migrate command %q (use up, down, reset, status)", cmd)
}
