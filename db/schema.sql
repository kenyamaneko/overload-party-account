-- overload-party-account - PostgreSQL DDL (このサービスの SSoT)
--
-- 他のサービスはこれらのテーブルを直接読み書きせず、アカウントサービスの内部 REST API を呼ぶ。
-- 陣営のマスターデータは common の factions.yaml が SSoT のため、サーバー側に陣営の表を持たない。
--
-- ops リポジトリが psqldef でこのファイルを Cloud SQL に適用するため、psqldef が解釈できる DDL だけを書く。

-- =============================================================================
-- スキーマ
-- =============================================================================

CREATE SCHEMA IF NOT EXISTS account;

-- =============================================================================
-- スキーマ内の補助関数
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

-- onboarding_status はオンボーディングの進行状態。表示名 (name) が NULL かどうかとは独立に、
-- 未完了 (not_started / name_set / faction_set) と完了 (completed) を区別できるよう、列挙値で持つ。
-- NULL はデータの消失を示す値として使うため、未完了にも使わない。
-- 進行は not_started → name_set → faction_set → completed の一方向だけ。
--
-- オンボーディングで選択した陣営は player_factions.is_initial が表す。所持陣営と初期陣営の SSoT を
-- 1 つの表に集めるため、players には専用の列を持たない。
--
-- last_premium_event_occurred_at は、古いプレミアム状態更新イベントが新しい状態を上書きしないよう、
-- 最後に反映したイベントの発生時刻を保存する。REST によるプレミアム状態の更新は、この列を変えない。
CREATE TABLE account.players (
  player_id          UUID NOT NULL DEFAULT gen_random_uuid(), -- UUID
  firebase_uid       VARCHAR(128) NOT NULL,          -- Firebase Auth UID (一意)
  name               VARCHAR(50),                    -- 表示名 (NULL: 未設定)
  is_premium         BOOLEAN NOT NULL,               -- プレミアム状態
  equipped_icon_no   BIGINT,                         -- 装備中アイコン番号（NULL: デフォルト）
  onboarding_status  VARCHAR(20) NOT NULL DEFAULT 'not_started', -- オンボーディング進行状態
  premium_expires_at TIMESTAMPTZ,                    -- プレミアムの有効期限
  last_premium_event_occurred_at TIMESTAMPTZ,        -- 最後に反映したプレミアム状態更新イベントの発生時刻 (NULL: 未反映)
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(), -- 作成日時
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(), -- 更新日時
  PRIMARY KEY (player_id),
  CONSTRAINT chk_players_onboarding_status
    CHECK (onboarding_status IN ('not_started', 'name_set', 'faction_set', 'completed'))
);

CREATE UNIQUE INDEX idx_players_firebase_uid ON account.players(firebase_uid);
CREATE TRIGGER trg_players_updated_at BEFORE UPDATE ON account.players FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

-- =============================================================================
-- account.player_progression (players の子、1:1)
--
-- exp は戦闘ごとに高頻度で UPDATE されるため、プロフィール系の players と別の表にする。
-- =============================================================================

CREATE TABLE account.player_progression (
  player_id  UUID PRIMARY KEY REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  exp        BIGINT NOT NULL DEFAULT 0,             -- 経験値 (初期値: 0)
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()     -- 更新日時
);
CREATE TRIGGER trg_player_progression_updated_at BEFORE UPDATE ON account.player_progression FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

-- =============================================================================
-- account.player_daily_battle (players の子、1 行/プレイヤー/ゲーム日)
-- 履歴を残し、分析できるようにする。
-- ゲーム日 (game_date) は JST 05:00 で切り替わる日付で、usecase 層が決める。
-- =============================================================================

CREATE TABLE account.player_daily_battle (
  player_id          UUID NOT NULL REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  game_date          DATE NOT NULL,                  -- ゲーム日 (JST 05:00 で切り替わる)
  daily_battle_count BIGINT NOT NULL,                -- そのゲーム日のバトル回数
  PRIMARY KEY (player_id, game_date)
);

-- =============================================================================
-- account.battle_count_reversals
-- 消費バトル回数の返却を、対戦ごとに 1 回だけ適用するための記録。
-- processed_events は Pub/Sub の再配信専用のため、REST 呼び出しの重複はここで別に防ぐ。
-- =============================================================================

CREATE TABLE account.battle_count_reversals (
  game_id     TEXT PRIMARY KEY,                     -- 対戦識別子 (battle が発行する game_id)
  reverted_at TIMESTAMPTZ NOT NULL DEFAULT now()     -- 返却処理日時
);

-- =============================================================================
-- account.player_factions (陣営所持の中間テーブル)
--
-- faction の CHECK 制約の値は、陣営の SSoT (factions.yaml) から生成された game-design-constants を元に
-- scripts/generate_schema_factions.go が書き換える。手で編集しない。
-- =============================================================================

CREATE TABLE account.player_factions (
  player_id   UUID NOT NULL REFERENCES account.players(player_id) ON DELETE CASCADE, -- 親テーブル参照
  faction     VARCHAR(20) NOT NULL CHECK (faction IN ('Neutral', 'SHE', 'Tenki', 'Sugar', 'Tuners')), -- 陣営名
  is_initial  BOOLEAN NOT NULL DEFAULT FALSE,        -- オンボーディングで選択した陣営か (1 プレイヤーにつき最大 1 行 TRUE)
  acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),    -- 取得日時
  PRIMARY KEY (player_id, faction)
);

-- オンボーディングで選択した陣営は 1 プレイヤーにつき最大 1 つなので、部分一意インデックスで保証する。
-- アプリ側でロックを取らずに、DB 側で一意性を担保するため。
CREATE UNIQUE INDEX idx_player_factions_initial
  ON account.player_factions(player_id) WHERE is_initial = TRUE;

-- =============================================================================
-- account.player_settings
-- =============================================================================

CREATE TABLE account.player_settings (
  player_id    UUID PRIMARY KEY REFERENCES account.players(player_id) ON DELETE CASCADE, -- プレイヤーID
  "language"   VARCHAR(10) NOT NULL,                 -- 言語設定
  bgm_volume   BIGINT NOT NULL,                      -- BGM音量 (0-100)
  se_volume    BIGINT NOT NULL,                      -- SE音量 (0-100)
  push_enabled BOOLEAN NOT NULL,                     -- 通知許可
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()    -- 更新日時
);
CREATE TRIGGER trg_player_settings_updated_at BEFORE UPDATE ON account.player_settings FOR EACH ROW EXECUTE FUNCTION account.update_updated_at();

-- =============================================================================
-- account.processed_events (Pub/Sub 購読の冪等性)
-- =============================================================================

CREATE TABLE account.processed_events (
  event_id     UUID PRIMARY KEY,                     -- Pub/Sub のイベントID (発行元が生成する UUIDv4)
  event_type   TEXT NOT NULL,                        -- 受信したイベントの event_type
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()    -- 処理日時
);
