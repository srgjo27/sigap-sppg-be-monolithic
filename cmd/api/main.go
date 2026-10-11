package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/config"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/application"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/infrastructure"
	authhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/auth/transport/http"
	menuapplication "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/application"
	menuinfrastructure "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/infrastructure"
	menuhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/transport/http"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/server"
)

func main() {
	cfg := config.Load()

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Println("warning: JWT_SECRET is empty; set JWT_SECRET in production")
		jwtSecret = "dev-only-secret-change-me"
	}

	issuer := infrastructure.NewJWTIssuer(jwtSecret)
	hasher := infrastructure.NewBcryptHasher(cfg.BcryptCost)

	var (
		userStore    application.UserRepository
		refreshStore application.RefreshTokenRepository
		auditStore   application.AuditRepository
		sppgCheck    application.SPPGChecker
		sekolahCheck application.SekolahChecker

		menuBahan   menuapplication.BahanRepository
		menuRepo    menuapplication.MenuRepository
		menuSPPG    menuapplication.SPPGProvider
		menuSekolah menuapplication.SekolahProvider
		menuAudit   menuapplication.AuditRepository
		menuNotifs  menuapplication.NotificationRepository
	)

	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("connect database: %v", err)
		}
		defer pool.Close()
		pg := infrastructure.NewPostgres(pool)
		userStore = pg
		refreshStore = pg
		auditStore = pg
		sppgCheck = pg
		sekolahCheck = pg
		mpg := menuinfrastructure.NewPostgres(pool)
		menuBahan = mpg
		menuRepo = mpg
		menuSPPG = mpg
		menuSekolah = mpg
		menuAudit = mpg
		menuNotifs = mpg
	} else {
		log.Println("DATABASE_URL empty; using in-memory auth stores (dev only)")
		mem := infrastructure.NewMemoryStores()
		userStore = mem
		refreshStore = mem
		auditStore = mem
		sppgCheck = mem
		sekolahCheck = mem
		mmem := menuinfrastructure.NewMemoryStores()
		menuBahan = mmem
		menuRepo = mmem
		menuSPPG = mmem
		menuSekolah = mmem
		menuAudit = mmem
		menuNotifs = mmem
	}

	svc := application.New(application.Deps{
		Users:      userStore,
		Refresh:    refreshStore,
		Audits:     auditStore,
		SPPG:       sppgCheck,
		Sekolah:    sekolahCheck,
		Tokens:     issuer,
		Hasher:     hasher,
		AccessTTL:  cfg.AccessTTL,
		RefreshTTL: cfg.RefreshTTL,
	})

	handler := authhttp.NewHandler(svc)
	mw := authhttp.NewMiddleware(issuer, svc)

	menuSvc := menuapplication.New(menuapplication.Deps{
		Bahan: menuBahan, Menus: menuRepo, SPPG: menuSPPG, Sekolah: menuSekolah, Audits: menuAudit, Notifs: menuNotifs,
	})
	menuHandler := menuhttp.NewHandler(menuSvc)

	router := server.New(cfg, server.Deps{
		AuthHandler:    handler,
		AuthMiddleware: mw,
		MenuHandler:    menuHandler,
	})

	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
