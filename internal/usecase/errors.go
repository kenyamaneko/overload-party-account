package usecase

import (
	"errors"
	"fmt"
)

var (
	// ErrPlayerAlreadyRegistered は同じ FirebaseUID で登録済みであることを示す。
	ErrPlayerAlreadyRegistered = errors.New("player already registered")
	// ErrInvalidFaction は選択できない陣営であることを示す。
	ErrInvalidFaction = errors.New("invalid faction")
	// ErrUnknownFaction は所持陣営として存在しない陣営であることを示す。
	ErrUnknownFaction = errors.New("unknown faction")
	// ErrFactionAlreadySelected は初期陣営を選択済みであることを示す。
	ErrFactionAlreadySelected = errors.New("initial faction already selected")
	// ErrFactionConflict は初期陣営として確定済みの陣営と異なる陣営が指定されたことを示す。
	ErrFactionConflict = errors.New("initial faction conflict")
	// ErrBattleLimitExceeded は当日のバトル回数が上限に達していることを示す。
	ErrBattleLimitExceeded = errors.New("daily battle limit exceeded")
	// ErrGameConfigNotFound は必要なゲーム設定が存在しないことを示す。プレイヤーが存在しないことを示すエラーとは別である。
	ErrGameConfigNotFound = errors.New("game config not found")
	// ErrGameConfigBelowMinimum は、ゲーム設定の値が設定ごとの最小値を下回っていることを示す。
	ErrGameConfigBelowMinimum = errors.New("game config is below the minimum")
	// ErrInvalidExpGain は経験値の加算量が負であることを示す。
	ErrInvalidExpGain = errors.New("invalid exp gain")
	// ErrPlayer2IDRequired は、対人戦なのに2人目のプレイヤーIDが指定されていないことを示す。
	ErrPlayer2IDRequired = errors.New("player2 id is required")
	// ErrInvalidGameResult は対戦種別・勝者の番号・終了理由の組が不正であることを示す。
	ErrInvalidGameResult = errors.New("invalid game result")
	// ErrInvalidWinnerNumber は勝者の番号が不正であることを示す。ErrInvalidGameResult の一種である。
	ErrInvalidWinnerNumber = fmt.Errorf("%w: winner number", ErrInvalidGameResult)
	// ErrInvalidMatchType は対戦種別が不正であることを示す。ErrInvalidGameResult の一種である。
	ErrInvalidMatchType = fmt.Errorf("%w: match type", ErrInvalidGameResult)
)
