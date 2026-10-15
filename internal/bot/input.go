package bot

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxNoteLen = 1000
	maxNameLen = 255
)

// amountRe matches an amount at the start of a message, optionally followed by
// a note: "5000", "12 500,50 обед", "7.5 такси". Thousands may be grouped with
// spaces only in groups of three, so "5000 2 пиццы" is 5000 with a note.
var amountRe = regexp.MustCompile(`^(\d{1,3}(?:[ \x{00A0}\x{202F}]\d{3})+|\d{1,12})(?:[.,](\d{1,2}))?(?:\s+(.*))?$`)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var errBadAmount = errors.New("not a positive amount")

// parseAmountNote splits "12 500,50 обед" into the amount "12500.50" and the note "обед".
func parseAmountNote(text string) (json.Number, string, error) {
	m := amountRe.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return "", "", errBadAmount
	}
	whole := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, m[1])
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	amount := whole
	if frac := strings.TrimRight(m[2], "0"); frac != "" {
		amount += "." + frac
	}
	if amount == "0" {
		return "", "", errBadAmount
	}
	return json.Number(amount), cleanText(m[3], maxNoteLen), nil
}

// parseAmount parses a message that must be an amount and nothing else.
func parseAmount(text string) (json.Number, error) {
	amount, note, err := parseAmountNote(text)
	if err != nil {
		return "", err
	}
	if note != "" {
		return "", errBadAmount
	}
	return amount, nil
}

// cleanText drops control and invisible format characters, collapses
// whitespace and caps the length in runes.
func cleanText(s string, maxLen int) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > maxLen {
		s = strings.TrimSpace(string([]rune(s)[:maxLen]))
	}
	return s
}

// splitItems turns a message into shopping list items, one per non-empty line.
func splitItems(text string, limit int) []string {
	var items []string
	for _, line := range strings.Split(text, "\n") {
		if item := cleanText(line, maxNameLen); item != "" {
			items = append(items, item)
			if len(items) == limit {
				break
			}
		}
	}
	return items
}

func isUUID(s string) bool {
	return uuidRe.MatchString(s)
}
