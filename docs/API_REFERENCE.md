<!-- 自動生成 — 直接編集しない (scripts/generate_constants.py) -->

# Account Service API Reference

## Internal REST

- **Base path:** `/internal/v1`
- **認証:** internal

### `POST /internal/v1/auth/register`

新規プレイヤーを登録する（gateway が Firebase UID + username を中継）

> 成功時 201 Created。username は 1〜50 文字

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `FirebaseUID` | `string` | `firebase_uid` |  |
| `Username` | `string` | `username` |  |

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `PlayerID` | `string` | `player_id` |  |
| `FirebaseUID` | `string` | `firebase_uid` |  |
| `Username` | `string` | `username` |  |
| `Level` | `int64` | `level` |  |
| `Exp` | `int64` | `exp` |  |
| `IsPremium` | `bool` | `is_premium` |  |
| `EquippedIconNo` | `*int64` | `equipped_icon_no,omitempty` |  |
| `SelectedFaction` | `*string` | `selected_faction,omitempty` |  |
| `PremiumExpiresAt` | `*time.Time` | `premium_expires_at,omitempty` |  |
| `CreatedAt` | `time.Time` | `created_at` |  |
| `UpdatedAt` | `time.Time` | `updated_at` |  |
| `LevelExpCurrent` | `int64` | `level_exp_current` |  |
| `LevelExpRequired` | `int64` | `level_exp_required` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | firebase_uid が空 / username が 1〜50 文字範囲外 |
| 409 | 同一 firebase_uid で既に登録済み |
| 500 | DB 接続エラー |

---

### `POST /internal/v1/auth/login`

Firebase UID でログイン（プレイヤー情報を返す）

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `FirebaseUID` | `string` | `firebase_uid` |  |

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `PlayerID` | `string` | `player_id` |  |
| `FirebaseUID` | `string` | `firebase_uid` |  |
| `Username` | `string` | `username` |  |
| `Level` | `int64` | `level` |  |
| `Exp` | `int64` | `exp` |  |
| `IsPremium` | `bool` | `is_premium` |  |
| `EquippedIconNo` | `*int64` | `equipped_icon_no,omitempty` |  |
| `SelectedFaction` | `*string` | `selected_faction,omitempty` |  |
| `PremiumExpiresAt` | `*time.Time` | `premium_expires_at,omitempty` |  |
| `CreatedAt` | `time.Time` | `created_at` |  |
| `UpdatedAt` | `time.Time` | `updated_at` |  |
| `LevelExpCurrent` | `int64` | `level_exp_current` |  |
| `LevelExpRequired` | `int64` | `level_exp_required` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | firebase_uid が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `GET /internal/v1/players/by-firebase-uid/{firebaseUID}`

Firebase UID からプレイヤーを検索する

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `PlayerID` | `string` | `player_id` |  |
| `FirebaseUID` | `string` | `firebase_uid` |  |
| `Username` | `string` | `username` |  |
| `Level` | `int64` | `level` |  |
| `Exp` | `int64` | `exp` |  |
| `IsPremium` | `bool` | `is_premium` |  |
| `EquippedIconNo` | `*int64` | `equipped_icon_no,omitempty` |  |
| `SelectedFaction` | `*string` | `selected_faction,omitempty` |  |
| `PremiumExpiresAt` | `*time.Time` | `premium_expires_at,omitempty` |  |
| `CreatedAt` | `time.Time` | `created_at` |  |
| `UpdatedAt` | `time.Time` | `updated_at` |  |
| `LevelExpCurrent` | `int64` | `level_exp_current` |  |
| `LevelExpRequired` | `int64` | `level_exp_required` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | firebaseUID が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `POST /internal/v1/players/award-game-exp`

対戦終了後に両プレイヤーへ経験値を付与する（battle → gateway → account）

> 成功時 204 No Content。サービス間呼び出し用（playerId パスパラメータなし）

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `Player1ID` | `string` | `player1_id` |  |
| `Player2ID` | `string` | `player2_id` |  |
| `WinnerNum` | `int64` | `winner_num` |  |
| `Reason` | `string` | `reason` |  |
| `MatchType` | `string` | `match_type` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | リクエストボディ不正 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `GET /internal/v1/players/{playerId}`

プレイヤー情報を返す（レベル進捗を含む）

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `PlayerID` | `string` | `player_id` |  |
| `FirebaseUID` | `string` | `firebase_uid` |  |
| `Username` | `string` | `username` |  |
| `Level` | `int64` | `level` |  |
| `Exp` | `int64` | `exp` |  |
| `IsPremium` | `bool` | `is_premium` |  |
| `EquippedIconNo` | `*int64` | `equipped_icon_no,omitempty` |  |
| `SelectedFaction` | `*string` | `selected_faction,omitempty` |  |
| `PremiumExpiresAt` | `*time.Time` | `premium_expires_at,omitempty` |  |
| `CreatedAt` | `time.Time` | `created_at` |  |
| `UpdatedAt` | `time.Time` | `updated_at` |  |
| `LevelExpCurrent` | `int64` | `level_exp_current` |  |
| `LevelExpRequired` | `int64` | `level_exp_required` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `PUT /internal/v1/players/{playerId}/name`

