package bot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		in   json.Number
		want string
	}{
		{"150000.0000", "150 000"},
		{"12.5000", "12,50"},
		{"7.05", "7,05"},
		{"999", "999"},
		{"1000", "1 000"},
		{"-1234567.8900", "-1 234 567,89"},
		{"0.0000", "0"},
	}
	for _, tt := range tests {
		if got := formatAmount(tt.in); got != tt.want {
			t.Errorf("formatAmount(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
	if got := formatMoney("100", "GBP"); got != "100 GBP" {
		t.Errorf("formatMoney with unknown currency = %q", got)
	}
}

func TestWithoutButton(t *testing.T) {
	kb := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "a", CallbackData: "b:1"}},
		{{Text: "b", CallbackData: "b:2"}},
	}}
	got := withoutButton(kb, "b:1")
	if got == nil || len(got.InlineKeyboard) != 1 || got.InlineKeyboard[0][0].CallbackData != "b:2" {
		t.Fatalf("withoutButton kept %+v", got)
	}
	if withoutButton(got, "b:2") != nil {
		t.Error("withoutButton should return nil once no buttons are left")
	}
}

func TestReminderEscapesNames(t *testing.T) {
	d := backend.DueBills{Bills: []backend.DueBill{
		{ID: "11111111-1111-1111-1111-111111111111", Name: "Интернет <Beeline>", Amount: "7990.0000", Currency: "KZT"},
	}}
	text, kb := reminder(d, time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC))
	if !strings.Contains(text, "15 октября") || !strings.Contains(text, "Интернет &lt;Beeline&gt;: 7 990 ₸") {
		t.Errorf("unexpected reminder text:\n%s", text)
	}
	if data := kb.InlineKeyboard[0][0].CallbackData; len(data) > 64 || data != "b:11111111-1111-1111-1111-111111111111" {
		t.Errorf("callback data %q", data)
	}
}

func TestRenderPurchasesCallbackDataFits(t *testing.T) {
	items := []backend.PlannedPurchase{
		{ID: "22222222-2222-2222-2222-222222222222", Name: "Хлеб", IsFamily: true, IsBought: true},
		{ID: "33333333-3333-3333-3333-333333333333", Name: "Молоко", IsFamily: true},
		{ID: "44444444-4444-4444-4444-444444444444", Name: "Книга", IsFamily: false},
	}
	text, kb := renderPurchases(items)
	if strings.Index(text, "Молоко") > strings.Index(text, "Хлеб") {
		t.Error("unbought items should come before bought ones")
	}
	if strings.Index(text, "Книга") < strings.Index(text, "Личный") {
		t.Error("personal item rendered in the family section")
	}
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if len(btn.CallbackData) > 64 {
				t.Errorf("callback data %q exceeds 64 bytes", btn.CallbackData)
			}
		}
	}
}
