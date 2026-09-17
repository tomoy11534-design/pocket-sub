# PocketSub 仕様書

要件定義は [requirements.md](./requirements.md) を参照。本書ではシステム構成・API・データベース設計等の実装仕様を定義する。

## 1. システム構成

```
[ ブラウザ ]
    │  HTML / JS (fetch)
    ▼
[ Go APIサーバー (net/http) ]
    ├─ 静的ファイル配信: web/static, web/templates
    └─ JSON API: /api/*
        │  database/sql + pgx
        ▼
[ PostgreSQL (Supabase / Neon) ]
```

- 単一のGoバイナリが、画面（静的HTML/CSS/JS）とAPI（JSON）の両方を配信する。
- フロントエンドはページロード時、および操作（追加・削除）のたびに `fetch` でAPIを呼び出し、DOMを再描画する（SPA的なノーリロード更新）。

## 2. ディレクトリ構成

```
PocketSub/
├── docs/                        # ドキュメント
│   ├── requirements.md          # 要件定義書
│   └── specification.md         # 仕様書（本書）
├── cmd/
│   └── server/
│       └── main.go              # エントリーポイント
├── internal/
│   ├── database/
│   │   └── database.go          # DB接続・スキーマ初期化
│   ├── model/
│   │   └── subscription.go      # ドメインモデル
│   ├── repository/
│   │   └── subscription-repository.go  # DBアクセス
│   └── handler/
│       └── subscription-handler.go     # HTTPハンドラー
├── web/
│   ├── static/
│   │   ├── css/style.css
│   │   └── js/app.js
│   └── templates/
│       └── index.html
├── .env.example
├── go.mod
└── readme.md
```

## 3. データベース設計

### 3.1 テーブル: subscriptions

| カラム名 | 型 | 制約 | 説明 |
|---|---|---|---|
| id | SERIAL | PRIMARY KEY | サブスクID |
| name | TEXT | NOT NULL | サービス名 |
| price | INTEGER | NOT NULL, CHECK (price >= 0) | 料金（円） |
| billing_cycle | TEXT | NOT NULL, CHECK IN ('monthly','yearly') | 支払いサイクル |
| next_billing_date | DATE | NOT NULL | 次回更新日 |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | 登録日時 |

サーバー起動時に `CREATE TABLE IF NOT EXISTS` を実行し、自動でスキーマを初期化する（`internal/database/database.go`）。

## 4. API仕様

すべてのAPIは `Content-Type: application/json` でJSONを返す。

### 4.1 GET /api/subscriptions
サブスク一覧を次回更新日が近い順に取得する。

**レスポンス例 200 OK**
```json
[
  {
    "id": 1,
    "name": "Netflix",
    "price": 1490,
    "billing_cycle": "monthly",
    "next_billing_date": "2026-09-25T00:00:00Z",
    "created_at": "2026-01-10T09:00:00Z",
    "annual_cost": 17880,
    "days_until_renewal": 9
  }
]
```

### 4.2 POST /api/subscriptions
サブスクを新規登録する。

**リクエストボディ**
```json
{
  "name": "Netflix",
  "price": 1490,
  "billing_cycle": "monthly",
  "next_billing_date": "2026-09-25"
}
```

**レスポンス**
- `201 Created`: 登録したサブスク（4.1と同形式）
- `400 Bad Request`: 入力値不正（`{"error": "..."}`）

### 4.3 DELETE /api/subscriptions/{id}
指定IDのサブスクを削除する。

**レスポンス**
- `204 No Content`: 削除成功

### 4.4 GET /api/summary
年間コストダッシュボード用の集計値を取得する。

**レスポンス例 200 OK**
```json
{
  "total_annual_cost": 42800,
  "subscription_count": 5
}
```

## 5. 画面仕様

### 5.1 トップページ（`/`）

1画面構成。上から以下のブロックを配置する。

1. ヘッダー: アプリ名 + 「＋ サブスクを追加」ボタン
2. ダッシュボード: 「年間合計コスト」「登録件数」の2カード
3. サブスク一覧: カードのグリッド表示（サービス名／料金／支払いサイクル／次回更新日／更新日ステータス／削除ボタン）
4. 追加モーダル: 「＋ サブスクを追加」ボタン押下で表示。サービス名・料金・支払いサイクル・次回更新日を入力するフォーム

### 5.2 更新日ステータスの表示ルール

| 残り日数 | ラベル | 色 |
|---|---|---|
| 0〜3日 | あと〇日 | 赤 (bg-red-100 / text-red-700) |
| 4〜7日 | あと〇日 | 黄 (bg-amber-100 / text-amber-700) |
| 8日以上 | あと〇日 | 緑 (bg-emerald-100 / text-emerald-700) |

## 6. 環境変数

| 変数名 | 必須 | 説明 |
|---|---|---|
| DATABASE_URL | ○ | PostgreSQL接続文字列（例: `postgres://user:pass@host:5432/db?sslmode=require`） |
| PORT | - | サーバーのリッスンポート（未指定時は `8080`） |

`.env` ファイル（`.env.example` を参照して作成）をプロジェクトルートに配置すると、サーバー起動時に自動で読み込まれる。

## 7. エラーハンドリング方針

- APIのエラーは `{"error": "日本語のメッセージ"}` の形式でJSON返却する
- クライアント起因のエラー（入力不正等）は `400`、サーバー内部エラーは `500` を返す

## 8. 今後の拡張案

要件定義書 9章「将来拡張案」を参照。
