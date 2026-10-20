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

func TestRenderBalance(t *testing.T) {
	accounts := []backend.Account{
		{ID: "55555555-5555-5555-5555-555555555555", Name: "Kaspi <Gold>", Currency: "KZT", Balance: "1250000.5000"},
		{ID: "66666666-6666-6666-6666-666666666666", Name: "Наличные $", Currency: "USD", Balance: "-20.0000"},
	}

	_, kb := renderBalancePicker(accounts)
	var data []string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if len(btn.CallbackData) > 64 {
				t.Errorf("callback data %q exceeds 64 bytes", btn.CallbackData)
			}
			data = append(data, btn.CallbackData)
		}
	}
	if len(data) != 2 || data[0] != "bal:"+accounts[0].ID || data[1] != "bal:"+accounts[1].ID {
		t.Errorf("picker callback data = %q", data)
	}

	now := time.Date(2026, 10, 20, 9, 5, 0, 0, time.UTC)
	text, kb := renderBalance(accounts[0], true, now)
	for _, want := range []string{"Kaspi &lt;Gold&gt;", "1 250 000,50 ₸", "20 октября, 09:05"} {
		if !strings.Contains(text, want) {
			t.Errorf("balance text missing %q:\n%s", want, text)
		}
	}
	if row := kb.InlineKeyboard[0]; len(row) != 2 || row[0].CallbackData != "bal:"+accounts[0].ID || row[1].CallbackData != "bal:all" {
		t.Errorf("balance buttons = %+v", row)
	}
	if text, kb := renderBalance(accounts[1], false, now); !strings.Contains(text, "-20 $") || len(kb.InlineKeyboard[0]) != 1 {
		t.Errorf("single-account balance: text %q, buttons %+v", text, kb.InlineKeyboard)
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

func TestTopicAndRateText(t *testing.T) {
	topic := backend.Topic{ID: "77777777-7777-7777-7777-777777777777", Name: "Турция <2026>", Spent: "350000.0000", PlannedAmount: "500000.0000", Remaining: "150000.0000"}
	header := topicHeader(topic)
	for _, want := range []string{"Турция &lt;2026&gt;", "Потрачено: 350\u00a0000 ₸ из 500\u00a0000 ₸", "Осталось: 150\u00a0000 ₸"} {
		if !strings.Contains(header, want) {
			t.Errorf("topic header missing %q:\n%s", want, header)
		}
	}
	if h := topicHeader(backend.Topic{Name: "Ремонт", Spent: "0.0000"}); strings.Contains(h, " из ") || strings.Contains(h, "Осталось") {
		t.Errorf("topic header without a plan mentions one:\n%s", h)
	}
	if data := topicsKeyboard([]backend.Topic{topic}).InlineKeyboard[0][0].CallbackData; data != "tp:"+topic.ID {
		t.Errorf("topic callback data %q", data)
	}

	acc := backend.Account{ID: "88888888-8888-8888-8888-888888888888", Name: "Наличные", Currency: "USD"}
	sess := session{txType: backend.TypeExpense, amount: "25", account: &acc, rate: "478.5", topic: &topic}
	draft := draftText(sess)
	for _, want := range []string{"25 $", "Курс: 1 USD = 478,50 ₸", "🧳 Турция &lt;2026&gt;"} {
		if !strings.Contains(draft, want) {
			t.Errorf("draft missing %q:\n%s", want, draft)
		}
	}

	sess.marketRate = "478.12"
	kb := rateKeyboard(sess, "19.10")
	if btn := kb.InlineKeyboard[0][0]; btn.CallbackData != cbRate || btn.Text != "📈 1 USD = 478,12 ₸ (на 19.10)" {
		t.Errorf("market rate button = %+v", btn)
	}
	sess.marketRate = ""
	if kb := rateKeyboard(sess, ""); len(kb.InlineKeyboard) != 1 || kb.InlineKeyboard[0][0].CallbackData != cbCancel {
		t.Errorf("without a market rate the keyboard should only cancel: %+v", kb.InlineKeyboard)
	}
}
