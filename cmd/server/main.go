package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/sync/errgroup"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging"
	"github.com/kenyamaneko/overload-party-account/internal/adapter/system"
	"github.com/kenyamaneko/overload-party-account/internal/config"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	accountfirestore "github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	setDefaultLogger(logging.NewCloudLoggingHandler(os.Stdout))
	if err := run(); err != nil {
		slog.Error("account fatal", "error", err)
		os.Exit(1)
	}
}

func setDefaultLogger(handler slog.Handler) {
	slog.SetDefault(slog.New(handler).With("service", "account"))
}

// setupLogger は LOG_MODE に応じてグローバル slog ロガーを初期化する。
func setupLogger(mode config.LogMode) error {
	switch mode {
	case config.LogModeProduction:
		setDefaultLogger(logging.NewCloudLoggingHandler(os.Stdout))
	case config.LogModeLocal:
		setDefaultLogger(logging.NewLocalHandler(os.Stderr))
	default:
		return fmt.Errorf("unexpected LOG_MODE: %s", mode)
	}
	return nil
}

func run() (err error) {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	if err := setupLogger(cfg.LogMode); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, closeDatabasePool, err := newDatabasePool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("new database pool: %w", err)
	}
	defer func() {
		if closeErr := closeDatabasePool(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close database pool: %w", closeErr))
		}
	}()

	firestoreClient, err := firestore.NewClient(ctx, cfg.GoogleCloudProjectID)
	if err != nil {
		return fmt.Errorf("firestore new client: %w", err)
	}
	defer func() {
		if closeErr := firestoreClient.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("firestore client close: %w", closeErr))
		}
	}()

	txManager := postgres.NewTxManager(pool)
	playerRepo := postgres.NewPlayerRepository(pool)
	playerViewRepo := postgres.NewPlayerViewRepository(pool)
	playerSettingsRepo := postgres.NewPlayerSettingsRepository(pool)
	gameConfigRepo := accountfirestore.New(firestoreClient)
	if err := usecase.ValidateGameConfig(ctx, gameConfigRepo); err != nil {
		return fmt.Errorf("validate game_config: %w", err)
	}
	factionRepo := postgres.NewFactionRepository(pool)
	eventRepo := postgres.NewProcessedEventRepository(pool)
	battleCountReversalRepo := postgres.NewBattleCountReversalRepository(pool)
	clock := system.NewSystemClock()
	playerIDGenerator := system.NewUUIDPlayerIDGenerator()

	authInteractor := usecase.NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, gameConfigRepo, txManager, clock, playerIDGenerator)
	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, battleCountReversalRepo, playerViewRepo, gameConfigRepo, txManager, clock)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, eventRepo, txManager)
	premiumInteractor := usecase.NewPremiumInteractor(playerRepo, eventRepo, txManager)
	onboardingInteractor := usecase.NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, txManager)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(playerSettingsRepo)

	factionAcquiredSubscriber := pubsubpush.NewFactionAcquiredSubscriber(factionInteractor)
	premiumUpdatedSubscriber := pubsubpush.NewPremiumUpdatedSubscriber(premiumInteractor)
	playerOnboardedSubscriber := pubsubpush.NewPlayerOnboardedSubscriber(onboardingInteractor)
	onboardingNameSetSubscriber := pubsubpush.NewOnboardingNameSetSubscriber(onboardingInteractor)
	onboardingFactionSetSubscriber := pubsubpush.NewOnboardingFactionSetSubscriber(onboardingInteractor)

	pubsubHandlers := pubsubpush.Handlers{
		FactionAcquired: pubsubpush.NewEventHandler(
			pubsubpush.FactionAcquiredSubscription, factionAcquiredSubscriber.ReceiveFactionAcquired),
		PremiumUpdated: pubsubpush.NewEventHandler(
			pubsubpush.PremiumUpdatedSubscription, premiumUpdatedSubscriber.ReceivePremiumUpdated),
		PlayerOnboarded: pubsubpush.NewEventHandler(
			pubsubpush.PlayerOnboardedSubscription, playerOnboardedSubscriber.ReceivePlayerOnboarded),
		OnboardingNameSet: pubsubpush.NewEventHandler(
			pubsubpush.OnboardingNameSetSubscription, onboardingNameSetSubscriber.ReceiveOnboardingNameSet),
		OnboardingFactionSet: pubsubpush.NewEventHandler(
			pubsubpush.OnboardingFactionSetSubscription, onboardingFactionSetSubscriber.ReceiveOnboardingFactionSet),
	}

	authHandler := rest.NewAuthHandler(authInteractor)
	playerHandler := rest.NewPlayerHandler(playerInteractor, premiumInteractor)
	factionHandler := rest.NewFactionHandler(factionInteractor)
	settingsHandler := rest.NewPlayerSettingsHandler(settingsInteractor)

	authVerifier := internalauth.NewVerifier(
		internalauth.StaticPublicKeyResolver(cfg.InternalAuthPublicKey, internalauth.DefaultKeyID),
	)

	r := router.New(authHandler, playerHandler, factionHandler, settingsHandler, authVerifier, pubsubHandlers)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	slog.Info("account starting",
		"addr", srv.Addr,
		"google_cloud_project", cfg.GoogleCloudProjectID,
	)

	return runHTTP(ctx, srv)
}

// runHTTP は HTTP サーバーを起動し、ctx が終了したら停止する。
func runHTTP(ctx context.Context, srv *http.Server) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("shutdown requested")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	})

	return g.Wait()
}
