package i18n

import "testing"

func TestDetectLocalePrecedenceAndFallback(t *testing.T) {
	tests := []struct {
		name     string
		language string
		lcAll    string
		messages string
		lang     string
		want     Locale
	}{
		{name: "default", want: English},
		{name: "simplified", lang: "zh_CN.UTF-8", want: SimplifiedChinese},
		{name: "traditional", lang: "zh_TW.UTF-8", want: TraditionalChinese},
		{name: "lc messages", messages: "zh_CN.UTF-8", lang: "en_US.UTF-8", want: SimplifiedChinese},
		{name: "lc all", lcAll: "en_US.UTF-8", messages: "zh_CN.UTF-8", lang: "zh_CN.UTF-8", want: English},
		{name: "language", language: "zh_TW:zh_CN", lang: "en_US.UTF-8", want: TraditionalChinese},
		{name: "unsupported", lang: "fr_FR.UTF-8", want: English},
		{name: "posix", language: "zh_CN", lang: "C", want: English},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LANGUAGE", tc.language)
			t.Setenv("LC_ALL", tc.lcAll)
			t.Setenv("LC_MESSAGES", tc.messages)
			t.Setenv("LANG", tc.lang)
			if got := Detect(); got != tc.want {
				t.Fatalf("Detect() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCatalogFallsBackToEnglish(t *testing.T) {
	if got := Locale("unsupported").T(ManagedTitle); got != catalogs[English][ManagedTitle] {
		t.Fatalf("unexpected fallback: %q", got)
	}
}
