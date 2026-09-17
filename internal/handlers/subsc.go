// Package handlers はサブスクリプションのCRUDを担当するHTTPハンドラーを提供する
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// billingCycleMonthly / billingCycleYearly は支払いサイクルの許容値
const (
	billingCycleMonthly = "monthly"
	billingCycleYearly  = "yearly"
)

// validCategories はカテゴリとして許容する値（DBのCHECK制約と一致させる）
var validCategories = map[string]bool{
	"エンタメ":   true,
	"音楽":     true,
	"アプリ":    true,
	"ツール":    true,
	"アーティスト": true,
	"その他":    true,
}

// Subscription は1件のサブスクリプション契約を表す
type Subscription struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Price            int       `json:"price"`
	BillingCycle     string    `json:"billing_cycle"`
	NextBillingDate  time.Time `json:"next_billing_date"`
	CreatedAt        time.Time `json:"created_at"`
	AnnualCost       int       `json:"annual_cost"`
	DaysUntilRenewal int       `json:"days_until_renewal"`
}

// calculateAnnualCost は支払いサイクルに基づいて年額換算した金額を返す
func calculateAnnualCost(s Subscription) int {
	if s.BillingCycle == billingCycleYearly {
		return s.Price
	}
	return s.Price * 12
}

// calculateDaysUntilRenewal は基準日から次回更新日までの残り日数を返す
func calculateDaysUntilRenewal(s Subscription, now time.Time) int {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextBilling := time.Date(
		s.NextBillingDate.Year(), s.NextBillingDate.Month(), s.NextBillingDate.Day(),
		0, 0, 0, 0, now.Location(),
	)
	return int(nextBilling.Sub(today).Hours() / 24)
}

// withComputedFields は年間コスト・残り日数を計算してセットする
func withComputedFields(s Subscription) Subscription {
	s.AnnualCost = calculateAnnualCost(s)
	s.DaysUntilRenewal = calculateDaysUntilRenewal(s, time.Now())
	return s
}

// SubscriptionHandler はサブスクリプションCRUD APIのハンドラー群
type SubscriptionHandler struct {
	db              *sql.DB
	anthropicAPIKey string
}

// NewSubscriptionHandler はSubscriptionHandlerを生成する
// anthropicAPIKeyが空文字の場合、AI提案機能(Advice)は503を返す
func NewSubscriptionHandler(db *sql.DB, anthropicAPIKey string) *SubscriptionHandler {
	return &SubscriptionHandler{db: db, anthropicAPIKey: anthropicAPIKey}
}

// List は登録済みサブスクリプションを次回更新日が近い順に返す
func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := h.findAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "サブスクリプションの取得に失敗しました")
		return
	}
	writeJSON(w, http.StatusOK, subscriptions)
}

// subscriptionRequest は登録・更新リクエストのボディ（Create/Updateで共用する）
type subscriptionRequest struct {
	Name            string `json:"name"`
	Category        string `json:"category"`
	Price           int    `json:"price"`
	BillingCycle    string `json:"billing_cycle"`
	NextBillingDate string `json:"next_billing_date"`
}

// parseSubscriptionRequest はリクエストボディを検証し、Subscriptionへ変換する
// Create/Updateの両方で共有するバリデーションロジック
func parseSubscriptionRequest(req subscriptionRequest) (Subscription, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || req.Price < 0 {
		return Subscription{}, errors.New("サービス名と料金は必須です")
	}

	if !validCategories[req.Category] {
		return Subscription{}, errors.New("カテゴリの指定が不正です")
	}

	if req.BillingCycle != billingCycleMonthly && req.BillingCycle != billingCycleYearly {
		return Subscription{}, errors.New("支払いサイクルは monthly か yearly を指定してください")
	}

	nextBillingDate, err := time.Parse("2006-01-02", req.NextBillingDate)
	if err != nil {
		return Subscription{}, errors.New("次回更新日の形式が不正です (YYYY-MM-DD)")
	}

	return Subscription{
		Name:            name,
		Category:        req.Category,
		Price:           req.Price,
		BillingCycle:    req.BillingCycle,
		NextBillingDate: nextBillingDate,
	}, nil
}

// Create は新しいサブスクリプションを登録する
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req subscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "リクエストの形式が不正です")
		return
	}

	parsed, err := parseSubscriptionRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.create(r.Context(), parsed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "サブスクリプションの登録に失敗しました")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// Update は指定IDのサブスクリプションを更新する
func (h *SubscriptionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IDの形式が不正です")
		return
	}

	var req subscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "リクエストの形式が不正です")
		return
	}

	parsed, err := parseSubscriptionRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.update(r.Context(), id, parsed)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "指定されたサブスクリプションが見つかりません")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "サブスクリプションの更新に失敗しました")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Delete は指定IDのサブスクリプションを削除する
func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IDの形式が不正です")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM subscriptions WHERE id = $1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "サブスクリプションの削除に失敗しました")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// summaryResponse は年間コストダッシュボード用のレスポンス
type summaryResponse struct {
	TotalAnnualCost       int `json:"total_annual_cost"`
	MonthlyEquivalentCost int `json:"monthly_equivalent_cost"`
	SubscriptionCount     int `json:"subscription_count"`
}

// Summary は全サブスクリプションの年間合計コスト・月額換算合計コスト・件数を返す
func (h *SubscriptionHandler) Summary(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := h.findAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "集計に失敗しました")
		return
	}

	total := 0
	for _, s := range subscriptions {
		total += s.AnnualCost
	}
	monthlyEquivalent := int(math.Round(float64(total) / 12))

	writeJSON(w, http.StatusOK, summaryResponse{
		TotalAnnualCost:       total,
		MonthlyEquivalentCost: monthlyEquivalent,
		SubscriptionCount:     len(subscriptions),
	})
}

// findAll はDBから全サブスクリプションを次回更新日が近い順に取得する
func (h *SubscriptionHandler) findAll(ctx context.Context) ([]Subscription, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT id, name, category, price, billing_cycle, next_billing_date, created_at
		FROM subscriptions
		ORDER BY next_billing_date ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subscriptions := []Subscription{}
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Price, &s.BillingCycle, &s.NextBillingDate, &s.CreatedAt); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, withComputedFields(s))
	}
	return subscriptions, rows.Err()
}

// create はDBに新しいサブスクリプションを1件登録する
func (h *SubscriptionHandler) create(ctx context.Context, s Subscription) (Subscription, error) {
	err := h.db.QueryRowContext(ctx, `
		INSERT INTO subscriptions (name, category, price, billing_cycle, next_billing_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, s.Name, s.Category, s.Price, s.BillingCycle, s.NextBillingDate).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return Subscription{}, err
	}
	return withComputedFields(s), nil
}

// update はDBの既存サブスクリプションを1件更新する。対象IDが存在しない場合はsql.ErrNoRowsを返す
func (h *SubscriptionHandler) update(ctx context.Context, id int64, s Subscription) (Subscription, error) {
	s.ID = id
	err := h.db.QueryRowContext(ctx, `
		UPDATE subscriptions
		SET name = $1, category = $2, price = $3, billing_cycle = $4, next_billing_date = $5
		WHERE id = $6
		RETURNING created_at
	`, s.Name, s.Category, s.Price, s.BillingCycle, s.NextBillingDate, id).Scan(&s.CreatedAt)
	if err != nil {
		return Subscription{}, err
	}
	return withComputedFields(s), nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
