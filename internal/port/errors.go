package port

import "errors"

// ErrNotFound はリソースが存在しない場合に返します。
var ErrNotFound = errors.New("not found")
