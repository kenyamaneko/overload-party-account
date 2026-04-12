package model

import "time"

// GameConfig は shared.game_config の行を表す内部型です。
// account 契約の一部ではなく、他サービスからは参照されないため internal に置きます。
type GameConfig struct {
	Key       string    `json:"key" db:"key"`
	Value     string    `json:"value" db:"value"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
