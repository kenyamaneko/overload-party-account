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
	prog := &apiaccount.PlayerProgression{
		PlayerID:  p.PlayerID,
		Level:     1,
		Exp:       0,
		UpdatedAt: now,
	}
	require.NoError(t, repo.Create(ctx, p, prog))

	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Alice", *got.Name)
	assert.Equal(t, int64(1), got.Level)
	assert.Equal(t, int64(0), got.Exp)
}

// FindByID の契約: シード済み player_id なら Player を返し、未シードなら ErrNotFound。
func TestPlayerRepository_FindByID(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		lookupID string
		wantName *string
		wantErr  error
	}{
		{
			name:     "シード済み player_id なら Player を返す",
			lookupID: testPlayerID1,
			wantName: ptr("Alice"),
			wantErr:  nil,
		},
		{
			name:     "未シード player_id なら ErrNotFound",
			lookupID: testPlayerID2,
			wantName: nil,
			wantErr:  port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			got, err := repo.FindByID(ctx, tt.lookupID)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantName, playerNamePtr(got))
		})
	}
}

// FindByFirebaseUID の契約: 一致する firebase_uid があれば Player を返し、
// 無ければ ErrNotFound。業務分岐 (Register の既登録検出など) は呼び出し側で errors.Is で行う。
func TestPlayerRepository_FindByFirebaseUID(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name      string
		lookupUID string
		wantName  *string
		wantErr   error
	}{
		{
			name:      "シード済み firebase_uid なら Player を返す",
			lookupUID: "uid-1",
			wantName:  ptr("Alice"),
			wantErr:   nil,
		},
		{
			name:      "未シード firebase_uid なら ErrNotFound",
			lookupUID: "uid-missing",
			wantName:  nil,
			wantErr:   port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			got, err := repo.FindByFirebaseUID(ctx, tt.lookupUID)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantName, playerNamePtr(got))
		})
	}
}

