package bot

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestParseAmountNote(t *testing.T) {
	tests := []struct {
		in         string
		wantAmount json.Number
		wantNote   string
		wantErr    bool
	}{
		{in: "5000", wantAmount: "5000"},
		{in: "5000 обед", wantAmount: "5000", wantNote: "обед"},
		{in: "12 500,50 кафе с друзьями", wantAmount: "12500.5", wantNote: "кафе с друзьями"},
		{in: "12 500", wantAmount: "12500"},
		{in: "1 200 300", wantAmount: "1200300"},
		{in: "7.05 такси", wantAmount: "7.05", wantNote: "такси"},
		{in: "5000 2 пиццы", wantAmount: "5000", wantNote: "2 пиццы"},
		{in: "  007  ", wantAmount: "7"},
		{in: "10.00", wantAmount: "10"},
		{in: "0", wantErr: true},
		{in: "0,00", wantErr: true},
		{in: "-5", wantErr: true},
		{in: "обед 5000", wantErr: true},
		{in: "5.123", wantErr: true},
		{in: "1e5", wantErr: true},
		{in: "", wantErr: true},
		{in: "1234567890123", wantErr: true},
	}
	for _, tt := range tests {
		amount, note, err := parseAmountNote(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseAmountNote(%q) = %q, %q; want error", tt.in, amount, note)
			}
			continue
		}
		if err != nil || amount != tt.wantAmount || note != tt.wantNote {
			t.Errorf("parseAmountNote(%q) = %q, %q, %v; want %q, %q", tt.in, amount, note, err, tt.wantAmount, tt.wantNote)
		}
	}
}

func TestParseAmountRejectsNote(t *testing.T) {
	if _, err := parseAmount("47000 тенге"); err == nil {
		t.Error("parseAmount accepted trailing text")
	}
	if got, err := parseAmount("47 000"); err != nil || got != "47000" {
		t.Errorf("parseAmount(47 000) = %q, %v", got, err)
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct{ in, want string }{
		{"  Молоко \t 2 л ", "Молоко 2 л"},
		{"hidden" + string(rune(0x200b)) + "char\x00s", "hiddenchars"},
		{"line\nbreak", "line break"},
		{"абвгд", "абв"},
	}
	for _, tt := range tests {
		maxLen := 255
		if tt.in == "абвгд" {
			maxLen = 3
		}
		if got := cleanText(tt.in, maxLen); got != tt.want {
			t.Errorf("cleanText(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestSplitItems(t *testing.T) {
	got := splitItems("Молоко\n\n  Хлеб  \r\nЯйца\nСыр", 3)
	want := []string{"Молоко", "Хлеб", "Яйца"}
	if !slices.Equal(got, want) {
		t.Errorf("splitItems = %q; want %q", got, want)
	}
}

func TestCommand(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"/start", "start", true},
		{"/Start@family_bot payload", "start", true},
		{"/", "", false},
		{"start", "", false},
	}
	for _, tt := range tests {
		got, ok := command(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("command(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestParseRate(t *testing.T) {
	tests := []struct {
		in      string
		want    json.Number
		wantErr bool
	}{
		{in: "478", want: "478"},
		{in: "478,5", want: "478.5"},
		{in: " 478.120000 ", want: "478.12"},
		{in: "0,0213", want: "0.0213"},
		{in: "005.50", want: "5.5"},
		{in: "0", wantErr: true},
		{in: "0,000000", wantErr: true},
		{in: "-478", wantErr: true},
		{in: "1,1234567", wantErr: true},
		{in: "478 тенге", wantErr: true},
		{in: "1 000", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseRate(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseRate(%q) = %q; want error", tt.in, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("parseRate(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}

	// Stored market rates may carry more places than the backend keeps.
	if got, err := normalizeRate("478.1234567"); err != nil || got != "478.123457" {
		t.Errorf("normalizeRate rounds to %q, %v; want 478.123457", got, err)
	}
	if _, err := normalizeRate("0.0000001"); err == nil {
		t.Error("normalizeRate should reject a rate that rounds to zero")
	}
}
