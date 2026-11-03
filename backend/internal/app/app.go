// Package app is the composition root: the only place that knows every module. It builds the
// platform services, wires the modules (spec §3.1), assembles the router and registers the jobs.
package app

import (
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"gorm.io/gorm"

	"gatepass/internal/modules/audit"
	"gatepass/internal/modules/auth"
	"gatepass/internal/modules/notifications"
	"gatepass/internal/modules/photos"
	"gatepass/internal/modules/reports"
	"gatepass/internal/modules/settings"
	"gatepass/internal/modules/users"
	"gatepass/internal/modules/visitors"
	"gatepass/internal/modules/visits"
	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/config"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/httpx"
	"gatepass/internal/platform/mailer"
	"gatepass/internal/platform/password"
	"gatepass/internal/platform/scheduler"
	"gatepass/internal/platform/storage"
)

// Options override the production defaults (tests use them).
type Options struct {
	Log            *slog.Logger
	Clock          *clock.Settable
	Store          storage.Store
	Mailer         mailer.Mailer
	Static         fs.FS         // built web app; nil = API only
	LoginRateLimit int           // requests per minute per IP on login/forgot-password (default 10)
	SyncEmail      bool          // send email inline (tests)
	SessionCleanup time.Duration // expired-session cleanup interval (default 5 min)
}

type App struct {
	Handler   http.Handler
	Scheduler *scheduler.Scheduler
	DB        *gorm.DB
	Clock     *clock.Settable
	Hasher    *password.Hasher
	Users     *users.Service
	Visits    *visits.Service
	Photos    *photos.Service
	Settings  *settings.Service
	Hub       *notifications.Hub
	Log       *slog.Logger
}

