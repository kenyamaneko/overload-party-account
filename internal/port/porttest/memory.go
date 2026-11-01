// Package porttest は port のインターフェースをメモリ上で実装した、テスト用の偽物を提供する。
package porttest

import (
	"maps"
	"time"

	"cloud.google.com/go/civil"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

const seedExp = 0

type dailyBattleKey struct {
	playerID string
	gameDate civil.Date
}

// Memory は、リポジトリの各 port が共有するメモリ上の保存内容である。
type Memory struct {
	players           map[string]domain.Player
	progressions      map[string]domain.PlayerProgression
	settings          map[string]domain.PlayerSettings
	factions          map[string]map[string]struct{}
	initialFactions   map[string]string
	dailyBattles      map[dailyBattleKey]int64
	revertedGames     map[string]struct{}
	processedEvents   map[string]string
	lastPremiumEvents map[string]time.Time
	gameConfig        map[string]int64

	playerCreateErr     error
	playerViewLookupErr error
	gameConfigLookupErr error

	playerViewLookupPanic any
}

// NewMemory は、ゲーム設定が gameConfig で、プレイヤーが1人も登録されていない Memory を返す。
func NewMemory(gameConfig map[string]int64) *Memory {
	config := make(map[string]int64, len(gameConfig))
	maps.Copy(config, gameConfig)
	return &Memory{
		players:           map[string]domain.Player{},
		progressions:      map[string]domain.PlayerProgression{},
		settings:          map[string]domain.PlayerSettings{},
		factions:          map[string]map[string]struct{}{},
		initialFactions:   map[string]string{},
		dailyBattles:      map[dailyBattleKey]int64{},
		revertedGames:     map[string]struct{}{},
		processedEvents:   map[string]string{},
		lastPremiumEvents: map[string]time.Time{},
		gameConfig:        config,
	}
}

func (m *Memory) clone() *Memory {
	factions := make(map[string]map[string]struct{}, len(m.factions))
	for playerID, owned := range m.factions {
		factions[playerID] = maps.Clone(owned)
	}
	return &Memory{
		players:             maps.Clone(m.players),
		progressions:        maps.Clone(m.progressions),
		settings:            maps.Clone(m.settings),
		factions:            factions,
		initialFactions:     maps.Clone(m.initialFactions),
		dailyBattles:        maps.Clone(m.dailyBattles),
		revertedGames:       maps.Clone(m.revertedGames),
		processedEvents:     maps.Clone(m.processedEvents),
		lastPremiumEvents:   maps.Clone(m.lastPremiumEvents),
		gameConfig:          m.gameConfig,
		playerCreateErr:     m.playerCreateErr,
		playerViewLookupErr: m.playerViewLookupErr,
		gameConfigLookupErr: m.gameConfigLookupErr,

		playerViewLookupPanic: m.playerViewLookupPanic,
	}
}

// PlayerSeed は、あらかじめ登録しておくプレイヤーの内容である。
type PlayerSeed struct {
	PlayerID         string
	FirebaseUID      string
	Name             *string
	Exp              int64
	OnboardingStatus string
	IsPremium        bool
	PremiumExpiresAt *time.Time
	RegisteredAt     time.Time
}

// NewPlayerSeed は、経験値 0・オンボーディング未開始のプレイヤーを registeredAt に登録した内容を返す。
func NewPlayerSeed(playerID, firebaseUID string, registeredAt time.Time) PlayerSeed {
	return PlayerSeed{
		PlayerID:         playerID,
		FirebaseUID:      firebaseUID,
		Exp:              seedExp,
		OnboardingStatus: domain.OnboardingStatusNotStarted,
		RegisteredAt:     registeredAt,
	}
}

// AddPlayer はプレイヤーとその累計経験値を登録する。
func (m *Memory) AddPlayer(seed PlayerSeed) {
	m.players[seed.PlayerID] = domain.Player{
		PlayerID:         seed.PlayerID,
		FirebaseUID:      seed.FirebaseUID,
		Name:             seed.Name,
		IsPremium:        seed.IsPremium,
		OnboardingStatus: seed.OnboardingStatus,
		PremiumExpiresAt: seed.PremiumExpiresAt,
		CreatedAt:        seed.RegisteredAt,
		UpdatedAt:        seed.RegisteredAt,
	}
	m.progressions[seed.PlayerID] = domain.PlayerProgression{
		PlayerID:  seed.PlayerID,
		Exp:       seed.Exp,
		UpdatedAt: seed.RegisteredAt,
	}
}

// AddSettings はプレイヤー設定を登録する。
func (m *Memory) AddSettings(settings domain.PlayerSettings) {
	m.settings[settings.PlayerID] = settings
}

// AddFaction は陣営をプレイヤーの所持陣営に加える。
func (m *Memory) AddFaction(playerID, faction string) {
	owned := m.factions[playerID]
	if owned == nil {
		owned = map[string]struct{}{}
		m.factions[playerID] = owned
	}
	owned[faction] = struct{}{}
}

// AddInitialFaction は陣営をプレイヤーの初期陣営にし、所持陣営にも加える。
func (m *Memory) AddInitialFaction(playerID, faction string) {
	m.initialFactions[playerID] = faction
	m.AddFaction(playerID, faction)
}

// SetProgression はプレイヤーの累計経験値を指定した値にする。
func (m *Memory) SetProgression(playerID string, exp int64) {
	m.progressions[playerID] = domain.PlayerProgression{PlayerID: playerID, Exp: exp}
}

// SetPremium はプレイヤーをプレミアム会員にする。
func (m *Memory) SetPremium(playerID string) {
	player := m.players[playerID]
	player.IsPremium = true
	m.players[playerID] = player
}

// SetDailyBattleCount はゲーム日のバトル回数を指定した値にする。
func (m *Memory) SetDailyBattleCount(playerID string, gameDate civil.Date, count int64) {
	m.dailyBattles[dailyBattleKey{playerID: playerID, gameDate: gameDate}] = count
}

// RemoveDailyBattle はゲーム日のバトル回数の記録を消す。
func (m *Memory) RemoveDailyBattle(playerID string, gameDate civil.Date) {
	delete(m.dailyBattles, dailyBattleKey{playerID: playerID, gameDate: gameDate})
}

// SetGameConfig はゲーム設定のキーを指定した値にする。
func (m *Memory) SetGameConfig(key string, value int64) {
	m.gameConfig[key] = value
}

// RemoveGameConfig はゲーム設定のキーを消す。
func (m *Memory) RemoveGameConfig(key string) {
	delete(m.gameConfig, key)
}

// FailPlayerCreateWith は、以降のプレイヤーの登録を err で失敗させる。
func (m *Memory) FailPlayerCreateWith(err error) {
	m.playerCreateErr = err
}

// FailPlayerViewLookupWith は、以降のプレイヤーの参照用の情報の取得を err で失敗させる。
func (m *Memory) FailPlayerViewLookupWith(err error) {
	m.playerViewLookupErr = err
}

// PanicOnPlayerViewLookupWith は、以降のプレイヤーの参照用の情報の取得を、値 value のパニックで異常終了させる。
func (m *Memory) PanicOnPlayerViewLookupWith(value any) {
	m.playerViewLookupPanic = value
}

// FailGameConfigLookupWith は、以降のゲーム設定の取得を err で失敗させる。
func (m *Memory) FailGameConfigLookupWith(err error) {
	m.gameConfigLookupErr = err
}

func (m *Memory) buildView(player domain.Player) *domain.PlayerView {
	progression := m.progressions[player.PlayerID]
	view := &domain.PlayerView{Player: player, Exp: progression.Exp}
	faction, hasInitialFaction := m.initialFactions[player.PlayerID]
	if hasInitialFaction {
		view.InitialFaction = &faction
	}
	return view
}
