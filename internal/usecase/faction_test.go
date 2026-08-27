//go:build integration

package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

func TestFactionInteractor_SelectInitialFaction(t *testing.T) {
	t.Run("[陣営]陣営のユースケース", func(t *testing.T) {
		t.Run("SelectInitialFaction", func(t *testing.T) {
			t.Run("選択可能な4ファクション(SHE/Tenki/Sugar/Tuners)に含まれないNeutralを指定したとき、エラーを返す", func(t *testing.T) {
				interactor := newTestFactionInteractor(t)
				playerID := registerTestPlayer(t, "firebase-select-1")

				err := interactor.SelectInitialFaction(context.Background(), playerID, gamedesign.FactionNeutral)

				require.Error(t, err)
			})

			t.Run("対象プレイヤーが存在しないとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				interactor := newTestFactionInteractor(t)

				err := interactor.SelectInitialFaction(context.Background(), uuid.NewString(), gamedesign.FactionSHE)

				assert.ErrorIs(t, err, usecase.ErrNotFound)
			})

			t.Run("対象プレイヤーが初期ファクション未選択のとき、選択を確定する", func(t *testing.T) {
				interactor := newTestFactionInteractor(t)
				playerID := registerTestPlayer(t, "firebase-select-2")

				err := interactor.SelectInitialFaction(context.Background(), playerID, gamedesign.FactionSHE)

				require.NoError(t, err)
				factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
				got, err := factionRepo.GetInitialFaction(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, gamedesign.FactionSHE, *got)
			})

			t.Run("対象プレイヤーが初期ファクション選択済みのとき、ErrFactionAlreadySelectedを返し、選択内容は変更しない", func(t *testing.T) {
				interactor := newTestFactionInteractor(t)
				playerID := registerTestPlayer(t, "firebase-select-3")
				require.NoError(t, interactor.SelectInitialFaction(context.Background(), playerID, gamedesign.FactionSHE))

				err := interactor.SelectInitialFaction(context.Background(), playerID, gamedesign.FactionTenki)

				assert.ErrorIs(t, err, usecase.ErrFactionAlreadySelected)
				factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
				got, err := factionRepo.GetInitialFaction(context.Background(), playerID)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, gamedesign.FactionSHE, *got)
			})
		})
	})
}

func TestFactionInteractor_GrantFaction(t *testing.T) {
	t.Run("[陣営]陣営のユースケース", func(t *testing.T) {
		t.Run("GrantFaction", func(t *testing.T) {
			t.Run("指定したプレイヤーへ指定ファクションを追加する", func(t *testing.T) {
				interactor := newTestFactionInteractor(t)
				playerID := registerTestPlayer(t, "firebase-grant-1")

				err := interactor.GrantFaction(context.Background(), playerID, gamedesign.FactionSugar)

				require.NoError(t, err)
				factions, err := interactor.ListFactions(context.Background(), playerID)
				require.NoError(t, err)
				assert.Contains(t, factions, gamedesign.FactionSugar)
			})
		})
	})
}

func TestFactionInteractor_ListFactions(t *testing.T) {
	t.Run("[陣営]陣営のユースケース", func(t *testing.T) {
		t.Run("ListFactions", func(t *testing.T) {
			t.Run("指定したプレイヤーの所持ファクション一覧をそのまま返す", func(t *testing.T) {
				interactor := newTestFactionInteractor(t)
				playerID := registerTestPlayer(t, "firebase-list-1")
				require.NoError(t, interactor.GrantFaction(context.Background(), playerID, gamedesign.FactionSHE))
				require.NoError(t, interactor.GrantFaction(context.Background(), playerID, gamedesign.FactionTuners))

				factions, err := interactor.ListFactions(context.Background(), playerID)

				require.NoError(t, err)
				assert.ElementsMatch(t, []string{gamedesign.FactionSHE, gamedesign.FactionTuners}, factions)
			})
		})
	})
}
