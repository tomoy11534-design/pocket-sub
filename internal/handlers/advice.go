// Package handlers はサブスクリプションのCRUDを担当するHTTPハンドラーを提供する
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	anthropicAPIURL         = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion     = "2023-06-01"
	anthropicModel          = "claude-sonnet-5"
	anthropicMaxTokens      = 1024
	anthropicRequestTimeout = 15 * time.Second
)

// adviceResponse はAI提案APIのレスポンス
type adviceResponse struct {
	Advice string `json:"advice"`
}

// anthropicSubscriptionSummary はAIへ送信するサブスク情報（内部ID等は含めない最小限の項目）
type anthropicSubscriptionSummary struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	Price        int    `json:"price"`
	BillingCycle string `json:"billing_cycle"`
	AnnualCost   int    `json:"annual_cost"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

// Advice は登録中のサブスク一覧をClaude APIへ送信し、支出見直しの提案を取得する
func (h *SubscriptionHandler) Advice(w http.ResponseWriter, r *http.Request) {
	if h.anthropicAPIKey == "" {
		writeError(w, http.StatusServiceUnavailable, "AI提案機能は現在利用できません")
		return
	}

	subscriptions, err := h.findAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "サブスクリプションの取得に失敗しました")
		return
	}
	if len(subscriptions) == 0 {
		writeError(w, http.StatusBadRequest, "登録されているサブスクがありません")
		return
	}

	summaries := make([]anthropicSubscriptionSummary, 0, len(subscriptions))
	for _, s := range subscriptions {
		summaries = append(summaries, anthropicSubscriptionSummary{
			Name:         s.Name,
			Category:     s.Category,
			Price:        s.Price,
			BillingCycle: s.BillingCycle,
			AnnualCost:   s.AnnualCost,
		})
	}
	payload, err := json.Marshal(summaries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "リクエストの作成に失敗しました")
		return
	}

	advice, err := h.requestAdvice(r.Context(), string(payload))
	if err != nil {
		writeError(w, http.StatusBadGateway, "AIへの問い合わせに失敗しました")
		return
	}

	writeJSON(w, http.StatusOK, adviceResponse{Advice: advice})
}

// requestAdvice はAnthropic Messages APIを呼び出し、アドバイステキストを取得する
func (h *SubscriptionHandler) requestAdvice(ctx context.Context, subscriptionsJSON string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, anthropicRequestTimeout)
	defer cancel()

	reqBody := anthropicRequest{
		Model:     anthropicModel,
		MaxTokens: anthropicMaxTokens,
		System: "あなたは家計の支出改善に詳しいアドバイザーです。" +
			"ユーザーが契約中のサブスクリプション一覧(JSON)を分析し、" +
			"無駄な支出や見直し候補を日本語の簡潔な箇条書きで提案してください。",
		Messages: []anthropicMessage{
			{Role: "user", Content: subscriptionsJSON},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", h.anthropicAPIKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic api error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, block := range parsed.Content {
		sb.WriteString(block.Text)
	}
	return sb.String(), nil
}
