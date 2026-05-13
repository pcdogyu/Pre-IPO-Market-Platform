package i18n

import "testing"

func TestTReturnsLanguageSpecificMessage(t *testing.T) {
	tests := []struct {
		name string
		lang string
		key  string
		want string
	}{
		{name: "chinese", lang: "zh", key: "nav.dashboard", want: "工作台"},
		{name: "english", lang: "en", key: "nav.dashboard", want: "Dashboard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := T(tt.lang, tt.key); got != tt.want {
				t.Fatalf("T(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.want)
			}
		})
	}
}

func TestTFallsBackToChineseForUnsupportedLanguage(t *testing.T) {
	if got := T("fr", "nav.dashboard"); got != "工作台" {
		t.Fatalf("unsupported language fallback = %q, want %q", got, "工作台")
	}
}

func TestTReturnsKeyWhenMessageIsMissing(t *testing.T) {
	const key = "missing.key"
	if got := T("en", key); got != key {
		t.Fatalf("missing message fallback = %q, want %q", got, key)
	}
}
