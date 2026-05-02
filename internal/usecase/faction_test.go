//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

func newFactionTestInteractor(t *testing.T, playerID string) *FactionInteractor {
	t.Helper()
	sharedPg.Truncate(t)
	seedPlayer(t, playerID, "uid-"+playerID, "tester", false)

	playerRepo, _, factionRepo, _, tx := newRealRepos()
	return NewFactionInteractor(playerRepo, factionRepo, tx)
}

func TestSelectInitialFaction(t *testing.T) {
	ctx := context.Background()
	s := func(v string) *string { return &v }

	noErr := func(t *testing.T, err error) { t.Helper(); require.NoError(t, err) }
	anyErr := func(t *testing.T, err error) { t.Helper(); require.Error(t, err) }
	errIs := func(target error) func(*testing.T, error) {
		return func(t *testing.T, err error) { t.Helper(); require.ErrorIs(t, err, target) }
	}

	const unknownPID = "99999999-9999-9999-9999-999999999999"

	tests := []struct {
		name        string
		seedShop    []string // AddPlayerFaction で testPlayerID1 に事前追加
		preSelect   []string // SelectInitialFaction を testPlayerID1 で事前実行 (順次)
		playerID    string   // テスト対象呼び出しの playerID
		faction     string
		assertErr   func(*testing.T, error)
		wantOwned   []string
		wantInitial *string
	}{
		{
			name:        "成功: SHE を初回選択",
			playerID:    testPlayerID1,
			faction:     "SHE",
			assertErr:   noErr,
			wantOwned:   []string{"SHE"},
			wantInitial: s("SHE"),
		},
		{
			name:        "成功: Tenki を初回選択",
			playerID:    testPlayerID1,
			faction:     "Tenki",
			assertErr:   noErr,
			wantOwned:   []string{"Tenki"},
			wantInitial: s("Tenki"),
		},
		{
			name:        "成功: ショップ所持と別 faction を initial 選択",
			seedShop:    []string{"SHE"},
			playerID:    testPlayerID1,
			faction:     "Tenki",
			assertErr:   noErr,
			wantOwned:   []string{"SHE", "Tenki"},
			wantInitial: s("Tenki"),
		},
		{
			name:        "失敗: ショップ所持と同一 faction の initial 選択は PK 重複",
			seedShop:    []string{"SHE"},
			playerID:    testPlayerID1,
			faction:     "SHE",
			assertErr:   anyErr,
			wantOwned:   []string{"SHE"},
			wantInitial: nil,
		},
		{
			name:        "失敗: 既に initial 選択済みなら ErrFactionAlreadySelected",
			preSelect:   []string{"Tenki"},
			playerID:    testPlayerID1,
			faction:     "Tenki",
			assertErr:   errIs(ErrFactionAlreadySelected),
			wantOwned:   []string{"Tenki"},
			wantInitial: s("Tenki"),
		},
		{
			name:        "失敗: Neutral は選択不可",
			playerID:    testPlayerID1,
			faction:     "Neutral",
			assertErr:   errIs(ErrInvalidFaction),
			wantOwned:   []string{},
			wantInitial: nil,
		},
		{
			name:        "失敗: 未知ファクション",
			playerID:    testPlayerID1,
			faction:     "bogus",
			assertErr:   errIs(ErrInvalidFaction),
			wantOwned:   []string{},
			wantInitial: nil,
		},
		{
			name:        "失敗: 空ファクション",
			playerID:    testPlayerID1,
			faction:     "",
			assertErr:   errIs(ErrInvalidFaction),
			wantOwned:   []string{},
			wantInitial: nil,
		},
		{
			name:        "失敗: 空 playerID は ErrInvalidFaction",
			playerID:    "",
			faction:     "SHE",
			assertErr:   errIs(ErrInvalidFaction),
			wantOwned:   []string{},
			wantInitial: nil,
		},
		{
			name:        "失敗: 存在しない playerID は ErrNotFound",
			playerID:    unknownPID,
			faction:     "SHE",
			assertErr:   errIs(port.ErrNotFound),
			wantOwned:   []string{},
			wantInitial: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFactionTestInteractor(t, testPlayerID1)
			_, _, factionRepo, _, _ := newRealRepos()

			for _, f := range tt.seedShop {
				require.NoError(t, factionRepo.AddPlayerFaction(ctx, testPlayerID1, f))
			}
			for _, f := range tt.preSelect {
				require.NoError(t, svc.SelectInitialFaction(ctx, testPlayerID1, f))
			}

			tt.assertErr(t, svc.SelectInitialFaction(ctx, tt.playerID, tt.faction))

			owned, err := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantOwned, owned)

			initial, err := factionRepo.GetInitialFaction(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, tt.wantInitial, initial)
		})
	}
}
