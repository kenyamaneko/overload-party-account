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

-- onboarding_status はオンボーディング進行の SSoT。`name` の nullable 状態とは独立に
-- 「未完了 (not_started / name_set / faction_set) と完了 (completed)」を識別可能にする。
-- NULL を「データ消失」専用にし、未完了は明示的な列挙値で表現する設計。state machine は
-- 一方向遷移のみ (not_started → name_set → faction_set → completed)。
--
-- 「オンボーディングで選択した faction」は player_factions.is_initial が表現し、players
-- 側に専用カラムを置かない (所持リストと初回選択の SSoT を 1 テーブルに集約)。
CREATE TABLE account.players (
  player_id          UUID NOT NULL DEFAULT gen_random_uuid(), -- UUID
  firebase_uid       VARCHAR(128) NOT NULL,          -- Firebase Auth UID (Unique)
  name               VARCHAR(50),                    -- 表示名 (NULL: 未設定)
  is_premium         BOOLEAN NOT NULL,               -- 課金ステータス
  equipped_icon_no   BIGINT,                         -- 装備中アイコン番号（NULL: デフォルト）
  onboarding_status  VARCHAR(20) NOT NULL DEFAULT 'not_started', -- オンボード進行状態
  premium_expires_at TIMESTAMPTZ,                    -- サブスク有効期限
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(), -- 作成日時
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(), -- 更新日時
  PRIMARY KEY (player_id),
  CONSTRAINT chk_players_onboarding_status
    CHECK (onboarding_status IN ('not_started', 'name_set', 'faction_set', 'completed'))
);

CREATE UNIQUE INDEX idx_players_firebase_uid ON account.players(firebase_uid);
CREATE TRIGGER trg_players_updated_at BEFORE UPDATE ON account.players FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

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
-- account.player_daily_battle (child of players, 1 行/プレイヤー/ゲーム日)
-- 履歴を残して将来の BigQuery エクスポートで分析可能にする。
-- ゲーム日 (game_date) は JST 05:00 リセットの civil.Date を service 層が決める。
-- =============================================================================

CREATE TABLE account.player_daily_battle (
  player_id          UUID NOT NULL REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  game_date          DATE NOT NULL,                  -- ゲーム日 (JST 05:00 リセット)
  daily_battle_count BIGINT NOT NULL,                -- そのゲーム日のバトル回数
  PRIMARY KEY (player_id, game_date)
);

-- =============================================================================
-- account.player_factions (陣営所持の中間テーブル)
-- =============================================================================

CREATE TABLE account.player_factions (
  player_id   UUID NOT NULL REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  faction     VARCHAR(20) NOT NULL CHECK (faction IN ('SHE', 'Tenki', 'Sugar', 'Tuners', 'Neutral')), -- 陣営名 (SHE / Tenki / Sugar / Tuners / Neutral)
  is_initial  BOOLEAN NOT NULL DEFAULT FALSE,        -- オンボーディングで選択した faction か (1 プレイヤーにつき最大 1 行 TRUE)
  acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),    -- 取得日時
  PRIMARY KEY (player_id, faction)
);

-- 「オンボーディングで選択した faction は 1 プレイヤーに最大 1 つ」を partial unique index で保証する。
-- アプリ層で SELECT FOR UPDATE する代わりに DB 側で唯一性を担保する。
CREATE UNIQUE INDEX idx_player_factions_initial
  ON account.player_factions(player_id) WHERE is_initial = TRUE;

-- =============================================================================
-- account.player_settings
-- =============================================================================

CREATE TABLE account.player_settings (
  player_id    UUID PRIMARY KEY REFERENCES account.players(player_id) ON DELETE CASCADE, -- プレイヤーID
  language     VARCHAR(10) NOT NULL,                 -- 言語設定
  bgm_volume   BIGINT NOT NULL,                      -- BGM音量 (0-100)
  se_volume    BIGINT NOT NULL,                      -- SE音量 (0-100)
  push_enabled BOOLEAN NOT NULL,                     -- 通知許可
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()    -- 更新日時
);
CREATE TRIGGER trg_player_settings_updated_at BEFORE UPDATE ON account.player_settings FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

-- =============================================================================
-- account.processed_events (Pub/Sub subscriber idempotency)
-- =============================================================================

CREATE TABLE account.processed_events (
  event_id     UUID PRIMARY KEY,                     -- Pub/Sub EventID (publisher 生成の UUIDv4)
  event_type   TEXT NOT NULL,                        -- イベント種別 (faction_acquired / premium_updated / player_onboarded)
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()    -- 処理日時
);