// Exists は player_id 行の存在確認のみを行う純プリミティブ。
func TestPlayerRepository_Exists(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		lookupID string
		want     bool
	}{
		{
			name:     "シード済みの player_id なら true",
			lookupID: testPlayerID1,
			want:     true,
		},
		{
			name:     "未シードの player_id なら false (該当なしはエラーではない)",
			lookupID: testPlayerID2,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			got, err := repo.Exists(ctx, tt.lookupID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// GetDailyBattle の契約: (player_id, game_date) に行があれば永続層の値をそのまま返し、
// 無ければ (nil, nil) (該当なしはエラーではなく、呼び出し側はカウント 0 として扱う)。
func TestPlayerRepository_GetDailyBattle(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()
	today := civil.DateOf(time.Now().UTC())

	tests := []struct {
		name      string
		seedCount []int64 // 0 件 = 未シード、1 件 = その count をシード
		want      *apiaccount.PlayerDailyBattle
	}{
		{
			name:      "該当行があれば永続層の値をそのまま返す",
			seedCount: []int64{5},
			want: &apiaccount.PlayerDailyBattle{
				PlayerID:         testPlayerID1,
				GameDate:         today,
				DailyBattleCount: 5,
			},
		},
		{
			name:      "該当行が無ければ (nil, nil) (エラーではない)",
			seedCount: nil,
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			for _, c := range tt.seedCount {
				seedPlayerDailyBattle(t, testPlayerID1, today, c)
			}

			got, err := repo.GetDailyBattle(ctx, testPlayerID1, today)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// GetProgressionForUpdate の契約: RunInTx 配下で現在値を返す純プリミティブ。
// シード済みなら現在値を返し、未シードなら ErrNotFound。FOR UPDATE による行ロック取得自体の
// 検証はせず、行取得経路とエラー伝播のみを repo テストの責務とする
// (加算・レベル計算は service 層の責務)。
func TestPlayerRepository_GetProgressionForUpdate(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	txMgr := postgres.NewTxManager(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name      string
		lookupID  string
		wantLevel int64
		wantExp   int64
		wantErr   error
	}{
		{
			name:      "シード済みなら現在値を返す",
			lookupID:  testPlayerID1,
			wantLevel: 1,
			wantExp:   0,
			wantErr:   nil,
		},
		{
			name:     "未シードなら ErrNotFound",
			lookupID: testPlayerID2,
			wantErr:  port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			err := txMgr.RunInTx(ctx, func(txCtx context.Context) error {
				prog, err := repo.GetProgressionForUpdate(txCtx, tt.lookupID)
				gotLevel, gotExp := progressionLevelExp(prog)
				assert.Equal(t, tt.wantLevel, gotLevel)
				assert.Equal(t, tt.wantExp, gotExp)
				return err
			})
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// UpdateName の契約: シード済みなら name を上書きし、未シードなら ErrNotFound。
// 永続化は FindByID 経由で確認する (シード済みケースのみ)。
func TestPlayerRepository_UpdateName(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		lookupID string
		wantName *string
		wantErr  error
	}{
		{
			name:     "シード済みなら name を更新",
			lookupID: testPlayerID1,
			wantName: ptr("Bob"),
			wantErr:  nil,
		},
		{
			name:     "未シードなら ErrNotFound",
			lookupID: testPlayerID2,
			wantName: nil,
			wantErr:  port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			err := repo.UpdateName(ctx, tt.lookupID, "Bob")
			require.ErrorIs(t, err, tt.wantErr)

			got, _ := repo.FindByID(ctx, tt.lookupID)
			assert.Equal(t, tt.wantName, playerNamePtr(got))
		})
	}
}

// UpdatePremium の契約: シード済みなら is_premium / premium_expires_at を更新し、
// 未シードなら ErrNotFound。永続化は FindByID 経由で確認する (シード済みケースのみ)。
func TestPlayerRepository_UpdatePremium(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	tests := []struct {
		name        string
		lookupID    string
		wantPremium bool
		wantErr     error
	}{
		{
			name:        "シード済みなら is_premium を更新",
			lookupID:    testPlayerID1,
			wantPremium: true,
			wantErr:     nil,
		},
		{
			name:        "未シードなら ErrNotFound",
			lookupID:    testPlayerID2,
			wantPremium: false,
			wantErr:     port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			err := repo.UpdatePremium(ctx, tt.lookupID, true, &expiresAt)
			require.ErrorIs(t, err, tt.wantErr)

			got, _ := repo.FindByID(ctx, tt.lookupID)
			assert.Equal(t, tt.wantPremium, playerIsPremium(got))
		})
	}
}

// IncrementDailyBattleCount は (player_id, game_date) を UPSERT する純プリミティブ。
// 「初回 INSERT で 1、2 回目以降 UPDATE で +1、別ゲーム日は別行として独立」を契約として
// 検証する。連続呼び出しの履歴を 1 ケースの calls スライスで表現することで、
// 「同じ日 → 加算」「別の日 → 1 から独立」「既存日に戻る → 履歴が独立して残る」の
// 一連の不変条件をテーブル駆動で表す。
func TestPlayerRepository_IncrementDailyBattleCount(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	today := civil.DateOf(time.Now().UTC())
	yesterday := today.AddDays(-1)

	type call struct {
		playerID string
		gameDate civil.Date
		want     int64
	}

	tests := []struct {
		name  string
		calls []call
	}{
		{
			name: "初回 INSERT で 1 が返る",
			calls: []call{
				{playerID: testPlayerID1, gameDate: today, want: 1},
			},
		},
		{
			name: "同一ゲーム日への 2 回目以降は +1 ずつ加算される",
			calls: []call{
				{playerID: testPlayerID1, gameDate: today, want: 1},
				{playerID: testPlayerID1, gameDate: today, want: 2},
				{playerID: testPlayerID1, gameDate: today, want: 3},
			},
		},
		{
			name: "別ゲーム日は独立した行として 1 から始まる",
			calls: []call{
				{playerID: testPlayerID1, gameDate: today, want: 1},
				{playerID: testPlayerID1, gameDate: yesterday, want: 1},
			},
		},
		{
			name: "既存日に戻っても履歴が独立して残り続けて加算される",
			calls: []call{
				{playerID: testPlayerID1, gameDate: today, want: 1},
				{playerID: testPlayerID1, gameDate: today, want: 2},
				{playerID: testPlayerID1, gameDate: yesterday, want: 1},
				{playerID: testPlayerID1, gameDate: today, want: 3},
			},
		},
		{
			name: "別プレイヤーのインクリメントは互いに独立しカウントを共有しない",
			calls: []call{
				{playerID: testPlayerID1, gameDate: today, want: 1},
				{playerID: testPlayerID2, gameDate: today, want: 1},
				{playerID: testPlayerID1, gameDate: today, want: 2},
				{playerID: testPlayerID2, gameDate: today, want: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayer(t, testPlayerID2, "uid-2", "Bob", false)

			for _, c := range tt.calls {
				got, err := repo.IncrementDailyBattleCount(ctx, c.playerID, c.gameDate)
				require.NoError(t, err)
				assert.Equal(t, c.want, got)
			}
		})
	}
}

// UpdateProgression の契約: シード済みなら受け取った exp / level をそのまま書き込み、
// 未シードなら ErrNotFound。永続化は FindByID 経由 (JOIN) で反映されることまで確認する。
func TestPlayerRepository_UpdateProgression(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name      string
		lookupID  string
		wantLevel int64
		wantExp   int64
		wantErr   error
	}{
		{
			name:      "シード済みなら exp / level をそのまま書き込む",
			lookupID:  testPlayerID1,
			wantLevel: 2,
			wantExp:   150,
			wantErr:   nil,
		},
		{
			name:     "未シードなら ErrNotFound",
			lookupID: testPlayerID2,
			wantErr:  port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			prog, err := repo.UpdateProgression(ctx, tt.lookupID, 150, 2)
			require.ErrorIs(t, err, tt.wantErr)
			gotLevel, gotExp := progressionLevelExp(prog)
			assert.Equal(t, tt.wantLevel, gotLevel)
			assert.Equal(t, tt.wantExp, gotExp)

			got, _ := repo.FindByID(ctx, tt.lookupID)
			persistedLevel, persistedExp := playerProgression(got)
			assert.Equal(t, tt.wantLevel, persistedLevel)
			assert.Equal(t, tt.wantExp, persistedExp)
		})
	}
}
