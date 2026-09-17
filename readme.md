# PocketSub

サブスク・固定費管理アプリ（Go + PostgreSQL + Tailwind CSS）

詳細な仕様は [docs/requirements.md](./docs/requirements.md)（要件定義書）、[docs/specification.md](./docs/specification.md)（仕様書）、[docs/design.md](./docs/design.md)（設計書）を参照。

## セットアップ

1. Go 1.22以上をインストールする
2. `.env.example` を `.env` にコピーし、`DATABASE_URL` に Supabase / Neon 等のPostgreSQL接続文字列を設定する
3. 依存パッケージを取得する

   ```
   go mod tidy
   ```

4. サーバーを起動する

   ```
   go run ./cmd/pocket-sub
   ```

5. ブラウザで `http://localhost:8080` を開く

起動時にテーブル（`subscriptions`）が存在しなければ自動で作成される。

## ディレクトリ構成

```
PocketSub/
├── docs/                        要件定義書・仕様書・設計書
├── cmd/pocket-sub/main.go       エントリーポイント
├── internal/db/db.go            DB接続・スキーマ初期化
├── internal/handlers/subsc.go   サブスクリプションCRUDハンドラー
└── web/static/                  フロントエンド (HTML/CSS/JS)
```
