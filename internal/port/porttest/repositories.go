package porttest

import (
	"context"
	"maps"
	"slices"
	"time"

	"cloud.google.com/go/civil"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PlayerRepo は Memory を保存先とする port.PlayerRepo を返す。
func (m *Memory) PlayerRepo() port.PlayerRepo { return playerRepo{m} }

// PlayerViewRepo は Memory を保存先とする port.PlayerViewRepo を返す。
func (m *Memory) PlayerViewRepo() port.PlayerViewRepo { return playerViewRepo{m} }

// PlayerPremiumRepo は Memory を保存先とする port.PlayerPremiumRepo を返す。
func (m *Memory) PlayerPremiumRepo() port.PlayerPremiumRepo { return playerPremiumRepo{m} }

// PlayerOnboardingRepo は Memory を保存先とする port.PlayerOnboardingRepo を返す。
func (m *Memory) PlayerOnboardingRepo() port.PlayerOnboardingRepo { return playerOnboardingRepo{m} }

// PlayerProgressionRepo は Memory を保存先とする port.PlayerProgressionRepo を返す。
func (m *Memory) PlayerProgressionRepo() port.PlayerProgressionRepo {
	return playerProgressionRepo{m}
}

// PlayerBattleRepo は Memory を保存先とする port.PlayerBattleRepo を返す。
func (m *Memory) PlayerBattleRepo() port.PlayerBattleRepo { return playerBattleRepo{m} }

// BattleCountReversalRepo は Memory を保存先とする port.BattleCountReversalRepo を返す。
func (m *Memory) BattleCountReversalRepo() port.BattleCountReversalRepo {
	return battleCountReversalRepo{m}
}

// PlayerSettingsRepo は Memory を保存先とする port.PlayerSettingsRepo を返す。
func (m *Memory) PlayerSettingsRepo() port.PlayerSettingsRepo { return playerSettingsRepo{m} }

// GameConfigRepo は Memory を参照する port.GameConfigRepo を返す。
func (m *Memory) GameConfigRepo() port.GameConfigRepo { return gameConfigRepo{m} }

// FactionRepo は Memory を保存先とする port.FactionRepo を返す。
func (m *Memory) FactionRepo() port.FactionRepo { return factionRepo{m} }

// ProcessedEventRepo は Memory を保存先とする port.ProcessedEventRepo を返す。
func (m *Memory) ProcessedEventRepo() port.ProcessedEventRepo { return processedEventRepo{m} }

// TxRunner は、処理が失敗したときに Memory の保存内容を処理の前へ戻す port.TxRunner を返す。
func (m *Memory) TxRunner() port.TxRunner { return txRunner{m} }

type playerRepo struct{ m *Memory }

func (r playerRepo) Create(_ context.Context, player *domain.Player, progression *domain.PlayerProgression) error {
	if r.m.playerCreateErr != nil {
		return r.m.playerCreateErr
	}
	_, isIDTaken := r.m.players[player.PlayerID]
	isFirebaseUIDTaken := slices.ContainsFunc(slices.Collect(maps.Values(r.m.players)), func(existing domain.Player) bool {
		return existing.FirebaseUID == player.FirebaseUID
	})
	if isIDTaken || isFirebaseUIDTaken {
		return port.ErrAlreadyExists
	}
	r.m.players[player.PlayerID] = *player
	r.m.progressions[player.PlayerID] = *progression
	return nil
}

func (r playerRepo) GetByID(_ context.Context, playerID string) (*domain.Player, error) {
	player, isFound := r.m.players[playerID]
	if !isFound {
		return nil, port.ErrNotFound
	}
	return &player, nil
}

func (r playerRepo) Exists(_ context.Context, playerID string) (bool, error) {
	_, isFound := r.m.players[playerID]
	return isFound, nil
}

func (r playerRepo) UpdateName(_ context.Context, playerID string, name string) error {
	player, isFound := r.m.players[playerID]
	if !isFound {
		return port.ErrNotFound
	}
	player.Name = &name
	r.m.players[playerID] = player
	return nil
}

type playerViewRepo struct{ m *Memory }

func (r playerViewRepo) GetByID(_ context.Context, playerID string) (*domain.PlayerView, error) {
	if r.m.playerViewLookupPanic != nil {
		panic(r.m.playerViewLookupPanic)
	}
	if r.m.playerViewLookupErr != nil {
		return nil, r.m.playerViewLookupErr
	}
	player, isFound := r.m.players[playerID]
	if !isFound {
		return nil, port.ErrNotFound
	}
	return r.m.buildView(player), nil
}

func (r playerViewRepo) GetByFirebaseUID(_ context.Context, firebaseUID string) (*domain.PlayerView, error) {
	if r.m.playerViewLookupPanic != nil {
		panic(r.m.playerViewLookupPanic)
	}
	if r.m.playerViewLookupErr != nil {
		return nil, r.m.playerViewLookupErr
	}
	for _, player := range r.m.players {
		if player.FirebaseUID == firebaseUID {
			return r.m.buildView(player), nil
		}
	}
	return nil, port.ErrNotFound
}

type playerPremiumRepo struct{ m *Memory }

func (r playerPremiumRepo) UpdatePremium(_ context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	player, isFound := r.m.players[playerID]
	if !isFound {
		return port.ErrNotFound
	}
	player.IsPremium = isPremium
	player.PremiumExpiresAt = expiresAt
	r.m.players[playerID] = player
	return nil
}

func (r playerPremiumRepo) UpdatePremiumFromEvent(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time, occurredAt time.Time) error {
	if err := r.UpdatePremium(ctx, playerID, isPremium, expiresAt); err != nil {
		return err
	}
	r.m.lastPremiumEvents[playerID] = occurredAt
	return nil
}

func (r playerPremiumRepo) GetLastPremiumEventOccurredAtForUpdate(_ context.Context, playerID string) (*time.Time, error) {
	if _, isFound := r.m.players[playerID]; !isFound {
		return nil, port.ErrNotFound
	}
	occurredAt, hasEvent := r.m.lastPremiumEvents[playerID]
	if !hasEvent {
		return nil, nil
	}
	return &occurredAt, nil
}

type playerOnboardingRepo struct{ m *Memory }

func (r playerOnboardingRepo) GetOnboardingStatus(_ context.Context, playerID string) (string, error) {
	player, isFound := r.m.players[playerID]
	if !isFound {
		return "", port.ErrNotFound
	}
	return player.OnboardingStatus, nil
}

func (r playerOnboardingRepo) UpdateOnboardingStatus(_ context.Context, playerID, status string) error {
	player, isFound := r.m.players[playerID]
	if !isFound {
		return port.ErrNotFound
	}
	player.OnboardingStatus = status
	r.m.players[playerID] = player
	return nil
}

type playerProgressionRepo struct{ m *Memory }

func (r playerProgressionRepo) GetProgressionForUpdate(_ context.Context, playerID string) (*domain.PlayerProgression, error) {
	progression, isFound := r.m.progressions[playerID]
	if !isFound {
		return nil, port.ErrNotFound
	}
	return &progression, nil
}

func (r playerProgressionRepo) UpdateProgression(_ context.Context, playerID string, exp int64) error {
	progression, isFound := r.m.progressions[playerID]
	if !isFound {
		return port.ErrNotFound
	}
	progression.Exp = exp
	r.m.progressions[playerID] = progression
	return nil
}

type playerBattleRepo struct{ m *Memory }

func (r playerBattleRepo) GetDailyBattle(_ context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error) {
	count, isFound := r.m.dailyBattles[dailyBattleKey{playerID: playerID, gameDate: gameDate}]
	if !isFound {
		return nil, nil
	}
	return &domain.PlayerDailyBattle{PlayerID: playerID, GameDate: gameDate, DailyBattleCount: count}, nil
}

func (r playerBattleRepo) GetDailyBattleForUpdate(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error) {
	return r.GetDailyBattle(ctx, playerID, gameDate)
}

func (r playerBattleRepo) IncrementDailyBattleCount(_ context.Context, playerID string, gameDate civil.Date) (int64, error) {
	key := dailyBattleKey{playerID: playerID, gameDate: gameDate}
	r.m.dailyBattles[key]++
	return r.m.dailyBattles[key], nil
}

func (r playerBattleRepo) DecrementDailyBattleCount(_ context.Context, playerID string, gameDate civil.Date) error {
	key := dailyBattleKey{playerID: playerID, gameDate: gameDate}
	if _, isFound := r.m.dailyBattles[key]; !isFound {
		return port.ErrNotFound
	}
	r.m.dailyBattles[key]--
	return nil
}

type battleCountReversalRepo struct{ m *Memory }

func (r battleCountReversalRepo) MarkReverted(_ context.Context, gameID string) (bool, error) {
	_, isAlreadyReverted := r.m.revertedGames[gameID]
	r.m.revertedGames[gameID] = struct{}{}
	return !isAlreadyReverted, nil
}

type playerSettingsRepo struct{ m *Memory }

func (r playerSettingsRepo) InsertPlayerSettings(_ context.Context, settings *domain.PlayerSettings) error {
	r.m.settings[settings.PlayerID] = *settings
	return nil
}

func (r playerSettingsRepo) GetPlayerSettings(_ context.Context, playerID string) (*domain.PlayerSettings, error) {
	settings, isFound := r.m.settings[playerID]
	if !isFound {
		return nil, port.ErrNotFound
	}
	return &settings, nil
}

func (r playerSettingsRepo) UpdatePlayerSettings(_ context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	settings, isFound := r.m.settings[playerID]
	if !isFound {
		return port.ErrNotFound
	}
	if patch.Language != nil {
		settings.Language = *patch.Language
	}
	if patch.BgmVolume != nil {
		settings.BgmVolume = *patch.BgmVolume
	}
	if patch.SeVolume != nil {
		settings.SeVolume = *patch.SeVolume
	}
	if patch.IsPushEnabled != nil {
		settings.IsPushEnabled = *patch.IsPushEnabled
	}
	r.m.settings[playerID] = settings
	return nil
}

type gameConfigRepo struct{ m *Memory }

func (r gameConfigRepo) GetInt64(_ context.Context, key string) (int64, error) {
	if r.m.gameConfigLookupErr != nil {
		return 0, r.m.gameConfigLookupErr
	}
	value, isFound := r.m.gameConfig[key]
	if !isFound {
		return 0, port.ErrNotFound
	}
	return value, nil
}

type factionRepo struct{ m *Memory }

func (r factionRepo) AddPlayerFaction(_ context.Context, playerID, faction string) error {
	r.m.AddFaction(playerID, faction)
	return nil
}

func (r factionRepo) GetPlayerFactions(_ context.Context, playerID string) ([]string, error) {
	return slices.AppendSeq(make([]string, 0), maps.Keys(r.m.factions[playerID])), nil
}

func (r factionRepo) GetInitialFaction(_ context.Context, playerID string) (*string, error) {
	faction, isFound := r.m.initialFactions[playerID]
	if !isFound {
		return nil, nil
	}
	return &faction, nil
}

func (r factionRepo) SetInitialFaction(_ context.Context, playerID, faction string) error {
	if _, hasInitialFaction := r.m.initialFactions[playerID]; hasInitialFaction {
		return port.ErrAlreadyExists
	}
	r.m.AddInitialFaction(playerID, faction)
	return nil
}

type processedEventRepo struct{ m *Memory }

func (r processedEventRepo) InsertProcessedEvent(_ context.Context, eventID, eventType string) (bool, error) {
	if _, isAlreadyProcessed := r.m.processedEvents[eventID]; isAlreadyProcessed {
		return false, nil
	}
	r.m.processedEvents[eventID] = eventType
	return true, nil
}

type txRunner struct{ m *Memory }

func (r txRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	snapshot := r.m.clone()
	err := fn(ctx)
	if err != nil {
		*r.m = *snapshot
	}
	return err
}
