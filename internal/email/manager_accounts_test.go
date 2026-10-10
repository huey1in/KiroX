package email

import (
	"maps"
	"path/filepath"
	"reflect"
	"testing"

	"reg_go/internal/storage"
)

func TestMailboxProviderIsolation(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"LOCALAPPDATA", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, root)
	}
	if dir := storage.GetDataDir(); dir != filepath.Join(root, "KiroX", "data") {
		t.Fatalf("test storage is not isolated: %s", dir)
	}
	t.Cleanup(storage.FlushAccountsSync)
	seed := []map[string]interface{}{
		{"email": "same@example.com", "registered": false},
		{"provider": "outlook", "email": "other@example.com", "registered": true},
		{"provider": "icloud", "email": "same@example.com", "registered": false, "messagesURL": "https://mail.test/messages"},
		{"provider": "icloud", "email": "cloud@example.com", "registered": true},
	}
	reset := func() {
		storage.ModifyAccountsCached(func([]map[string]interface{}) []map[string]interface{} {
			accounts := make([]map[string]interface{}, len(seed))
			for i, account := range seed {
				accounts[i] = maps.Clone(account)
			}
			return accounts
		})
	}
	cases := []struct {
		name string
		run  func(*testing.T)
	}{
		{"list", func(t *testing.T) {
			if got := GetOutlookAccounts(); len(got) != 2 || !isOutlookAccount(got[0]) || !isOutlookAccount(got[1]) {
				t.Fatalf("Outlook list includes other providers: %v", got)
			}
		}},
		{"import", func(t *testing.T) {
			data := "cloud@example.com----password----client----token"
			if result := AddOutlookAccounts(data); result["added"] != 1 || result["total"] != 3 {
				t.Fatalf("same-email iCloud account blocked Outlook import: %v", result)
			}
			if result := AddOutlookAccounts(data); result["added"] != 0 || result["total"] != 3 {
				t.Fatalf("duplicate Outlook import: %v", result)
			}
		}},
		{"delete", func(t *testing.T) {
			if result := DeleteOutlookAccount("same@example.com"); result["total"] != 1 {
				t.Fatalf("delete result = %v", result)
			}
		}},
		{"update", func(t *testing.T) {
			UpdateAccountStatus("same@example.com", true, true)
			if !GetOutlookAccounts()[0]["success"].(bool) {
				t.Fatal("legacy Outlook account was not updated")
			}
		}},
		{"clear", func(t *testing.T) {
			ClearOutlookAccounts()
			if len(GetOutlookAccounts()) != 0 {
				t.Fatal("Outlook accounts remain after clear")
			}
		}},
		{"clear registered", func(t *testing.T) {
			if result := ClearRegisteredOutlookAccounts(); result["removed"] != 1 || result["total"] != 1 {
				t.Fatalf("registered clear result = %v", result)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			before := GetICloudAccounts()
			tc.run(t)
			if after := GetICloudAccounts(); !reflect.DeepEqual(before, after) {
				t.Fatalf("Outlook action changed iCloud accounts: before=%v after=%v", before, after)
			}
		})
	}
	for _, action := range []struct {
		name string
		run  func()
	}{
		{"delete", func() { DeleteICloudAccount("same@example.com") }},
		{"mark", func() { MarkICloudAccountRegistered("same@example.com") }},
		{"clear", func() { ClearICloudAccounts() }},
	} {
		t.Run("iCloud "+action.name, func(t *testing.T) {
			reset()
			before := GetOutlookAccounts()
			action.run()
			if after := GetOutlookAccounts(); !reflect.DeepEqual(before, after) {
				t.Fatalf("iCloud action changed same-email Outlook account: %v", after)
			}
		})
	}
}
