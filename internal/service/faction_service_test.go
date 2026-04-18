package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFactionTestService は実 DB に playerID をシードし、実 repository で
// FactionService を組む。shop 流の「service テストも sharedPg で組む」方針。
func newFactionTestService(t *testing.T, playerID string) *FactionService {
	t.Helper()
	sharedPg.Truncate(t)
	seedPlayer(t, playerID, "uid-"+playerID, "tester", false)

	playerRepo, factionRepo, _, tx := newRealRepos()
	return NewFactionService(playerRepo, factionRepo, tx)
}

func TestFactionService_SelectInitialFaction(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		playerID      string
		preSelect     string // 事前に InsertInitial を通す faction（空なら skip）
		faction       string
		wantErrIs     error
		wantFactions  []string
		wantSelected  *string
		wantSkipSeed  bool // true なら seed player を行わない
		useEmptyPID   bool // SelectInitialFaction に空の playerID を渡す
	}{
		{
			name:         "初回選択は player_factions 挿入 + selected_faction 更新",
			playerID:     testPlayerID1,
			faction:      "SHE",
			wantFactions: []string{"SHE"},
			wantSelected: strPtr("SHE"),
		},
		{
			name:         "既存選択済みは ErrFactionAlreadySelected",
			playerID:     testPlayerID1,
			preSelect:    "Tenki",
			faction:      "Tenki",
			wantErrIs:    ErrFactionAlreadySelected,
			wantFactions: []string{"Tenki"},
			wantSelected: strPtr("Tenki"),
		},
		{
			name:         "Neutral は選択不可 (ErrInvalidFaction)",
			playerID:     testPlayerID1,
			faction:      "Neutral",
			wantErrIs:    ErrInvalidFaction,
			wantFactions: nil,
			wantSelected: nil,
		},
		{
			name:         "未知ファクションは ErrInvalidFaction",
			playerID:     testPlayerID1,
			faction:      "bogus",
			wantErrIs:    ErrInvalidFaction,
			wantFactions: nil,
			wantSelected: nil,
		},
		{
			name:         "空ファクションは ErrInvalidFaction",
			playerID:     testPlayerID1,
			faction:      "",
			wantErrIs:    ErrInvalidFaction,
			wantFactions: nil,
			wantSelected: nil,
		},
		{
			name:        "空 playerID は ErrInvalidFaction",
			playerID:    testPlayerID1,
			useEmptyPID: true,
			faction:     "SHE",
			wantErrIs:   ErrInvalidFaction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFactionTestService(t, tt.playerID)

			if tt.preSelect != "" {
				require.NoError(t, svc.SelectInitialFaction(ctx, tt.playerID, tt.preSelect))
			}

			pidArg := tt.playerID
			if tt.useEmptyPID {
				pidArg = ""
			}
			err := svc.SelectInitialFaction(ctx, pidArg, tt.faction)

			if tt.wantErrIs != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErrIs),
					"err %v should be %v", err, tt.wantErrIs)
			} else {
				require.NoError(t, err)
			}

			if tt.useEmptyPID {
				// 空 playerID のケースは副作用が無いことだけを確認して終了。
				return
			}

			// player_factions の状態
			_, factionRepo, _, _ := newRealRepos()
			factions, err := factionRepo.GetPlayerFactions(ctx, tt.playerID)
			require.NoError(t, err)
			if tt.wantFactions == nil {
				assert.Empty(t, factions)
			} else {
				assert.ElementsMatch(t, tt.wantFactions, factions)
			}

			// players.selected_faction の状態
			playerRepo, _, _, _ := newRealRepos()
			p, err := playerRepo.FindByID(ctx, tt.playerID)
			require.NoError(t, err)
			if tt.wantSelected == nil {
				assert.Nil(t, p.SelectedFaction)
			} else {
				require.NotNil(t, p.SelectedFaction)
				assert.Equal(t, *tt.wantSelected, *p.SelectedFaction)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
