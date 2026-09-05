package storage

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func isolateAccountsCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldDir := _dataDir
	_accountsMu.Lock()
	oldCache, oldLoaded, oldDirty, oldVersion := _accountsCache, _accountsLoaded, _accountsDirty, _accountsVersion
	if _flushTimer != nil {
		_flushTimer.Stop()
	}
	_accountsCache, _accountsLoaded, _accountsDirty, _accountsVersion, _flushTimer = nil, false, false, 0, nil
	_accountsMu.Unlock()
	_dataDir = dir
	_dataDirOnce = sync.Once{}
	_dataDirOnce.Do(func() {})
	t.Cleanup(func() {
		FlushAccountsSync()
		_accountsLoadMu.Lock()
		defer _accountsLoadMu.Unlock()
		_accountsFlushMu.Lock()
		defer _accountsFlushMu.Unlock()
		_accountsMu.Lock()
		_accountsCache, _accountsLoaded, _accountsDirty, _accountsVersion = oldCache, oldLoaded, oldDirty, oldVersion
		_accountsMu.Unlock()
		_dataDir = oldDir
		_dataDirOnce = sync.Once{}
	})
	return filepath.Join(dir, "accounts.json")
}

func TestAccountsCacheOwnsInputAndReturnedMaps(t *testing.T) {
	isolateAccountsCache(t)
	input := []map[string]interface{}{{"email": "user@example.com", "registered": false}}
	SetAccountsCached(input)
	input[0]["email"] = "mutated-input@example.com"
	snapshot := GetAccountsCached()
	snapshot[0]["email"] = "mutated-snapshot@example.com"
	ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		accounts[0]["registered"] = true
		return accounts
	})
	if snapshot[0]["registered"] != false {
		t.Fatal("cache mutation changed an earlier snapshot")
	}
	latest := GetAccountsCached()
	if latest[0]["email"] != "user@example.com" || latest[0]["registered"] != true {
		t.Fatalf("cache shares maps with caller: %v", latest)
	}
}

type blockingJSONValue struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (v *blockingJSONValue) MarshalJSON() ([]byte, error) {
	v.once.Do(func() { close(v.entered) })
	<-v.release
	return []byte(`"stored"`), nil
}

func waitForSignal(t *testing.T, ch <-chan struct{}, action string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", action)
	}
}

func TestAccountsFlushPreservesUpdatesDuringIO(t *testing.T) {
	path := isolateAccountsCache(t)
	gate := &blockingJSONValue{entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(gate.release) })
	SetAccountsCached([]map[string]interface{}{{"email": "user@example.com", "registered": false, "gate": gate}})
	flushed := make(chan struct{})
	go func() {
		FlushAccountsSync()
		close(flushed)
	}()
	waitForSignal(t, gate.entered, "flush to start writing")

	updated := make(chan struct{})
	go func() {
		ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
			accounts[0]["registered"] = true
			return accounts
		})
		close(updated)
	}()
	waitForSignal(t, updated, "memory update while disk I/O is blocked")
	_accountsMu.Lock()
	_flushTimer.Stop()
	_flushTimer = nil
	_accountsMu.Unlock()
	releaseOnce.Do(func() { close(gate.release) })
	waitForSignal(t, flushed, "first flush")
	_accountsMu.RLock()
	dirty := _accountsDirty
	_accountsMu.RUnlock()
	if !dirty {
		t.Fatal("old flush cleared the newer update's dirty flag")
	}
	first, err := LoadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if first[0]["registered"] != false {
		t.Fatalf("flush snapshot changed during encoding: %v", first)
	}
	FlushAccountsSync()
	latest, err := LoadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if latest[0]["registered"] != true {
		t.Fatalf("newer update was not persisted: %v", latest)
	}
}

func TestAccountsConcurrentFlushesPersistLatestSnapshot(t *testing.T) {
	path := isolateAccountsCache(t)
	gate := &blockingJSONValue{entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(gate.release) })
	SetAccountsCached([]map[string]interface{}{{"email": "user@example.com", "revision": 1, "gate": gate}})
	firstDone, secondDone := make(chan struct{}), make(chan struct{})
	go func() {
		FlushAccountsSync()
		close(firstDone)
	}()
	waitForSignal(t, gate.entered, "first flush to start writing")
	ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		accounts[0]["revision"] = 2
		return accounts
	})
	go func() {
		FlushAccountsSync()
		close(secondDone)
	}()
	releaseOnce.Do(func() { close(gate.release) })
	waitForSignal(t, firstDone, "first flush")
	waitForSignal(t, secondDone, "second flush")
	latest, err := LoadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if latest[0]["revision"] != float64(2) {
		t.Fatalf("concurrent flushes lost newest revision: %v", latest)
	}
}

func TestAccountsInitialLoadDoesNotBlockOrOverwriteReplacement(t *testing.T) {
	path := isolateAccountsCache(t)
	if err := SaveJSON(path, []map[string]interface{}{{"email": "old@example.com"}}); err != nil {
		t.Fatal(err)
	}
	fileMu := getFileMutex(path)
	fileMu.Lock()
	var unlockOnce sync.Once
	defer unlockOnce.Do(fileMu.Unlock)
	loaded := make(chan []map[string]interface{}, 1)
	go func() { loaded <- GetAccountsCached() }()
	deadline := time.Now().Add(5 * time.Second)
	for _accountsLoadMu.TryLock() {
		_accountsLoadMu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("initial load did not start")
		}
		time.Sleep(time.Millisecond)
	}
	replaced := make(chan struct{})
	go func() {
		SetAccountsCached([]map[string]interface{}{{"email": "new@example.com"}})
		close(replaced)
	}()
	waitForSignal(t, replaced, "cache replacement while initial disk read is blocked")
	unlockOnce.Do(fileMu.Unlock)
	select {
	case accounts := <-loaded:
		if len(accounts) != 1 || accounts[0]["email"] != "new@example.com" {
			t.Fatalf("initial disk load overwrote replacement: %v", accounts)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("initial load did not finish")
	}
}

func TestAccountsConcurrentReadersModifiersAndFlushes(t *testing.T) {
	path := isolateAccountsCache(t)
	const count = 32
	accounts := make([]map[string]interface{}, count)
	for i := range accounts {
		accounts[i] = map[string]interface{}{"index": i, "registered": false}
	}
	SetAccountsCached(accounts)
	var workers sync.WaitGroup
	for i := range accounts {
		workers.Add(3)
		go func() {
			defer workers.Done()
			ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
				accounts[i]["registered"] = true
				return accounts
			})
		}()
		go func() {
			defer workers.Done()
			if _, err := json.Marshal(GetAccountsCached()); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer workers.Done()
			FlushAccountsSync()
		}()
	}
	workers.Wait()
	FlushAccountsSync()
	saved, err := LoadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != count {
		t.Fatalf("saved %d accounts, want %d", len(saved), count)
	}
	for _, account := range saved {
		if account["registered"] != true {
			t.Errorf("status update was lost: %v", account)
		}
	}
}
