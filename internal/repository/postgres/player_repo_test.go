//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	testPlayerID1 = "11111111-1111-1111-1111-111111111111"
	testPlayerID2 = "22222222-2222-2222-2222-222222222222"
)

func TestPlayerRepository_Create_Then_FindByID(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	now := time.Now().UTC()
	alice := "Alice"
	p := &apiaccount.Player{
		PlayerID:    testPlayerID1,
		FirebaseUID: "uid-1",
		Name:        &alice,
		Level:       1,
		Exp:         0,
		IsPremium:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	daily := &apiaccount.PlayerDailyBattle{
		PlayerID:         p.PlayerID,
		DailyBattleCount: 0,
		LastResetDate:    civil.DateOf(now),
	}
	prog := &apiaccount.PlayerProgression{
		PlayerID:  p.PlayerID,
		Level:     1,
		Exp:       0,
		UpdatedAt: now,
	}
	require.NoError(t, repo.Create(ctx, p, daily, prog))

	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Alice", *got.Name)
	assert.Equal(t, int64(1), got.Level)
	assert.Equal(t, int64(0), got.Exp)
}

func TestPlayerRepository_FindByID_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	_, err := repo.FindByID(ctx, testPlayerID2)
	assert.ErrorIs(t, err, port.ErrNotFound)
}

func TestPlayerRepository_FindByFirebaseUID_Seeded(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	got, err := repo.FindByFirebaseUID(ctx, "uid-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Alice", *got.Name)
}

func TestPlayerRepository_FindByFirebaseUID_NotFound_ReturnsNil(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	got, err := repo.FindByFirebaseUID(ctx, "uid-missing")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPlayerRepository_UpdateName(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	require.NoError(t, repo.UpdateName(ctx, testPlayerID1, "Bob"))

	// 永続化を確認
	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Bob", *got.Name)
}

func TestPlayerRepository_UpdateName_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	err := repo.UpdateName(ctx, testPlayerID1, "Bob")
	assert.ErrorIs(t, err, port.ErrNotFound)
}

func TestPlayerRepository_UpdatePremium(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	require.NoError(t, repo.UpdatePremium(ctx, testPlayerID1, true, &expiresAt))

	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.True(t, got.IsPremium)
	require.NotNil(t, got.PremiumExpiresAt)
	assert.WithinDuration(t, expiresAt, *got.PremiumExpiresAt, time.Second)
}

func TestPlayerRepository_UpdateFaction(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	require.NoError(t, repo.UpdateFaction(ctx, testPlayerID1, "SHE"))

	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got.SelectedFaction)
	assert.Equal(t, "SHE", *got.SelectedFaction)
}

// SetSelectedFactionIfNull は selected_faction が NULL のときだけ書き込む純プリミティブ。
// 「未更新の理由が既選択かプレイヤー不在か」の識別は repo の責務外なので、ここでは
// 「行が更新されたか」のみを検証する。プレイヤー不在ケースも 0 行扱い。
func TestPlayerRepository_SetSelectedFactionIfNull(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	noPreSelect := func(*testing.T) {}
	preSelectTenki := func(t *testing.T) {
		require.NoError(t, repo.UpdateFaction(ctx, testPlayerID1, "Tenki"))
	}
	skipSeed := func(*testing.T) {}

	tests := []struct {
		name       string
		seed       bool
		preSelect  func(*testing.T)
		wantSet    bool
		wantStored *string
	}{
		{
			name:       "selected_faction が NULL なら更新成立",
			seed:       true,
			preSelect:  noPreSelect,
			wantSet:    true,
			wantStored: ptrStr("SHE"),
		},
		{
			name:       "既に選択済みなら更新不成立で値は上書きされない",
			seed:       true,
			preSelect:  preSelectTenki,
			wantSet:    false,
			wantStored: ptrStr("Tenki"),
		},
		{
			name:       "プレイヤー不在も更新不成立 (NotFound 判定は service 層)",
			seed:       false,
			preSelect:  skipSeed,
			wantSet:    false,
			wantStored: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			if tt.seed {
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
				tt.preSelect(t)
			}

			set, err := repo.SetSelectedFactionIfNull(ctx, testPlayerID1, "SHE")
			require.NoError(t, err)
			assert.Equal(t, tt.wantSet, set)

			if tt.seed {
				got, err := repo.FindByID(ctx, testPlayerID1)
				require.NoError(t, err)
				require.NotNil(t, got.SelectedFaction)
				assert.Equal(t, *tt.wantStored, *got.SelectedFaction)
			}
		})
	}
}

