// Package db はPostgreSQLへの接続とスキーマ初期化を担当する
package db

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// migrations はサーバー起動時に順番に適用するDDL群。すべて冪等（何度実行しても安全）にする
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS subscriptions (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		price INTEGER NOT NULL CHECK (price >= 0),
		billing_cycle TEXT NOT NULL CHECK (billing_cycle IN ('monthly', 'yearly')),
		next_billing_date DATE NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`,
	`ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'その他'`,
	`ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_category_check`,
	`ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_category_check
		CHECK (category IN ('エンタメ', '音楽', 'アプリ', 'ツール', 'アーティスト', 'その他'))`,
}

// Connect はDSNを使ってPostgreSQLへの接続プールを作成する
func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := conn.PingContext(ctx); err != nil {
		return nil, err
	}
	return conn, nil
}

// Migrate はテーブル・カラムが存在しない場合に作成する
func Migrate(ctx context.Context, conn *sql.DB) error {
	for _, stmt := range migrations {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
