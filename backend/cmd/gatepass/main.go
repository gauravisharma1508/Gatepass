// Command gatepass is the Gatepass server. Sub-commands: serve (default), migrate, seed, create-admin.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gatepass/internal/app"
	"gatepass/internal/platform/config"
	"gatepass/internal/platform/db"
	"gatepass/webui"
)

func main() {
	lg := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(lg)
	cmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	if err := run(cmd, args, lg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string, lg *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "migrate":
		if len(args) != 1 {
			return errors.New("usage: gatepass migrate up|down|reset|status")
		}
		return db.Migrate(ctx, migrateURL(cfg), args[0], os.Stdout)
	case "seed":
		if cfg.Production() {
			return errors.New("seed refuses to run when APP_ENV=production")
		}
		a, err := app.New(cfg, app.Options{Log: lg})
		if err != nil {
			return err
		}
		defer a.Close()
		return a.Seed(ctx, cfg.SeedPassword, os.Stdout)
	case "create-admin":
		fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
		email := fs.String("email", "", "admin email (required)")
		name := fs.String("name", "Administrator", "display name")
		code := fs.String("code", "ADMIN001", "employee code")
		if err := fs.Parse(args); err != nil || *email == "" {
			return errors.New("usage: gatepass create-admin --email you@company.com [--name ...] [--code ...]")
		}
		a, err := app.New(cfg, app.Options{Log: lg})
		if err != nil {
			return err
		}
		defer a.Close()
		return a.CreateAdmin(ctx, *email, *name, *code, os.Stdout)
	case "serve":
		return serve(ctx, cfg, lg)
	}
	return fmt.Errorf("unknown command %q (use serve, migrate, seed, create-admin)", cmd)
}

func migrateURL(cfg config.Config) string {
	if cfg.MigrateDatabaseURL != "" {
		return cfg.MigrateDatabaseURL
	}
	return cfg.DatabaseURL
}

func serve(ctx context.Context, cfg config.Config, lg *slog.Logger) error {
	if cfg.MigrateDatabaseURL != "" { // migrations run before the server starts; a failure stops start-up
		if err := db.Migrate(ctx, cfg.MigrateDatabaseURL, "up", os.Stdout); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	a, err := app.New(cfg, app.Options{Log: lg, Static: webui.FS()})
	if err != nil {
		return err
	}
	defer a.Close()
	a.Scheduler.Start()

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: a.Handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	srv.RegisterOnShutdown(a.Hub.CloseAll) // end the event streams so shutdown can finish

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	lg.Info("gatepass listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	lg.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	a.Scheduler.Stop(shutdownCtx)
	return err
}
