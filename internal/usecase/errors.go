package usecase

import (
	"errors"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// usecase 層は HTTP ステータスを知らず、以下のセンチネルで業務上の意味を表現する。
// repository は port.ErrNotFound とラップした SQL エラーだけを返し、
// handler (rest) が errors.Is でこれらのセンチネルを分類して HTTP ステータスに変換する。
var (
	// ErrNotFound は port.ErrNotFound の re-export。
	ErrNotFound = port.ErrNotFound
	// ErrPlayerNotFound はプレイヤーが存在しないことを示す。
	ErrPlayerNotFound = errors.New("player not found")
	// ErrPlayerAlreadyRegistered は同一 Firebase UID での二重登録を示す。
	ErrPlayerAlreadyRegistered = errors.New("player already registered")
	// ErrInvalidFaction は不正なファクション値を示す。
	ErrInvalidFaction = errors.New("invalid faction")
	// ErrFactionAlreadySelected は初期ファクション選択済みを示す。フローは冪等。
	ErrFactionAlreadySelected = errors.New("initial faction already selected")
	// ErrFactionConflict は subscriber 経路で「既に別 faction が initial 確定済み」を
	// 検出したことを示す。publisher のバグとして NACK 扱いする。
	ErrFactionConflict = errors.New("initial faction conflict")
	// ErrBattleLimitExceeded は当日のバトル回数が上限に達していることを示す。
	ErrBattleLimitExceeded = errors.New("daily battle limit exceeded")
)
