package postgrestest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

const (
	countProcessedEventsQuery      = "SELECT count(*) FROM account.processed_events"
	countBattleCountReversalsQuery = "SELECT count(*) FROM account.battle_count_reversals"
)

// Repositories はケース専用のデータベースに接続した各リポジトリと、そのデータベースの接続プール。
type Repositories struct {
	Pool      *pgxpool.Pool
	Players   *postgres.PlayerRepository
	Views     *postgres.PlayerViewRepository
	Settings  *postgres.PlayerSettingsRepository
	Factions  *postgres.FactionRepository
	Events    *postgres.ProcessedEventRepository
	Reversals *postgres.BattleCountReversalRepository
	TxManager *postgres.TxManager
}

// NewRepositories はケース専用のデータベースを作り、それに接続した各リポジトリを返す。
func NewRepositories(t *testing.T, pg *Postgres) *Repositories {
	t.Helper()
	pool := pg.NewDatabase(t).Pool
	return &Repositories{
		Pool:      pool,
		Players:   postgres.NewPlayerRepository(pool),
		Views:     postgres.NewPlayerViewRepository(pool),
		Settings:  postgres.NewPlayerSettingsRepository(pool),
		Factions:  postgres.NewFactionRepository(pool),
		Events:    postgres.NewProcessedEventRepository(pool),
		Reversals: postgres.NewBattleCountReversalRepository(pool),
		TxManager: postgres.NewTxManager(pool),
	}
}

// RegisterPlayer はプレイヤーを、NewProgression の進行状況とあわせて登録する。
func (r *Repositories) RegisterPlayer(t *testing.T, player *domain.Player) {
	t.Helper()
	require.NoError(t, r.Players.Create(t.Context(), player, NewProgression(player.PlayerID)))
}

// RegisterThreeNamedPlayers は表示名が「テスト太郎」「テスト次郎」「テスト三郎」の 3 件のプレイヤーを登録する。
func (r *Repositories) RegisterThreeNamedPlayers(t *testing.T) {
	t.Helper()
	r.RegisterPlayer(t, NewPlayerWith(PlayerID1, FirebaseUID1, func(player *domain.Player) { player.Name = NewPointer("テスト太郎") }))
	r.RegisterPlayer(t, NewPlayerWith(PlayerID2, FirebaseUID2, func(player *domain.Player) { player.Name = NewPointer("テスト次郎") }))
	r.RegisterPlayer(t, NewPlayerWith(PlayerID3, FirebaseUID3, func(player *domain.Player) { player.Name = NewPointer("テスト三郎") }))
}

// RegisterPlayerWithProgression はプレイヤーを、指定した進行状況とあわせて登録する。
func (r *Repositories) RegisterPlayerWithProgression(t *testing.T, player *domain.Player, progression *domain.PlayerProgression) {
	t.Helper()
	require.NoError(t, r.Players.Create(t.Context(), player, progression))
}

// GetPlayer はプレイヤーIDでプレイヤーを取得する。
func (r *Repositories) GetPlayer(t *testing.T, playerID string) *domain.Player {
	t.Helper()
	player, err := r.Players.GetByID(t.Context(), playerID)
	require.NoError(t, err)
	return player
}

// GetProgression はトランザクションの中でプレイヤーIDの進行状況を排他取得する。
func (r *Repositories) GetProgression(t *testing.T, playerID string) *domain.PlayerProgression {
	t.Helper()
	progression, err := ReadInTx(t, r.TxManager, func(ctx context.Context) (*domain.PlayerProgression, error) {
		return r.Players.GetProgressionForUpdate(ctx, playerID)
	})
	require.NoError(t, err)
	return progression
}

// CountProcessedEvents はケース専用のデータベースに保存された処理済みイベントの件数を返す。
func (r *Repositories) CountProcessedEvents(t *testing.T) int {
	t.Helper()
	return r.countRows(t, countProcessedEventsQuery)
}

// CountBattleCountReversals はケース専用のデータベースに保存されたバトル回数返却の記録の件数を返す。
func (r *Repositories) CountBattleCountReversals(t *testing.T) int {
	t.Helper()
	return r.countRows(t, countBattleCountReversalsQuery)
}

func (r *Repositories) countRows(t *testing.T, query string) int {
	t.Helper()
	var count int
	require.NoError(t, r.Pool.QueryRow(t.Context(), query).Scan(&count))
	return count
}

// AddFaction は陣営を所持陣営に加える。
func (r *Repositories) AddFaction(t *testing.T, playerID, faction string) {
	t.Helper()
	require.NoError(t, r.Factions.AddPlayerFaction(t.Context(), playerID, faction))
}

// SetInitialFaction は陣営を初期陣営にする。
func (r *Repositories) SetInitialFaction(t *testing.T, playerID, faction string) {
	t.Helper()
	require.NoError(t, r.Factions.SetInitialFaction(t.Context(), playerID, faction))
}
