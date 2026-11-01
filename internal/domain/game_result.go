package domain

import (
	gamelogic "github.com/kenyamaneko/overload-party-battle/packages/game-logic-constants-go"
	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"
)

// MatchType は対戦種別である。
type MatchType string

// 対戦種別。
const (
	MatchTypePvp MatchType = gamedesign.MatchTypePvp
	MatchTypeNpc MatchType = gamedesign.MatchTypeNpc
)

// WinnerNumber は対戦の勝者の番号である。
type WinnerNumber int64

// 対戦の勝者の番号。
const (
	// WinnerNumberNone は勝者がいない引き分けを表す。
	WinnerNumberNone    WinnerNumber = 0
	WinnerNumberPlayer1 WinnerNumber = 1
	WinnerNumberPlayer2 WinnerNumber = 2
)

// WinReason は対戦の終了理由である。
type WinReason string

// WinReasonDraw は引き分けによる終了を表す終了理由である。
const WinReasonDraw WinReason = gamelogic.WinReasonDraw
