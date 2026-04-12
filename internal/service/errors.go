package service

import (
	"errors"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var (
	// ErrNotFound は port.ErrNotFound の re-export です。
	ErrNotFound = port.ErrNotFound
	// ErrPlayerNotFound はプレイヤーが存在しない場合に返します。
	ErrPlayerNotFound = errors.New("player not found")
	// ErrPlayerAlreadyRegistered は同一 Firebase UID での二重登録時に返します。
	ErrPlayerAlreadyRegistered = errors.New("player already registered")
	// ErrInvalidFaction は不正なファクション値の場合に返します。
	ErrInvalidFaction = errors.New("invalid faction")
	// ErrFactionAlreadySelected は初期ファクション選択済みの場合に返します。
	// REST handler は HTTP 409 にマップしますが、フローは冪等なのでクライアント視点ではエラーではありません。
	ErrFactionAlreadySelected = errors.New("initial faction already selected")
)
