package email

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"
)

func TestParseIMAPListMailbox(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantName  string
		wantFlags string
	}{
		{
			name:      "quoted junk folder",
			line:      `* LIST (\HasNoChildren \Junk) "/" "Junk Email"`,
			wantName:  "Junk Email",
			wantFlags: `\HasNoChildren \Junk`,
		},
		{
			name:      "atom folder name",
			line:      `* LIST (\Junk) "/" Junk`,
			wantName:  "Junk",
			wantFlags: `\Junk`,
		},
		{
			name:      "localized folder identified by special use flag",
			line:      `* LIST (\HasNoChildren \Junk) "/" "垃圾邮件"`,
			wantName:  "垃圾邮件",
			wantFlags: `\HasNoChildren \Junk`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, flags, ok := parseIMAPListMailbox(tt.line)
			if !ok {
				t.Fatal("parseIMAPListMailbox returned ok=false")
			}
			if name != tt.wantName {
				t.Fatalf("name = %q, want %q", name, tt.wantName)
			}
			if flags != tt.wantFlags {
				t.Fatalf("flags = %q, want %q", flags, tt.wantFlags)
			}
		})
	}
}

func TestQuoteIMAPMailbox(t *testing.T) {
	tests := []struct {
		mailbox string
		want    string
	}{
		{mailbox: "INBOX", want: "INBOX"},
		{mailbox: "inbox", want: "INBOX"},
		{mailbox: "Junk Email", want: `"Junk Email"`},
		{mailbox: `Junk "Email"\Archive`, want: `"Junk \"Email\"\\Archive"`},
	}

	for _, tt := range tests {
		if got := quoteIMAPMailbox(tt.mailbox); got != tt.want {
			t.Errorf("quoteIMAPMailbox(%q) = %q, want %q", tt.mailbox, got, tt.want)
		}
	}
}

func TestFindOTPGraphWithTokenFindsNewJunkMessage(t *testing.T) {
	var requestedJunkMessages atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/me/mailFolders/inbox":
			_ = json.NewEncoder(w).Encode(outlookGraphFolderResponse{TotalItemCount: 10})
		case "/me/mailFolders/junkemail":
			_ = json.NewEncoder(w).Encode(outlookGraphFolderResponse{TotalItemCount: 5})
		case "/me/mailFolders/junkemail/messages":
			requestedJunkMessages.Store(true)
			if got := r.URL.Query().Get("$top"); got != "1" {
				t.Errorf("$top = %q, want 1", got)
			}
			_ = json.NewEncoder(w).Encode(outlookGraphMessagesResponse{Value: []outlookGraphMessage{
				{Subject: "Your verification code is 482731"},
			}})
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	previousBaseURL := outlookGraphAPIBaseURL
	outlookGraphAPIBaseURL = server.URL
	t.Cleanup(func() { outlookGraphAPIBaseURL = previousBaseURL })

	code, err := findOTPGraphWithToken(
		"test-token",
		OutlookMailboxCounts{Inbox: 10, Junk: 4},
		regexp.MustCompile(`\b(\d{6})\b`),
		"",
	)
	if err != nil {
		t.Fatalf("findOTPGraphWithToken returned error: %v", err)
	}
	if code != "482731" {
		t.Fatalf("code = %q, want 482731", code)
	}
	if !requestedJunkMessages.Load() {
		t.Fatal("junk messages endpoint was not requested")
	}
}
