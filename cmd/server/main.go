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

	pubsubadapter "github.com/kenyamaneko/overload-party-account/internal/adapter/pubsub"
	"github.com/kenyamaneko/overload-party-account/internal/config"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	accountfirestore "github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

func main() {
	if err := run(); err != nil {
		slog.Error("account fatal", "error", err)
		os.Exit(1)
	}
}

// setupLogger は LOG_MODE に応じてグローバル slog ロガーを初期化する。
func setupLogger(mode config.LogMode) error {
	switch mode {
	case config.LogModeProduction:
		slog.SetDefault(slog.New(newCloudLoggingHandler()).With("service", "account"))
	case config.LogModeLocal:
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		slog.SetDefault(slog.New(h).With("service", "account"))
	default:
		return fmt.Errorf("unexpected LOG_MODE: %s", mode)
	}
	return nil
}

// newCloudLoggingHandler は Cloud Logging 互換の JSON ハンドラを返す。
// slog のデフォルトフィールド名 (level/msg) は Cloud Logging が認識しないので severity/message に変換する。
func newCloudLoggingHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				a.Key = "severity"
				if level, ok := a.Value.Any().(slog.Level); ok {
					switch {
					case level >= slog.LevelError:
						a.Value = slog.StringValue("ERROR")
					case level >= slog.LevelWarn:
						a.Value = slog.StringValue("WARNING")
					case level >= slog.LevelInfo:
						a.Value = slog.StringValue("INFO")
					default:
						a.Value = slog.StringValue("DEBUG")
					}
				}
			}
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	})
}

func run() error {
	cfg, err := config.FromEnv()
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
	defer closeDatabasePool()
	defer pool.Close()

	fsClient, err := firestore.NewClient(ctx, cfg.GoogleCloudProjectID)
	if err != nil {
		return fmt.Errorf("firestore new client: %w", err)
	}
	defer func() {
		if cerr := fsClient.Close(); cerr != nil {
			slog.Warn("firestore client close", "error", cerr)
		}
	}()

	txManager := postgres.NewTxManager(pool)
	playerRepo := postgres.NewPlayerRepository(pool)
	playerViewRepo := postgres.NewPlayerViewRepository(pool)
	playerSettingsRepo := postgres.NewPlayerSettingsRepository(pool)
	gameConfigRepo := accountfirestore.NewGameConfigRepository(fsClient)
	if err := usecase.ValidateGameConfig(ctx, gameConfigRepo); err != nil {
		return fmt.Errorf("validate game_config: %w", err)
	}
	factionRepo := postgres.NewFactionRepository(pool)
	eventRepo := postgres.NewProcessedEventRepository(pool)

	// playerRepo (*postgres.PlayerRepository) は責務別 interface
	// (PlayerRepo / PlayerPremiumRepo / PlayerOnboardingRepo / PlayerProgressionRepo / PlayerBattleRepo)
	// すべてを暗黙的に満たすため、同じインスタンスを複数引数に渡せる (Go の structural typing)。
	authInteractor := usecase.NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, gameConfigRepo, txManager)
	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, playerViewRepo, gameConfigRepo, txManager)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, txManager)
	onboardingInteractor := usecase.NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, txManager)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(playerSettingsRepo)

	factionSub := pubsubadapter.NewFactionAcquiredSubscriber(factionRepo, txManager, eventRepo)
	premiumSub := pubsubadapter.NewPremiumUpdatedSubscriber(playerRepo, txManager, eventRepo)
	onboardedSub := pubsubadapter.NewPlayerOnboardedSubscriber(onboardingInteractor)
	nameSetSub := pubsubadapter.NewOnboardingNameSetSubscriber(onboardingInteractor)
	factionSetSub := pubsubadapter.NewOnboardingFactionSetSubscriber(onboardingInteractor)

	pubsubHandlers := pubsubpush.Handlers{
		FactionAcquired:      pubsubpush.NewEventHandler(factionSub.HandleMessage),
		PremiumUpdated:       pubsubpush.NewEventHandler(premiumSub.HandleMessage),
		PlayerOnboarded:      pubsubpush.NewEventHandler(onboardedSub.HandleMessage),
		OnboardingNameSet:    pubsubpush.NewEventHandler(nameSetSub.HandleMessage),
		OnboardingFactionSet: pubsubpush.NewEventHandler(factionSetSub.HandleMessage),
	}

	authH := rest.NewAuthHandler(authInteractor)
	playerH := rest.NewPlayerHandler(playerInteractor)
	factionH := rest.NewFactionHandler(factionInteractor)
	settingsH := rest.NewPlayerSettingsHandler(settingsInteractor)

	authVerifier := internalauth.NewVerifier(
		internalauth.StaticHS256Resolver([]byte(cfg.InternalAuthSecret), internalauth.DefaultKeyID),
	)

	r := router.New(authH, playerH, factionH, settingsH, authVerifier, pubsubHandlers)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("account starting",
		"addr", srv.Addr,
		"google_cloud_project", cfg.GoogleCloudProjectID,
	)

	return runHTTP(ctx, srv)
}

// runHTTP は HTTP server を起動し、シグナル到来で graceful に停止する。
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
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	})

	return g.Wait()
}
