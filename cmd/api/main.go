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
	berkasapplication "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/application"
	berkasinfrastructure "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/infrastructure"
	berkashttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/berkas/transport/http"
	menuapplication "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/application"
	menuinfrastructure "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/infrastructure"
	menuhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/menu/transport/http"
	pengadaanapplication "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/application"
	pengadaaninfrastructure "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/infrastructure"
	pengadaanhttp "github.com/srgjo27/sigap-sppg-be-monolithic/internal/modules/pengadaan/transport/http"
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

		pengadaanPemasok pengadaanapplication.PemasokRepository
		pengadaanBahan   pengadaanapplication.BahanRepository
		pengadaanMenus   pengadaanapplication.MenuRepository
		pengadaanSiklus  pengadaanapplication.SiklusRepository
		pengadaanPO      pengadaanapplication.PurchaseOrderRepository
		pengadaanSPPG    pengadaanapplication.SPPGProvider
		pengadaanAudit   pengadaanapplication.AuditRepository
		pengadaanNotifs  pengadaanapplication.NotificationRepository

		berkasStore berkasapplication.ObjectStore
		berkasRepo  berkasapplication.BerkasRepository
		berkasSPPG  berkasapplication.SPPGCodeProvider
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
		pengadaanPemasok = pengadaaninfrastructure.NewPemasokPostgres(pool)
		pengadaanBahan = pengadaaninfrastructure.NewBahanPostgres(pool)
		pengadaanMenus = pengadaaninfrastructure.NewMenuPostgres(pool)
		pengadaanSiklus = pengadaaninfrastructure.NewSiklusPostgres(pool)
		pengadaanPO = pengadaaninfrastructure.NewPurchaseOrderPostgres(pool)
		pengadaanSPPG = pengadaaninfrastructure.NewSPPGPostgres(pool)
		pengadaanAudit = pengadaaninfrastructure.NewAuditPostgres(pool)
		pengadaanNotifs = pengadaaninfrastructure.NewNotificationPostgres(pool)
		// Berkas (upload foto MVP-003.7) butuh database + object storage.
		berkasRepo = berkasinfrastructure.NewBerkasPostgres(pool)
		berkasSPPG = berkasinfrastructure.NewSPPGPostgres(pool)
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

	pengadaanSvc := pengadaanapplication.New(pengadaanapplication.Deps{
		Pemasok: pengadaanPemasok, Bahan: pengadaanBahan, Menus: pengadaanMenus, Siklus: pengadaanSiklus, PO: pengadaanPO,
		SPPG: pengadaanSPPG, Audits: pengadaanAudit, Notifs: pengadaanNotifs,
	})
	pengadaanHandler := pengadaanhttp.NewHandler(pengadaanSvc)

	deps := server.Deps{
		AuthHandler:      handler,
		AuthMiddleware:   mw,
		MenuHandler:      menuHandler,
		PengadaanHandler: pengadaanHandler,
	}

	// Berkas upload aktif hanya bila object storage ada
	// (STORAGE_ENDPOINT, STORAGE_BUCKET, STORAGE_ACCESS_KEY,
	// STORAGE_SECRET_KEY) dan database tersambung.
	if cfg.Storage.Enabled && berkasRepo != nil {
		store, err := berkasinfrastructure.NewMinioObjectStore(
			cfg.Storage.Endpoint, cfg.Storage.AccessKey, cfg.Storage.SecretKey,
			cfg.Storage.Region, cfg.Storage.UseTLS, cfg.Storage.Bucket,
		)
		if err != nil {
			log.Fatalf("init object storage: %v", err)
		}
		berkasStore = store
		berkasSvc := berkasapplication.New(berkasapplication.Deps{
			Store: berkasStore, Repo: berkasRepo, SPPG: berkasSPPG,
		})
		deps.BerkasHandler = berkashttp.NewHandler(berkasSvc)

		// Bersihkan berkas tiap jam (berkas yang tidak
		// dipakai lebih dari 24 jam dihapus dari object storage + tabel).
		purgeCtx := context.Background()
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-purgeCtx.Done():
					return
				case <-ticker.C:
					if err := berkasSvc.PurgeUnused(purgeCtx, 24*time.Hour); err != nil {
						log.Printf("warning: berkas purge: %v", err)
					}
				}
			}
		}()
	} else {
		if cfg.Storage.Enabled && berkasRepo == nil {
			log.Println("warning: STORAGE_* di-set tapi DATABASE_URL kosong; /uploads dinonaktifkan")
		}
	}

	router := server.New(cfg, deps)

	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
