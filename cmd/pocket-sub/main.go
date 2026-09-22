// PocketSub サーバーのエントリーポイント
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"pocketsub/internal/db"
	"pocketsub/internal/handlers"
)

func main() {
	loadEnvFile(".env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("環境変数 DATABASE_URL が設定されていません")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := db.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("データベース接続に失敗しました: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(ctx, conn); err != nil {
		log.Fatalf("スキーマの初期化に失敗しました: %v", err)
	}

	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	subscriptionHandler := handlers.NewSubscriptionHandler(conn, anthropicAPIKey)

	mux := http.NewServeMux()

	// APIエンドポイント
	mux.HandleFunc("GET /api/subscriptions", subscriptionHandler.List)
	mux.HandleFunc("POST /api/subscriptions", subscriptionHandler.Create)
	mux.HandleFunc("PUT /api/subscriptions/{id}", subscriptionHandler.Update)
	mux.HandleFunc("DELETE /api/subscriptions/{id}", subscriptionHandler.Delete)
	mux.HandleFunc("GET /api/summary", subscriptionHandler.Summary)
	mux.HandleFunc("POST /api/ai/advice", subscriptionHandler.Advice)

	// 静的ファイル・画面（web/static配下をルート直下に配信する）
	mux.Handle("/", http.FileServer(http.Dir("web/static")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("PocketSub サーバーを起動しました: http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("サーバーの起動に失敗しました: %v", err)
	}
}

// loadEnvFile は.envファイルを読み込み、未設定の環境変数にのみ値をセットする
func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
