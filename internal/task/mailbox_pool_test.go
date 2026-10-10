package task

import (
	"path/filepath"
	"testing"

	"reg_go/internal/storage"
)

func TestOutlookTaskDoesNotCountICloudAccounts(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"LOCALAPPDATA", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, root)
	}
	if dir := storage.GetDataDir(); dir != filepath.Join(root, "KiroX", "data") {
		t.Fatalf("storage is not isolated: %s", dir)
	}
	t.Cleanup(storage.FlushAccountsSync)
	cases := []struct {
		name     string
		accounts []map[string]interface{}
		want     string
	}{
		{"only iCloud", []map[string]interface{}{{"provider": "icloud", "email": "cloud@example.com", "registered": false}}, "请先添加微软邮箱账号"},
		{"registered Outlook plus iCloud", []map[string]interface{}{
			{"email": "legacy@example.com", "registered": true},
			{"provider": "outlook", "email": "explicit@example.com", "registered": true},
			{"provider": "icloud", "email": "cloud@example.com", "registered": false},
		}, "没有可用的 Outlook 账号（所有账号已注册成功）"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage.ModifyAccountsCached(func([]map[string]interface{}) []map[string]interface{} { return tc.accounts })
			// Request more than the entire fixture so even the old selection path
			// rejects before launching a batch or performing any network request.
			result := StartTask(StartTaskRequest{Count: 10, EmailProvider: "outlook"})
			if result["error"] != tc.want {
				t.Fatalf("selection error = %v, want %s", result, tc.want)
			}
		})
	}
}
