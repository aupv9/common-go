package middleware

import (
	"net/url"
	"strings"
	"testing"
)

func TestRedactQuery(t *testing.T) {
	cases := []struct {
		name string
		in   string
		// các chuỗi PHẢI không xuất hiện trong output
		mustNotContain []string
		// các chuỗi phải còn (param không nhạy cảm giữ nguyên)
		mustContain []string
	}{
		{"token redacted", "token=abc123&page=2", []string{"abc123"}, []string{"page=2"}},
		{"case-insensitive", "Access_Token=xyz", []string{"xyz"}, nil},
		{"api_key redacted", "api_key=KKK&q=hello", []string{"KKK"}, []string{"q=hello"}},
		{"no sensitive", "page=1&size=20", nil, []string{"page=1", "size=20"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactQuery(tc.in)
			for _, s := range tc.mustNotContain {
				if strings.Contains(got, s) {
					t.Errorf("output %q should not contain %q", got, s)
				}
			}
			for _, s := range tc.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output %q should contain %q", got, s)
				}
			}
		})
	}
}

func TestRedactQueryMalformed(t *testing.T) {
	// query không parse được → "[redacted]" cho an toàn.
	if got := redactQuery("%zz=bad"); got != "[redacted]" {
		t.Errorf("malformed query = %q, want [redacted]", got)
	}
}

func TestRedactQueryPreservesValues(t *testing.T) {
	got := redactQuery("token=secret&name=john")
	v, _ := url.ParseQuery(got)
	if v.Get("token") != "***" {
		t.Errorf("token = %q, want ***", v.Get("token"))
	}
	if v.Get("name") != "john" {
		t.Errorf("name = %q, want john", v.Get("name"))
	}
}