// Exists は player_id 行の存在確認のみを行う純プリミティブ。
func TestPlayerRepository_Exists(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name string
		seed bool
		want bool
	}{
		{name: "シード済みなら true", seed: true, want: true},
		{name: "未シードなら false (エラーではない)", seed: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			if tt.seed {
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			}

			got, err := repo.Exists(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func ptrStr(s string) *string { return &s }

// UpdateDailyBattleCount は渡された値をそのまま書き込むプリミティブ。
// 日付リセットやインクリメントの判断は service 層の責務なのでここでは検証しない。
func TestPlayerRepository_UpdateDailyBattleCount(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	today := civil.DateOf(time.Now().UTC())

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	require.NoError(t, repo.UpdateDailyBattleCount(ctx, testPlayerID1, 7, today))

	got, err := repo.GetDailyBattle(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(7), got.DailyBattleCount)
	assert.Equal(t, today, got.LastResetDate)
}

func TestPlayerRepository_UpdateDailyBattleCount_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	err := repo.UpdateDailyBattleCount(ctx, testPlayerID1, 1, civil.DateOf(time.Now().UTC()))
	assert.ErrorIs(t, err, port.ErrNotFound)
}

func TestPlayerRepository_GetDailyBattle_Seeded(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	got, err := repo.GetDailyBattle(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(0), got.DailyBattleCount)
}

func TestPlayerRepository_GetDailyBattle_Unseeded_ReturnsNil(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)

	got, err := repo.GetDailyBattle(ctx, testPlayerID2)
	require.NoError(t, err)
	assert.Nil(t, got)
}

// GetProgressionForUpdate は RunInTx 配下で現在値を返す純プリミティブ。
// 加算・レベル計算は service 層の責務なので、repo テストは行取得とエラー伝播だけを検証する。
func TestPlayerRepository_GetProgressionForUpdate(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	txMgr := postgres.NewTxManager(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	require.NoError(t, txMgr.RunInTx(ctx, func(txCtx context.Context) error {
		prog, err := repo.GetProgressionForUpdate(txCtx, testPlayerID1)
		require.NoError(t, err)
		require.NotNil(t, prog)
		assert.Equal(t, testPlayerID1, prog.PlayerID)
		assert.Equal(t, int64(1), prog.Level)
		assert.Equal(t, int64(0), prog.Exp)
		return nil
	}))
}

func TestPlayerRepository_GetProgressionForUpdate_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	txMgr := postgres.NewTxManager(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)

	err := txMgr.RunInTx(ctx, func(txCtx context.Context) error {
		_, err := repo.GetProgressionForUpdate(txCtx, testPlayerID1)
		return err
	})
	assert.ErrorIs(t, err, port.ErrNotFound)
}

// UpdateProgression は受け取った exp / level をそのまま書き込むプリミティブ。
// FindByID 側にも JOIN 経由で反映されることを 1 度だけ担保する。
func TestPlayerRepository_UpdateProgression(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	prog, err := repo.UpdateProgression(ctx, testPlayerID1, 150, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(150), prog.Exp)
	assert.Equal(t, int64(2), prog.Level)

	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.Equal(t, int64(150), got.Exp)
	assert.Equal(t, int64(2), got.Level)
}

func TestPlayerRepository_UpdateProgression_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	_, err := repo.UpdateProgression(ctx, testPlayerID1, 10, 1)
	assert.ErrorIs(t, err, port.ErrNotFound)
}
