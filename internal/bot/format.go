package bot

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var currencySymbols = map[string]string{
	"KZT": "₸",
	"USD": "$",
	"EUR": "€",
	"RUB": "₽",
}

var monthsNominative = [...]string{
	"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
	"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

var monthsGenitive = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// formatMoney renders 150000.0000 KZT as "150 000 ₸".
func formatMoney(amount json.Number, currency string) string {
	sym, ok := currencySymbols[currency]
	if !ok {
		sym = currency
	}
	return formatAmount(amount) + " " + sym
}

// formatAmount groups thousands with non-breaking spaces and drops a zero
// fraction: "150000.0000" → "150 000", "12.5000" → "12.50".
func formatAmount(amount json.Number) string {
	s := amount.String()
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	whole, frac, _ := strings.Cut(s, ".")
	frac = strings.TrimRight(frac, "0")
	if frac != "" && len(frac) < 2 {
		frac += "0"
	}

	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	if frac != "" {
		b.WriteString("," + frac)
	}
	return sign + b.String()
}

// formatDay renders a YYYY-MM-DD date as "05.10"; anything unparsable is returned as is.
func formatDay(date string) string {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return t.Format("02.01")
}

// formatLongDay renders a date as "15 октября".
func formatLongDay(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), monthsGenitive[t.Month()-1])
}

// formatMonth renders a month as "Октябрь 2026".
func formatMonth(t time.Time) string {
	return fmt.Sprintf("%s %d", monthsNominative[t.Month()-1], t.Year())
}
