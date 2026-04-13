package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/config"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/repository"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/service"
	pubsubadapter "github.com/kenyamaneko/overload-party-account/internal/adapter/pubsub"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("account: %v", err)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool new: %w", err)
	}
	defer pool.Close()

	fsClient, err := firestore.NewClient(ctx, cfg.FirestoreProjectID)
	if err != nil {
		return fmt.Errorf("firestore new client: %w", err)
	}
	defer func() { _ = fsClient.Close() }()

	txManager := repository.NewTxManager(pool)
	playerRepo := repository.NewPgPlayerRepository(pool)
	userSettingsRepo := repository.NewPgUserSettingsRepository(pool)
	gameConfigRepo := repository.NewFirestoreGameConfigRepository(fsClient)
	factionRepo := repository.NewPgFactionRepository(pool)
	eventRepo := repository.NewPgProcessedEventRepository(pool)

	authSvc := service.NewAuthService(playerRepo, userSettingsRepo, txManager)
	playerSvc := service.NewPlayerService(playerRepo, gameConfigRepo, factionRepo)
	factionSvc := service.NewFactionService(playerRepo, factionRepo, txManager)
	settingsSvc := service.NewUserSettingsService(userSettingsRepo)

	authH := rest.NewAuthHandler(authSvc)
	playerH := rest.NewPlayerHandler(playerSvc)
	factionH := rest.NewFactionHandler(factionSvc)
	settingsH := rest.NewUserSettingsHandler(settingsSvc)

	r := router.New(authH, playerH, factionH, settingsH)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	factionSub, err := pubsubadapter.NewFactionSelectedSubscriber(
		ctx, cfg.PubsubProjectID, cfg.FactionSelectedSubscription,
		playerRepo, factionRepo, txManager, eventRepo,
	)
	if err != nil {
		return fmt.Errorf("faction-selected subscriber: %w", err)
	}
	defer func() { _ = factionSub.Close() }()

	premiumSub, err := pubsubadapter.NewPremiumUpdatedSubscriber(
		ctx, cfg.PubsubProjectID, cfg.PremiumUpdatedSubscription,
		playerRepo, txManager, eventRepo,
	)
	if err != nil {
		return fmt.Errorf("premium-updated subscriber: %w", err)
	}
	defer func() { _ = premiumSub.Close() }()

	go func() {
		if err := factionSub.Start(ctx); err != nil && ctx.Err() == nil {
			log.Fatalf("faction-selected subscriber error: %v", err)
		}
	}()
	go func() {
		if err := premiumSub.Start(ctx); err != nil && ctx.Err() == nil {
			log.Fatalf("premium-updated subscriber error: %v", err)
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("account: listening on %s (pubsub project=%s)", srv.Addr, cfg.PubsubProjectID)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("account: shutdown requested")
	case err := <-errCh:
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	return nil
}
