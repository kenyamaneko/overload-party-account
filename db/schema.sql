-- overload-party-account - PostgreSQL DDL (service-local SSoT)
--
-- Tables owned by the account service per ADR-011 (service split) and
-- ADR-014 (schema ownership). Other services MUST NOT read or write these
-- tables directly — they call the account internal REST API instead.
--
-- All tables live in the `account` schema.
--
-- NOTE: The `factions` reference table has been retired by the ADR-014
-- 補遺. Faction master data is now sourced from common/data/factions.yaml
-- and compiled into the shared constants packages; we no longer store a
-- server-side mirror.
--
-- psqldef compatible — ops repo applies this file to Cloud SQL.

-- =============================================================================
-- Schema
-- =============================================================================

CREATE SCHEMA IF NOT EXISTS account;

-- =============================================================================
-- Schema-local helpers
-- =============================================================================

CREATE OR REPLACE FUNCTION account.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- account.players
-- =============================================================================

CREATE TABLE account.players (
  player_id          UUID NOT NULL DEFAULT gen_random_uuid(), -- UUID
  firebase_uid       VARCHAR(128) NOT NULL,          -- Firebase Auth UID (Unique)
  username           VARCHAR(50) NOT NULL,           -- 表示名
  is_premium         BOOLEAN NOT NULL,               -- 課金ステータス
  equipped_icon_no   BIGINT,                         -- 装備中アイコン番号（NULL: デフォルト）
  selected_faction   VARCHAR(20),                    -- 選択済みファクション
  premium_expires_at TIMESTAMPTZ,                    -- サブスク有効期限
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(), -- 作成日時
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(), -- 更新日時
  PRIMARY KEY (player_id)
);

CREATE UNIQUE INDEX idx_players_firebase_uid ON account.players(firebase_uid);
CREATE TRIGGER trg_players_updated_at BEFORE UPDATE ON account.players FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

ALTER TABLE account.players
  ADD CONSTRAINT chk_players_selected_faction
    CHECK (selected_faction IS NULL OR selected_faction IN ('SHE', 'Tenki', 'Sugar', 'Tuners', 'Neutral'));

-- =============================================================================
-- account.player_progression (child of players, 1:1)
--
-- level/exp は戦闘ごとに高頻度で UPDATE されるため、手動操作で更新される players
-- のプロフィール系カラムと分離する。これにより:
--   * players.updated_at がバトル頻度で動かず、プロフィール変更の検知に使える
--   * 経験値加算の行ロックが players 全体に波及しない
--   * VACUUM / MVCC のコストが players に集中しない
-- =============================================================================

CREATE TABLE account.player_progression (
  player_id  UUID PRIMARY KEY REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  level      BIGINT NOT NULL DEFAULT 1,             -- レベル (Default: 1)
  exp        BIGINT NOT NULL DEFAULT 0,             -- 経験値 (Default: 0)
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()     -- 更新日時
);
CREATE TRIGGER trg_player_progression_updated_at BEFORE UPDATE ON account.player_progression FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

-- =============================================================================
-- account.player_daily_battle (child of players, 1:1)
-- =============================================================================

CREATE TABLE account.player_daily_battle (
  player_id          UUID PRIMARY KEY REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  daily_battle_count BIGINT NOT NULL,                -- 本日のバトル回数
  last_reset_date    DATE NOT NULL                   -- 最終リセット日
);

-- =============================================================================
-- account.player_factions (陣営所持の中間テーブル)
-- =============================================================================

CREATE TABLE account.player_factions (
  player_id   UUID NOT NULL REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  faction     VARCHAR(20) NOT NULL CHECK (faction IN ('SHE', 'Tenki', 'Sugar', 'Tuners', 'Neutral')), -- 陣営名 (SHE / Tenki / Sugar / Tuners / Neutral)
  source      VARCHAR(20) NOT NULL CHECK (source IN ('initial_selection', 'shop_purchase')), -- 取得経路 (initial_selection / shop_purchase)
  acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),    -- 取得日時
  PRIMARY KEY (player_id, faction)
);

-- =============================================================================
-- account.user_settings
-- =============================================================================

CREATE TABLE account.user_settings (
  player_id    UUID PRIMARY KEY REFERENCES account.players(player_id) ON DELETE CASCADE, -- ユーザーID
  language     VARCHAR(10) NOT NULL,                 -- 言語設定
  bgm_volume   BIGINT NOT NULL,                      -- BGM音量 (0-100)
  se_volume    BIGINT NOT NULL,                      -- SE音量 (0-100)
  push_enabled BOOLEAN NOT NULL,                     -- 通知許可
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()    -- 更新日時
);
CREATE TRIGGER trg_user_settings_updated_at BEFORE UPDATE ON account.user_settings FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

-- =============================================================================
-- account.processed_events (Pub/Sub subscriber idempotency)
-- =============================================================================

CREATE TABLE account.processed_events (
  event_id     UUID PRIMARY KEY,                     -- Pub/Sub EventID (publisher 生成の UUIDv4)
  event_type   TEXT NOT NULL,                        -- イベント種別 (faction_selected / premium_updated)
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()    -- 処理日時
);
