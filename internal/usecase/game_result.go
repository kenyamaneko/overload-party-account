package usecase

import (
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

// gameOutcome は1人のプレイヤーから見た対戦の結果である。
type gameOutcome string

const (
	gameOutcomeWin  gameOutcome = "win"
	gameOutcomeLoss gameOutcome = "loss"
	gameOutcomeDraw gameOutcome = "draw"
)

// decideGameOutcomes は、対戦の結果を1人目と2人目のプレイヤーそれぞれの結果に分ける。
// 勝者の番号が 0 のときは終了理由によらず引き分けになる。
// 対戦種別が対人戦とNPC戦以外のときは ErrInvalidMatchType、勝者の番号が 0・1・2 以外のとき、
// または終了理由が引き分けなのに勝者の番号が 1 か 2 のときは ErrInvalidWinnerNumber を返す。
func decideGameOutcomes(winnerNumber domain.WinnerNumber, reason domain.WinReason, matchType domain.MatchType) (player1, player2 gameOutcome, err error) {
	switch matchType {
	case domain.MatchTypePvp, domain.MatchTypeNpc:
	default:
		return "", "", fmt.Errorf("%w: %q", ErrInvalidMatchType, matchType)
	}
	if winnerNumber != domain.WinnerNumberNone && reason == domain.WinReasonDraw {
		return "", "", fmt.Errorf("%w: reason %q with winner %d", ErrInvalidWinnerNumber, reason, winnerNumber)
	}
	switch winnerNumber {
	case domain.WinnerNumberNone:
		return gameOutcomeDraw, gameOutcomeDraw, nil
	case domain.WinnerNumberPlayer1:
		return gameOutcomeWin, gameOutcomeLoss, nil
	case domain.WinnerNumberPlayer2:
		return gameOutcomeLoss, gameOutcomeWin, nil
	default:
		return "", "", fmt.Errorf("%w: %d", ErrInvalidWinnerNumber, winnerNumber)
	}
}
