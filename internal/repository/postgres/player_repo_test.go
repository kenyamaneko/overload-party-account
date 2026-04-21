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
	p := &apiaccount.Player{
		PlayerID:    testPlayerID1,
		FirebaseUID: "uid-1",
		Username:    "Alice",
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
	require.NoError(t, repo.Create(ctx, p, daily))

	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Username)
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
	assert.Equal(t, "Alice", got.Username)
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

func TestPlayerRepository_UpdateUsername(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	updated, err := repo.UpdateUsername(ctx, testPlayerID1, "Bob")
	require.NoError(t, err)
	assert.Equal(t, "Bob", updated.Username)

	// 永続化を確認
	got, err := repo.FindByID(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.Equal(t, "Bob", got.Username)
}

func TestPlayerRepository_UpdateUsername_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	_, err := repo.UpdateUsername(ctx, testPlayerID1, "Bob")
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

func TestPlayerRepository_IncrementDailyBattle(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	today := civil.DateOf(time.Now().UTC())
	yesterday := civil.Date{Year: today.Year, Month: today.Month, Day: today.Day - 1}

	tests := []struct {
		name      string
		seedCount int64
		seedDate  civil.Date
		today     civil.Date
		wantCount int64
	}{
		{
			name:      "同日ならインクリメント",
			seedCount: 5,
			seedDate:  today,
			today:     today,
			wantCount: 6,
		},
		{
			name:      "日付変われば 1 にリセット",
			seedCount: 9,
			seedDate:  yesterday,
			today:     today,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			// seedPlayer は LastResetDate=today でシードするので、ケースごとに上書き。
			_, err := sharedPg.Pool.Exec(ctx,
				`UPDATE account.player_daily_battle SET daily_battle_count = $1, last_reset_date = $2 WHERE player_id = $3`,
				tt.seedCount,
				time.Date(tt.seedDate.Year, tt.seedDate.Month, tt.seedDate.Day, 0, 0, 0, 0, time.UTC),
				testPlayerID1,
			)
			require.NoError(t, err)

			got, err := repo.IncrementDailyBattle(ctx, testPlayerID1, tt.today)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, got)
		})
	}
}

func TestPlayerRepository_IncrementDailyBattle_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	_, err := repo.IncrementDailyBattle(ctx, testPlayerID1, civil.DateOf(time.Now().UTC()))
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

func TestPlayerRepository_AddExp(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	// computeLevel: exp>=100 なら level=2、それ以上は 1 増やすだけの単純な関数。
	compute := func(newExp, curLevel int64) int64 {
		if newExp >= 100 {
			return 2
		}
		return curLevel
	}

	p, err := repo.AddExp(ctx, testPlayerID1, 150, compute)
	require.NoError(t, err)
	assert.Equal(t, int64(150), p.Exp)
	assert.Equal(t, int64(2), p.Level)
}

func TestPlayerRepository_AddExp_NotFound(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	_, err := repo.AddExp(ctx, testPlayerID1, 10, func(newExp, cur int64) int64 { return cur })
	assert.ErrorIs(t, err, port.ErrNotFound)
}
