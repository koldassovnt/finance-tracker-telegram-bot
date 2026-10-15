package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientSendsBotHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Bot-Token"); got != "secret" {
			t.Errorf("X-Bot-Token = %q", got)
		}
		if got := r.Header.Get("X-Telegram-User-Id"); got != "42" {
			t.Errorf("X-Telegram-User-Id = %q", got)
		}
		if r.URL.Path != "/api/v1/bills" || r.URL.Query().Get("unpaid") != "true" || r.URL.Query().Get("scope") != "OWN" {
			t.Errorf("unexpected request %s", r.URL)
		}
		_, _ = w.Write([]byte(`[{"id":"b1","name":"Rent","amount":150000.0000,"currency":"KZT","dueDate":"2026-10-15","isPaid":false,"overdue":false,"access":"OWNER"}]`))
	}))
	defer srv.Close()

	bills, err := New(srv.URL, "secret").UnpaidBills(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(bills) != 1 || bills[0].Amount != "150000.0000" {
		t.Errorf("bills = %+v; amount must stay exact", bills)
	}
}

func TestBotLevelCallsOmitUserHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Header["X-Telegram-User-Id"]; ok {
			t.Error("bills-due must not act as a user")
		}
		if got := r.URL.Query().Get("date"); got != "2026-10-15" {
			t.Errorf("date = %q", got)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "secret").BillsDue(context.Background(), time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
}

func TestLinkSendsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req LinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req != (LinkRequest{TelegramUsername: "ivan", TelegramUserID: 7, ChatID: 7}) {
			t.Errorf("link body = %+v", req)
		}
		_, _ = w.Write([]byte(`{"userId":"u1","displayName":"Иван","role":"MEMBER"}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, "secret").Link(context.Background(), LinkRequest{TelegramUsername: "ivan", TelegramUserID: 7, ChatID: 7})
	if err != nil || got.DisplayName != "Иван" {
		t.Fatalf("Link = %+v, %v", got, err)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		notLinked  bool
		wantStatus int
	}{
		{"not linked", http.StatusForbidden, `{"code":"TELEGRAM_NOT_LINKED","message":"not linked","fieldErrors":{}}`, true, http.StatusForbidden},
		{"forbidden", http.StatusForbidden, `{"code":"FORBIDDEN","message":"no","fieldErrors":{}}`, false, http.StatusForbidden},
		{"not json", http.StatusBadGateway, `<html>bad gateway</html>`, false, http.StatusBadGateway},
	}
	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tt.status)
			_, _ = w.Write([]byte(tt.body))
		}))
		_, err := New(srv.URL, "secret").Accounts(context.Background(), 1)
		srv.Close()

		if err == nil {
			t.Errorf("%s: want error", tt.name)
			continue
		}
		if IsNotLinked(err) != tt.notLinked {
			t.Errorf("%s: IsNotLinked = %v", tt.name, !tt.notLinked)
		}
		if !HasStatus(err, tt.wantStatus) {
			t.Errorf("%s: HasStatus(%d) false for %v", tt.name, tt.wantStatus, err)
		}
	}
}

func TestCreateTransactionOmitsEmptyFields(t *testing.T) {
	b, err := json.Marshal(CreateTransactionRequest{Type: TypeExpense, AccountID: "a", Amount: "5000", OccurredOn: "2026-10-15"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"EXPENSE","accountId":"a","amount":5000,"occurredOn":"2026-10-15"}`
	if string(b) != want {
		t.Errorf("got %s; want %s", b, want)
	}
}