プレイヤー名を変更する

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `Name` | `string` | `name` |  |

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `PlayerID` | `string` | `player_id` |  |
| `FirebaseUID` | `string` | `firebase_uid` |  |
| `Username` | `string` | `username` |  |
| `Level` | `int64` | `level` |  |
| `Exp` | `int64` | `exp` |  |
| `IsPremium` | `bool` | `is_premium` |  |
| `EquippedIconNo` | `*int64` | `equipped_icon_no,omitempty` |  |
| `SelectedFaction` | `*string` | `selected_faction,omitempty` |  |
| `PremiumExpiresAt` | `*time.Time` | `premium_expires_at,omitempty` |  |
| `CreatedAt` | `time.Time` | `created_at` |  |
| `UpdatedAt` | `time.Time` | `updated_at` |  |
| `LevelExpCurrent` | `int64` | `level_exp_current` |  |
| `LevelExpRequired` | `int64` | `level_exp_required` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 / name が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `GET /internal/v1/players/{playerId}/battle-limit`

1 日のバトル回数制限と残り回数を返す

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `DailyBattleCount` | `int64` | `daily_battle_count` |  |
| `DailyBattleLimit` | `int64` | `daily_battle_limit` |  |
| `CanBattle` | `bool` | `can_battle` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `POST /internal/v1/players/{playerId}/battle-limit/increment`

バトル回数を 1 増やす（バトル開始時に gateway が呼び出す）

> 成功時 204 No Content

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `PUT /internal/v1/players/{playerId}/premium`

プレミアムステータスを更新する（shop webhook 経由）

> 成功時 204 No Content

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `IsPremium` | `bool` | `is_premium` |  |
| `ExpiresAtMillis` | `*int64` | `expires_at_millis,omitempty` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 / リクエストボディ不正 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `PUT /internal/v1/players/{playerId}/faction`

選択中ファクションを変更する

> 成功時 204 No Content

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `Faction` | `string` | `faction` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 / faction が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `POST /internal/v1/players/{playerId}/exp`

経験値を加算する

> 成功時 204 No Content

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `ExpGain` | `int64` | `exp_gain` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 / リクエストボディ不正 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `POST /internal/v1/players/{playerId}/factions`

ファクション所有権を付与する（購入・報酬等）

> 成功時 204 No Content

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `Faction` | `string` | `faction` |  |
| `Source` | `string` | `source` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 / faction または source が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `POST /internal/v1/players/{playerId}/factions/select`

初期ファクション選択（チュートリアル完了時に scenario が呼び出す）

> 冪等。既に選択済みの場合は 409 を返すがクライアントにとってはエラーではない

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `FactionID` | `string` | `faction_id` |  |
| `Source` | `string` | `source` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | faction_id が空 / 無効なファクション / source が initial_selection 以外 |
| 404 | プレイヤーが存在しない |
| 409 | 初期ファクション選択済み（冪等 no-op） |
| 500 | DB 接続エラー |

---

### `GET /internal/v1/players/{playerId}/factions`

プレイヤーが所有するファクション一覧を返す

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `Factions` | `[]string` | `factions` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `GET /internal/v1/players/{playerId}/settings`

ユーザー設定を返す

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `PlayerID` | `string` | `player_id` |  |
| `Language` | `string` | `language` |  |
| `BgmVolume` | `int64` | `bgm_volume` |  |
| `SeVolume` | `int64` | `se_volume` |  |
| `PushEnabled` | `bool` | `push_enabled` |  |
| `UpdatedAt` | `time.Time` | `updated_at` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

### `PUT /internal/v1/players/{playerId}/settings`

ユーザー設定を更新する

#### リクエスト

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `Language` | `string` | `language` |  |
| `BgmVolume` | `int64` | `bgm_volume` |  |
| `SeVolume` | `int64` | `se_volume` |  |
| `PushEnabled` | `bool` | `push_enabled` |  |

#### レスポンス

| フィールド | 型 | JSON | 説明 |
|---|---|---|---|
| `PlayerID` | `string` | `player_id` |  |
| `Language` | `string` | `language` |  |
| `BgmVolume` | `int64` | `bgm_volume` |  |
| `SeVolume` | `int64` | `se_volume` |  |
| `PushEnabled` | `bool` | `push_enabled` |  |
| `UpdatedAt` | `time.Time` | `updated_at` |  |

#### エラー

| ステータス | 説明 |
|---|---|
| 400 | playerId が空 / language が空 |
| 404 | プレイヤーが存在しない |
| 500 | DB 接続エラー |

---

