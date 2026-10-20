package bot

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

// step is what the bot is waiting for from the user in a multi-step dialog.
type step int

const (
	stepNone         step = iota
	stepAmount            // text: "amount [note]"
	stepAccount           // button: source account
	stepToAccount         // button: transfer destination
	stepToAmount          // text: amount received, for a cross-currency transfer
	stepCategory          // button: category
	stepPurchaseName      // text: shopping list items, one per line
	stepTopic             // button: topic for an expense
	stepRate              // text or button: KZT rate of a foreign-currency account
)

// session is one user's dialog in progress. Sessions live in memory only; a
// restart drops unfinished dialogs, which the user simply starts again.
type session struct {
	step step

	// Transaction dialog.
	txType     string
	amount     json.Number
	note       string
	accounts   []backend.Account
	categories []backend.Category
	account    *backend.Account
	toAccount  *backend.Account
	category   *backend.Category
	toAmount   json.Number
	topics     []backend.Topic
	topic      *backend.Topic
	// rate is KZT per unit of account's currency, set only for a non-KZT
	// account; marketRate is the stored rate offered as a button for it.
	rate       json.Number
	marketRate json.Number

	// Shopping list dialog.
	isFamily bool

	updatedAt time.Time
}

// sessions holds dialogs by Telegram user id. Updates are handled
// concurrently, so it hands out copies rather than shared pointers.
type sessions struct {
	mu  sync.Mutex
	m   map[int64]session
	ttl time.Duration
}

func newSessions(ttl time.Duration) *sessions {
	return &sessions{m: make(map[int64]session), ttl: ttl}
}

// get returns the user's dialog, or a zero session if there is none or it expired.
func (s *sessions) get(userID int64) session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[userID]
	if !ok || time.Since(sess.updatedAt) > s.ttl {
		delete(s.m, userID)
		return session{}
	}
	return sess
}

func (s *sessions) set(userID int64, sess session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess.updatedAt = time.Now()
	s.m[userID] = sess
}

func (s *sessions) clear(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, userID)
}

// sweep drops expired dialogs so abandoned ones don't accumulate.
func (s *sessions) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.m {
		if time.Since(sess.updatedAt) > s.ttl {
			delete(s.m, id)
		}
	}
}