func New(cfg config.Config, o Options) (*App, error) {
	lg := o.Log
	if lg == nil {
		lg = slog.Default()
	}
	clk := o.Clock
	if clk == nil {
		clk = clock.New()
	}
	if o.LoginRateLimit == 0 {
		o.LoginRateLimit = 10
	}
	switch {
	case o.SessionCleanup == 0:
		o.SessionCleanup = 5 * time.Minute
	case o.SessionCleanup < 0: // tests: no background cleanup goroutine
		o.SessionCleanup = 0
	}
	st := o.Store
	if st == nil {
		d, err := storage.NewDisk(cfg.StorageDir)
		if err != nil {
			return nil, err
		}
		st = d
	}
	ml := o.Mailer
	if ml == nil {
		if cfg.SMTPHost != "" {
			ml = mailer.NewSMTP(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.SMTPUser, cfg.SMTPPass)
		} else {
			ml = mailer.Log{Log: lg}
		}
	}

	g, err := db.Open(cfg.DatabaseURL, clk, lg)
	if err != nil {
		return nil, err
	}
	hasher := password.NewHasher(cfg.BcryptCost)

	// platform-level modules first, then the business modules in dependency order
	auditSvc := audit.New(g, clk)
	settingsSvc := settings.New(g, auditSvc, clk)
	visitorsSvc := visitors.New(g)
	usersSvc := users.New(g, auditSvc, hasher, clk)
	hub := notifications.NewHub()

	authSvc, err := auth.New(auth.Deps{DB: g, Users: usersSvc, Hasher: hasher, Clock: clk, Mailer: ml, WebOrigin: cfg.WebOrigin,
		Production: cfg.Production(), CleanupInterval: o.SessionCleanup, OnRevoke: hub.DisconnectUser})
	if err != nil {
		return nil, err
	}
	usersSvc.SetRevoker(authSvc) // reverse dependency: users -> (interface) -> auth

	notifSvc := notifications.New(notifications.Deps{DB: g, Users: usersSvc, Mailer: ml, Clock: clk, Hub: hub, Log: lg,
		WebOrigin: cfg.WebOrigin, SyncEmail: o.SyncEmail, EmailBackoff: backoff(o.SyncEmail)})
	visitsSvc := visits.New(visits.Deps{DB: g, Clock: clk, Users: usersSvc, Visitors: visitorsSvc, Audit: auditSvc,
		Settings: settingsSvc, Events: notifSvc, Log: lg})
	photosSvc := photos.New(g, st, clk, auditSvc, settingsSvc, lg)
	photosSvc.SetVisits(visitsSvc) // photos needs visits ...
	visitsSvc.SetPhotos(photosSvc) // ... and visits asks photos which photos exist

	sched := scheduler.New(g, clock.OfficeLocation(), lg)
	for _, jobs := range [][]scheduler.Job{visitsSvc.Jobs(), photosSvc.Jobs(), authSvc.Jobs()} {
		for _, j := range jobs {
			if err := sched.Add(j); err != nil {
				return nil, err
			}
		}
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestID, httpx.AccessLog(lg), httpx.Recover(lg), httpx.SecurityHeaders(cfg.Production()))

	loginLimit := httprate.Limit(o.LoginRateLimit, time.Minute, httprate.WithKeyFuncs(remoteIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, apperr.ErrRateLimited) }))

	r.Route("/api", func(api chi.Router) {
		api.Use(httpx.NoStore, auth.BearerBridge, authSvc.Sessions().LoadAndSave, httpx.CSRF(auth.IsBearer), authSvc.Middleware)
		api.NotFound(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, apperr.ErrNotFound) })
		api.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, apperr.New(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed"))
		})
		api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			sqlDB, err := g.DB()
			if err == nil {
				err = sqlDB.PingContext(r.Context())
			}
			if err != nil {
				httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		if cfg.AppEnv == "test" { // never registered in production (spec §14)
			api.Post("/test/clock", httpx.H(func(w http.ResponseWriter, r *http.Request) error {
				var in struct {
					Now *time.Time `json:"now"`
				}
				if err := httpx.DecodeJSON(w, r, &in); err != nil {
					return err
				}
				if in.Now == nil {
					clk.Unfreeze()
				} else {
					clk.Set(*in.Now)
				}
				httpx.WriteJSON(w, http.StatusOK, map[string]any{"now": clk.Now(), "today": clk.Today()})
				return nil
			}))
		}

		authH := auth.NewHandler(authSvc)
		authH.PublicRoutes(api, loginLimit)

		api.Group(func(g chi.Router) {
			g.Use(httpx.RequireAuth, httpx.PasswordGate)
			authH.AuthedRoutes(g)
			users.NewHandler(usersSvc).Routes(g)
			visitors.NewHandler(visitorsSvc).Routes(g)
			visits.NewHandler(visitsSvc).Routes(g)
			photos.NewHandler(photosSvc).Routes(g)
			notifications.NewHandler(notifSvc).Routes(g)
			audit.NewHandler(auditSvc).Routes(g)
			reports.NewHandler(visitsSvc).Routes(g)
			settings.NewHandler(settingsSvc).Routes(g)
		})
	})
	if o.Static != nil {
		r.NotFound(spa(o.Static))
	}

	return &App{Handler: r, Scheduler: sched, DB: g, Clock: clk, Hasher: hasher, Users: usersSvc, Visits: visitsSvc,
		Photos: photosSvc, Settings: settingsSvc, Hub: hub, Log: lg}, nil
}

func backoff(sync bool) time.Duration {
	if sync {
		return time.Millisecond
	}
	return 0
}

// remoteIP keys the rate limiter by the TCP peer (X-Forwarded-For can be forged; configure a trusted proxy before changing this).
func remoteIP(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr, nil
	}
	return host, nil
}

// Close releases the database pool.
func (a *App) Close() {
	if sqlDB, err := a.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// spa serves the built web app, falling back to index.html so client-side routes work on refresh.
func spa(static fs.FS) http.HandlerFunc {
	files := http.FileServerFS(static)
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := static.Open(p); err == nil {
			f.Close()
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(static, "index.html"); err != nil {
			http.Error(w, "The web app is not built. Run `make build` (or use the Vite dev server).", http.StatusNotFound)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	}
}
