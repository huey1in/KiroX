package data

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func successResult(email, token string) map[string]interface{} {
	return map[string]interface{}{
		"status": "success",
		"email":  email,
		"aws_token": map[string]interface{}{
			"refreshToken": token,
		},
	}
}

func saveConcurrently(t *testing.T, dir string, results []map[string]interface{}) {
	t.Helper()
	start := make(chan struct{})
	errors := make(chan error, len(results))
	var workers sync.WaitGroup
	for _, result := range results {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errors <- SaveKiroSuccess(result, dir)
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestSaveKiroSuccessConcurrentEmails(t *testing.T) {
	dir := t.TempDir()
	const count = 48
	results := make([]map[string]interface{}, count)
	for i := range results {
		results[i] = successResult(fmt.Sprintf("user%d@example.com", i), fmt.Sprintf("token%d", i))
	}
	saveConcurrently(t, dir, results)

	accounts, err := LoadAccounts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != count {
		t.Fatalf("saved %d accounts, want %d", len(accounts), count)
	}
	saved := make(map[string]interface{}, count)
	for _, account := range accounts {
		saved[account["email"].(string)] = account["refreshToken"]
	}
	for i := range results {
		email := fmt.Sprintf("user%d@example.com", i)
		if saved[email] != fmt.Sprintf("token%d", i) {
			t.Errorf("saved token for %s = %v", email, saved[email])
		}
	}
}

func TestSaveKiroSuccessConcurrentDuplicateEmails(t *testing.T) {
	dir := t.TempDir()
	const email = "shared@example.com"
	results := make([]map[string]interface{}, 32)
	for i := range results {
		results[i] = successResult(email, fmt.Sprintf("token%d", i))
	}
	saveConcurrently(t, dir, results)

	accounts, err := LoadAccounts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0]["email"] != email {
		t.Fatalf("saved accounts = %v, want one shared email", accounts)
	}
	if err := SaveKiroSuccess(successResult(email, "latest-token"), dir); err != nil {
		t.Fatal(err)
	}
	accounts, err = LoadAccounts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0]["refreshToken"] != "latest-token" {
		t.Fatalf("latest save did not replace shared email: %v", accounts)
	}
}

func TestSaveAndDeleteAccountsConcurrent(t *testing.T) {
	dir := t.TempDir()
	const count = 16
	for i := 0; i < count; i++ {
		if err := SaveKiroSuccess(successResult(fmt.Sprintf("old%d@example.com", i), "old"), dir); err != nil {
			t.Fatal(err)
		}
	}

	start := make(chan struct{})
	errors := make(chan error, count*2)
	var workers sync.WaitGroup
	for i := 0; i < count; i++ {
		workers.Add(2)
		go func() {
			defer workers.Done()
			<-start
			removed, err := DeleteAccount(dir, fmt.Sprintf("old%d@example.com", i))
			if err == nil && !removed {
				err = fmt.Errorf("old account %d was not removed", i)
			}
			errors <- err
		}()
		go func() {
			defer workers.Done()
			<-start
			errors <- SaveKiroSuccess(successResult(fmt.Sprintf("new%d@example.com", i), "new"), dir)
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}

	accounts, err := LoadAccounts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != count {
		t.Fatalf("saved %d accounts after concurrent deletion, want %d", len(accounts), count)
	}
	for _, account := range accounts {
		if account["refreshToken"] != "new" {
			t.Errorf("deleted account reappeared: %v", account)
		}
	}
}

func TestAccountOutputDirectoriesDoNotBlockEachOther(t *testing.T) {
	blockedDir, otherDir := t.TempDir(), t.TempDir()
	mu := accountsFileMutex(filepath.Join(blockedDir, "accounts.json"))
	mu.Lock()
	defer mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- SaveKiroSuccess(successResult("other@example.com", "token"), otherDir)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("saving another output directory waited on the blocked directory")
	}
}
