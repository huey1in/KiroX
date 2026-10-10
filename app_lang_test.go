package main

import "testing"

func TestMapLocaleToLang(t *testing.T) {
	cases := []struct {
		locale string
		want   string
	}{
		{"zh_CN.UTF-8", "zh"},
		{"ja-JP", "ja"},
		{"EN_US", "en"},
		{"en@", "en"},
		{"en.", "en"},
		{"ja_", "ja"},
		{"zh-", "zh"},
		{"", ""},
		{"de_DE.UTF-8", ""},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			if got := mapLocaleToLang(tc.locale); got != tc.want {
				t.Fatalf("mapLocaleToLang(%q) = %q, want %q", tc.locale, got, tc.want)
			}
		})
	}
}
