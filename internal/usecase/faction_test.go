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

	const unknownPID = "99999999-9999-9999-9999-999999999999"

	// selectAndAssertState は事前セットアップ (seedShop / preSelect) を適用して
	// SelectInitialFaction を呼び、呼び出し後の所持一覧と initial を検証して返す。
	selectAndAssertState := func(t *testing.T, seedShop, preSelect []string, playerID, faction string, wantOwned []string, wantInitial *string) error {
		t.Helper()
		svc := newFactionTestInteractor(t, testPlayerID1)
		_, _, factionRepo, _, _ := newRealRepos()

		for _, f := range seedShop {
			require.NoError(t, factionRepo.AddPlayerFaction(ctx, testPlayerID1, f))
		}
		for _, f := range preSelect {
			require.NoError(t, svc.SelectInitialFaction(ctx, testPlayerID1, f))
		}

		err := svc.SelectInitialFaction(ctx, playerID, faction)

		owned, gerr := factionRepo.GetPlayerFactions(ctx, testPlayerID1)
		require.NoError(t, gerr)
		assert.ElementsMatch(t, wantOwned, owned)

		initial, gerr := factionRepo.GetInitialFaction(ctx, testPlayerID1)
		require.NoError(t, gerr)
		assert.Equal(t, wantInitial, initial)

		return err
	}

	t.Run("初期ファクションの選択", func(t *testing.T) {
		validCases := []struct {
			name        string
			seedShop    []string // AddPlayerFaction で testPlayerID1 に事前追加
			faction     string
			wantOwned   []string
			wantInitial *string
		}{
			{
				name:        "SHE を初回選択するとき、所持 [SHE]・initial=SHE になる",
				faction:     "SHE",
				wantOwned:   []string{"SHE"},
				wantInitial: ptr("SHE"),
			},
			{
				name:        "Tenki を初回選択するとき、所持 [Tenki]・initial=Tenki になる",
				faction:     "Tenki",
				wantOwned:   []string{"Tenki"},
				wantInitial: ptr("Tenki"),
			},
			{
				name:        "ショップ所持 (SHE) と別の Tenki を初回選択するとき、所持 [SHE, Tenki]・initial=Tenki になる",
				seedShop:    []string{"SHE"},
				faction:     "Tenki",
				wantOwned:   []string{"SHE", "Tenki"},
				wantInitial: ptr("Tenki"),
			},
		}
		for _, tc := range validCases {
			t.Run(tc.name, func(t *testing.T) {
				err := selectAndAssertState(t, tc.seedShop, nil, testPlayerID1, tc.faction, tc.wantOwned, tc.wantInitial)
				require.NoError(t, err)
			})
		}

		invalidCases := []struct {
			name        string
			preSelect   []string // SelectInitialFaction を testPlayerID1 で事前実行 (順次)
			playerID    string   // テスト対象呼び出しの playerID
			faction     string
			wantErr     error
			wantOwned   []string
			wantInitial *string
		}{
			{
				name:        "既に initial 選択済みで再選択するとき、ErrFactionAlreadySelected になる",
				preSelect:   []string{"Tenki"},
				playerID:    testPlayerID1,
				faction:     "Tenki",
				wantErr:     ErrFactionAlreadySelected,
				wantOwned:   []string{"Tenki"},
				wantInitial: ptr("Tenki"),
			},
			{
				name:        "Neutral を選択するとき、ErrInvalidFaction になる",
				playerID:    testPlayerID1,
				faction:     "Neutral",
				wantErr:     ErrInvalidFaction,
				wantOwned:   []string{},
				wantInitial: nil,
			},
			{
				name:        "未知ファクションのとき、ErrInvalidFaction になる",
				playerID:    testPlayerID1,
				faction:     "bogus",
				wantErr:     ErrInvalidFaction,
				wantOwned:   []string{},
				wantInitial: nil,
			},
			{
				name:        "空ファクションのとき、ErrInvalidFaction になる",
				playerID:    testPlayerID1,
				faction:     "",
				wantErr:     ErrInvalidFaction,
				wantOwned:   []string{},
				wantInitial: nil,
			},
			{
				name:        "空 playerID のとき、ErrInvalidFaction になる",
				playerID:    "",
				faction:     "SHE",
				wantErr:     ErrInvalidFaction,
				wantOwned:   []string{},
				wantInitial: nil,
			},
			{
				name:        "存在しない playerID のとき、port.ErrNotFound になる",
				playerID:    unknownPID,
				faction:     "SHE",
				wantErr:     port.ErrNotFound,
				wantOwned:   []string{},
				wantInitial: nil,
			},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				err := selectAndAssertState(t, nil, tc.preSelect, tc.playerID, tc.faction, tc.wantOwned, tc.wantInitial)
				require.ErrorIs(t, err, tc.wantErr)
			})
		}

		t.Run("ショップ所持と同一 faction を初回選択するとき、PK 重複でエラーになる", func(t *testing.T) {
			// PK 重複はドメイン sentinel を持たない永続層エラーなので、種別ではなくエラー発生のみ確かめる。
			err := selectAndAssertState(t, []string{"SHE"}, nil, testPlayerID1, "SHE", []string{"SHE"}, nil)
			require.Error(t, err)
		})
	})
}
