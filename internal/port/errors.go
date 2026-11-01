package port

import "errors"

var (
	// ErrNotFound は対象が存在しないことを示す。
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists は同じ対象がすでに登録されていることを示す。
	ErrAlreadyExists = errors.New("already exists")
	// ErrValueNotInteger はゲーム設定の値が整数として読めないことを示す。
	ErrValueNotInteger = errors.New("value is not an integer")
	// ErrDatabaseUnavailable はデータベースに接続できず、トランザクションを開始できないことを示す。
	ErrDatabaseUnavailable = errors.New("database unavailable")
)
