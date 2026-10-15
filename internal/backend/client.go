// Package backend is a client for the family-finance-crm REST API.
//
// The bot authenticates as a service with X-Bot-Token and acts on behalf of a
// family member by naming their Telegram user id in X-Telegram-User-Id; the
// backend then applies that member's own access rules.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	headerBotToken       = "X-Bot-Token"
	headerTelegramUserID = "X-Telegram-User-Id"

	// maxErrorBody caps how much of an error response is read.
	maxErrorBody = 64 << 10
)

// Client calls the backend API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a client for the backend at baseURL (e.g. http://localhost:8080).
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL + "/api/v1",
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Link binds a Telegram account to the family member with that username.
func (c *Client) Link(ctx context.Context, req LinkRequest) (LinkResponse, error) {
	var out LinkResponse
	err := c.do(ctx, http.MethodPost, "/bot/link", 0, nil, req, &out)
	return out, err
}

// BillsDue returns every linked member's unpaid bills due on date.
func (c *Client) BillsDue(ctx context.Context, date time.Time) ([]DueBills, error) {
	var out []DueBills
	q := url.Values{"date": {date.Format(time.DateOnly)}}
	err := c.do(ctx, http.MethodGet, "/bot/bills-due", 0, q, nil, &out)
	return out, err
}

// Accounts returns the member's own accounts.
func (c *Client) Accounts(ctx context.Context, tgUserID int64) ([]Account, error) {
	var out []Account
	err := c.do(ctx, http.MethodGet, "/accounts", tgUserID, url.Values{"scope": {"OWN"}}, nil, &out)
	return out, err
}

// Categories returns the member's categories of both kinds.
func (c *Client) Categories(ctx context.Context, tgUserID int64) ([]Category, error) {
	var out []Category
	err := c.do(ctx, http.MethodGet, "/categories", tgUserID, nil, nil, &out)
	return out, err
}

// CreateTransaction records a transaction for the member.
func (c *Client) CreateTransaction(ctx context.Context, tgUserID int64, req CreateTransactionRequest) (Transaction, error) {
	var out Transaction
	err := c.do(ctx, http.MethodPost, "/transactions", tgUserID, nil, req, &out)
	return out, err
}

// MonthlySummary returns the member's income and expense totals for month (YYYY-MM).
func (c *Client) MonthlySummary(ctx context.Context, tgUserID int64, month string) (MonthlySummary, error) {
	var out MonthlySummary
	err := c.do(ctx, http.MethodGet, "/transactions/summary", tgUserID, url.Values{"month": {month}}, nil, &out)
	return out, err
}

// UnpaidBills returns all of the member's own unpaid bills, past and future,
// earliest due date first.
func (c *Client) UnpaidBills(ctx context.Context, tgUserID int64) ([]Bill, error) {
	var out []Bill
	q := url.Values{"unpaid": {"true"}, "scope": {"OWN"}}
	err := c.do(ctx, http.MethodGet, "/bills", tgUserID, q, nil, &out)
	return out, err
}

// MarkBillPaid marks one of the member's bills as paid.
func (c *Client) MarkBillPaid(ctx context.Context, tgUserID int64, billID string) (Bill, error) {
	var out Bill
	paid := true
	err := c.do(ctx, http.MethodPatch, "/bills/"+url.PathEscape(billID), tgUserID, nil, UpdateBillRequest{IsPaid: &paid}, &out)
	return out, err
}

// PlannedPurchases returns the member's personal items and all family items.
func (c *Client) PlannedPurchases(ctx context.Context, tgUserID int64) ([]PlannedPurchase, error) {
	var out []PlannedPurchase
	err := c.do(ctx, http.MethodGet, "/planned-purchases", tgUserID, nil, nil, &out)
	return out, err
}

// CreatePlannedPurchase adds an item to the family or the member's personal list.
func (c *Client) CreatePlannedPurchase(ctx context.Context, tgUserID int64, req CreatePlannedPurchaseRequest) (PlannedPurchase, error) {
	var out PlannedPurchase
	err := c.do(ctx, http.MethodPost, "/planned-purchases", tgUserID, nil, req, &out)
	return out, err
}

// SetPlannedPurchaseBought marks an item bought or not bought.
func (c *Client) SetPlannedPurchaseBought(ctx context.Context, tgUserID int64, id string, bought bool) (PlannedPurchase, error) {
	var out PlannedPurchase
	req := UpdatePlannedPurchaseRequest{IsBought: &bought}
	err := c.do(ctx, http.MethodPatch, "/planned-purchases/"+url.PathEscape(id), tgUserID, nil, req, &out)
	return out, err
}

// DeletePlannedPurchase removes an item from its list.
func (c *Client) DeletePlannedPurchase(ctx context.Context, tgUserID int64, id string) error {
	return c.do(ctx, http.MethodDelete, "/planned-purchases/"+url.PathEscape(id), tgUserID, nil, nil, nil)
}

// do sends one request. tgUserID 0 means a bot-level call with no acting member.
func (c *Client) do(ctx context.Context, method, path string, tgUserID int64, query url.Values, body, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s %s: %w", method, path, err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, path, err)
	}
	req.Header.Set(headerBotToken, c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tgUserID != 0 {
		req.Header.Set(headerTelegramUserID, strconv.FormatInt(tgUserID, 10))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return decodeError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

func decodeError(resp *http.Response) error {
	apiErr := &APIError{Status: resp.StatusCode}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err := json.Unmarshal(b, apiErr); err != nil || apiErr.Message == "" {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}
	return apiErr
}

// Error codes from the backend's ErrorResponse.
const (
	CodeTelegramNotLinked = "TELEGRAM_NOT_LINKED"
	CodeValidationFailed  = "VALIDATION_FAILED"
)

// APIError is a non-2xx response from the backend.
type APIError struct {
	Status      int               `json:"-"`
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	FieldErrors map[string]string `json:"fieldErrors"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("backend %d %s: %s", e.Status, e.Code, e.Message)
}

// IsNotLinked reports whether the Telegram account isn't linked to any member.
func IsNotLinked(err error) bool {
	var e *APIError
	return errors.As(err, &e) && e.Code == CodeTelegramNotLinked
}

// HasStatus reports whether err is an APIError with the given HTTP status.
func HasStatus(err error, status int) bool {
	var e *APIError
	return errors.As(err, &e) && e.Status == status
}
