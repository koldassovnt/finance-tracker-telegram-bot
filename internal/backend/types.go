package backend

import "encoding/json"

// Amounts travel as JSON numbers with up to four decimals (BigDecimal on the
// backend). json.Number keeps them exact instead of rounding through float64.

// Transaction types accepted by the backend.
const (
	TypeIncome   = "INCOME"
	TypeExpense  = "EXPENSE"
	TypeTransfer = "TRANSFER"
)

// Category kinds.
const (
	KindIncome  = "INCOME"
	KindExpense = "EXPENSE"
)

// AccessOwner marks a resource the caller owns, as opposed to one shared with them.
const AccessOwner = "OWNER"

// LinkRequest binds a Telegram account to the user who has that username.
type LinkRequest struct {
	TelegramUsername string `json:"telegramUsername"`
	TelegramUserID   int64  `json:"telegramUserId"`
	ChatID           int64  `json:"chatId"`
}

// LinkResponse identifies the user the Telegram account is now linked to.
type LinkResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

// DueBills is one user's unpaid bills for a date, with where to send them.
type DueBills struct {
	UserID         string    `json:"userId"`
	TelegramUserID int64     `json:"telegramUserId"`
	ChatID         int64     `json:"chatId"`
	DisplayName    string    `json:"displayName"`
	Bills          []DueBill `json:"bills"`
}

// DueBill is the short form of a bill in the reminder feed.
type DueBill struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Amount   json.Number `json:"amount"`
	Currency string      `json:"currency"`
	DueDate  string      `json:"dueDate"`
}

// Bill is a scheduled payment.
type Bill struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Amount   json.Number `json:"amount"`
	Currency string      `json:"currency"`
	DueDate  string      `json:"dueDate"`
	IsPaid   bool        `json:"isPaid"`
	Overdue  bool        `json:"overdue"`
	Access   string      `json:"access"`
}

// UpdateBillRequest is a partial update; nil fields are left unchanged.
type UpdateBillRequest struct {
	IsPaid *bool `json:"isPaid,omitempty"`
}

// Account is a money account (card, cash, deposit...).
type Account struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Currency string      `json:"currency"`
	Balance  json.Number `json:"balance"`
	Access   string      `json:"access"`
}

// Category classifies income and expenses; categories may nest one level via ParentID.
type Category struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	ParentID *string `json:"parentId"`
}

// CreateTransactionRequest records an income, expense or transfer.
type CreateTransactionRequest struct {
	Type        string      `json:"type"`
	AccountID   string      `json:"accountId"`
	Amount      json.Number `json:"amount"`
	OccurredOn  string      `json:"occurredOn"`
	CategoryID  string      `json:"categoryId,omitempty"`
	ToAccountID string      `json:"toAccountId,omitempty"`
	ToAmount    json.Number `json:"toAmount,omitempty"`
	Note        string      `json:"note,omitempty"`
}

// Transaction is a recorded transaction.
type Transaction struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	AccountID  string      `json:"accountId"`
	Amount     json.Number `json:"amount"`
	Currency   string      `json:"currency"`
	OccurredOn string      `json:"occurredOn"`
}

// MonthlySummary is income and expense totals for a month, in Currency.
type MonthlySummary struct {
	Month             string            `json:"month"`
	Currency          string            `json:"currency"`
	TotalIncome       json.Number       `json:"totalIncome"`
	TotalExpense      json.Number       `json:"totalExpense"`
	Net               json.Number       `json:"net"`
	ExpenseByCategory []CategorySummary `json:"expenseByCategory"`
}

// CategorySummary is one category's total in a MonthlySummary.
type CategorySummary struct {
	CategoryName *string     `json:"categoryName"`
	Total        json.Number `json:"total"`
}

// UserRef is a short reference to a family member.
type UserRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// PlannedPurchase is an item on the family or a personal shopping list.
type PlannedPurchase struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	IsFamily bool    `json:"isFamily"`
	IsBought bool    `json:"isBought"`
	AddedBy  UserRef `json:"addedBy"`
}

// CreatePlannedPurchaseRequest adds an item to a shopping list.
type CreatePlannedPurchaseRequest struct {
	Name     string `json:"name"`
	IsFamily bool   `json:"isFamily"`
}

// UpdatePlannedPurchaseRequest is a partial update; nil fields are left unchanged.
type UpdatePlannedPurchaseRequest struct {
	IsBought *bool `json:"isBought,omitempty"`
}
