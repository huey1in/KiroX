package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func isolateStorageLayout(t *testing.T) (string, string) {
	t.Helper()
	localBase := filepath.Join(t.TempDir(), "local")
	configBase := filepath.Join(t.TempDir(), "roaming")
	oldLocalResolver := localDataBaseResolver
	oldConfigResolver := configBaseResolver
	localDataBaseResolver = func() (string, error) { return localBase, nil }
	configBaseResolver = func() (string, error) { return configBase, nil }
	resetStorageGlobalsForTest()
	t.Cleanup(func() {
		localDataBaseResolver = oldLocalResolver
		configBaseResolver = oldConfigResolver
		resetStorageGlobalsForTest()
	})
	return localBase, configBase
}

func resetStorageGlobalsForTest() {
	layoutOnce = sync.Once{}
	layoutErr = nil
	_dataDir = ""
	_dataDirOnce = sync.Once{}
	_resultOutputDir = ""
	_resultOutputOnce = sync.Once{}
	_proxy = ""
	_proxyOnce = sync.Once{}
	_language = ""
	_languageOnce = sync.Once{}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}

func TestLegacyRoamingLayoutMigratesToLocalAppData(t *testing.T) {
	localBase, configBase := isolateStorageLayout(t)
	legacyRoot := filepath.Join(configBase, "kirox")
	resultDir := filepath.Join(t.TempDir(), "results")
	writeTestFile(t, filepath.Join(legacyRoot, "storage.conf"),
		"result_output_dir="+resultDir+"\nproxy=127.0.0.1:8080\nlanguage=ja\n")

	legacyFiles := map[string]string{
		"accounts.dat":    "accounts",
		"moemail.dat":     "moemail",
		"cloudmail.dat":   "cloudmail",
		"mailnest.dat":    "mailnest",
		"proxy_pool.json": "proxy-pool",
		"identities.dat":  "identities",
	}
	for name, content := range legacyFiles {
		writeTestFile(t, filepath.Join(legacyRoot, name), content)
	}

	wantRoot := filepath.Join(localBase, "KiroX")
	wantDataDir := filepath.Join(wantRoot, "data")
	if got := GetDataDir(); got != wantDataDir {
		t.Fatalf("GetDataDir() = %q, want %q", got, wantDataDir)
	}
	for name, want := range map[string]string{
		"accounts.json":   "accounts",
		"moemail.json":    "moemail",
		"cloudmail.json":  "cloudmail",
		"mailnest.json":   "mailnest",
		"proxy_pool.json": "proxy-pool",
	} {
		assertFileContent(t, filepath.Join(wantDataDir, name), want)
	}
	assertFileContent(t, filepath.Join(wantRoot, "cache", "identities.json"), "identities")
	assertFileContent(t, filepath.Join(legacyRoot, "accounts.dat"), "accounts")

	data, err := os.ReadFile(filepath.Join(wantRoot, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings settingsFile
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.SchemaVersion != settingsSchemaVersion || settings.DataDir != "" ||
		settings.ResultOutputDir != resultDir || settings.Proxy != "127.0.0.1:8080" || settings.Language != "ja" {
		t.Fatalf("unexpected migrated settings: %+v", settings)
	}
}

func TestLegacyCustomDataDirIsRetainedAndNormalized(t *testing.T) {
	localBase, configBase := isolateStorageLayout(t)
	customDir := filepath.Join(t.TempDir(), "custom-data")
	writeTestFile(t, filepath.Join(configBase, "kirox", "storage.conf"), "data_dir="+customDir+"\n")
	writeTestFile(t, filepath.Join(customDir, "moemail.dat"), "custom-moemail")
	writeTestFile(t, filepath.Join(customDir, "identities.dat"), "custom-identities")

	if got := GetDataDir(); got != customDir {
		t.Fatalf("GetDataDir() = %q, want %q", got, customDir)
	}
	assertFileContent(t, filepath.Join(customDir, "moemail.json"), "custom-moemail")
	assertFileContent(t, filepath.Join(localBase, "KiroX", "cache", "identities.json"), "custom-identities")
	settings := readSettingsFile()
	if settings.DataDir != customDir || settings.SchemaVersion != settingsSchemaVersion {
		t.Fatalf("unexpected migrated settings: %+v", settings)
	}
}

func TestSchemaTwoSettingsGainRecommendedRuntimeDefaults(t *testing.T) {
	localBase, _ := isolateStorageLayout(t)
	path := filepath.Join(localBase, "KiroX", "settings.json")
	writeTestFile(t, path, `{"schemaVersion":2,"language":"en"}`)

	settings := GetAppSettings()
	if settings.DefaultCount != 1 || settings.DefaultConcurrency != 1 || settings.DefaultDelay != 1 {
		t.Fatalf("unexpected task defaults: %+v", settings)
	}
	if settings.EmailProxyMode != "follow-task" || settings.OTPTimeoutSeconds != 120 || !settings.StopOnRisk {
		t.Fatalf("unexpected resilience defaults: %+v", settings)
	}
	if settings.FingerprintAlgorithm != "balanced" {
		t.Fatalf("unexpected fingerprint algorithm: %q", settings.FingerprintAlgorithm)
	}
	if got := fmt.Sprint(settings.FingerprintOffsets); got != "[0 0 0 15 100]" {
		t.Fatalf("unexpected fingerprint offsets: %s", got)
	}
	if settings.Language != "en" || settings.Theme != "system" || !settings.AutoCheckUpdates {
		t.Fatalf("legacy language or interface defaults were not migrated: %+v", settings)
	}
}

func TestSaveAppSettingsNormalizesBoundsAndRejectsInvalidEndpoint(t *testing.T) {
	isolateStorageLayout(t)
	settings := DefaultAppSettings()
	settings.DefaultCount = 999
	settings.DefaultConcurrency = 0
	settings.SoundVolume = 150
	settings.EmailProxyMode = "custom"
	settings.EmailProxy = "127.0.0.1:1080"
	settings.FingerprintAlgorithm = "unknown"

	saved, err := SaveAppSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DefaultCount != 1 || saved.DefaultConcurrency != 1 || saved.SoundVolume != 70 {
		t.Fatalf("numeric settings were not normalized: %+v", saved)
	}
	if saved.EmailProxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("custom email proxy = %q", saved.EmailProxy)
	}
	if saved.FingerprintAlgorithm != "balanced" {
		t.Fatalf("fingerprint algorithm = %q", saved.FingerprintAlgorithm)
	}

	settings = saved
	settings.FingerprintOffsets = []int{-10, 25, 50, 75, 120}
	saved, err = SaveAppSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(saved.FingerprintOffsets); got != "[0 25 50 75 100]" || saved.FingerprintAlgorithm != "custom" {
		t.Fatalf("custom fingerprint curve was not normalized: %s (%s)", got, saved.FingerprintAlgorithm)
	}

	settings = saved
	settings.OIDCBase = "not a URL"
	if _, err := SaveAppSettings(settings); err == nil {
		t.Fatal("invalid advanced endpoint was accepted")
	}
}

func TestPersistentLogsRedactCredentials(t *testing.T) {
	isolateStorageLayout(t)
	ConfigurePersistentLogs(true, 7)
	t.Cleanup(func() {
		persistentLogMu.Lock()
		persistentLogEnabled = false
		persistentLogMu.Unlock()
	})

	AppendPersistentLog("验证码: 123456 accessToken=secret-token\n")
	data, err := os.ReadFile(filepath.Join(GetLogsDir(), time.Now().Format("2006-01-02")+".log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "123456") || strings.Contains(text, "secret-token") {
		t.Fatalf("persistent log contains credentials: %q", text)
	}
	if !strings.Contains(text, "<redacted>") {
		t.Fatalf("persistent log does not contain redaction marker: %q", text)
	}
}

func TestMigrateDataCopiesAllBusinessFilesWithoutOverwriting(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()
	writeTestFile(t, filepath.Join(oldDir, "accounts.dat"), "old-accounts")
	writeTestFile(t, filepath.Join(oldDir, "moemail.dat"), "moemail")
	writeTestFile(t, filepath.Join(oldDir, "cloudmail.dat"), "cloudmail")
	writeTestFile(t, filepath.Join(oldDir, "mailnest.dat"), "mailnest")
	writeTestFile(t, filepath.Join(oldDir, "proxy_pool.json"), "proxy-pool")
	writeTestFile(t, filepath.Join(newDir, "accounts.json"), "existing-accounts")

	migrated, err := migrateData(oldDir, newDir)
	if err != nil {
		t.Fatal(err)
	}
	if migrated != 4 {
		t.Fatalf("migrated %d files, want 4", migrated)
	}
	assertFileContent(t, filepath.Join(newDir, "accounts.json"), "existing-accounts")
	for name, want := range map[string]string{
		"moemail.json": "moemail", "cloudmail.json": "cloudmail",
		"mailnest.json": "mailnest", "proxy_pool.json": "proxy-pool",
	} {
		assertFileContent(t, filepath.Join(newDir, name), want)
	}
	assertFileContent(t, filepath.Join(oldDir, "moemail.dat"), "moemail")
}

func TestSetDataDirReloadsAccountsFromNewDirectory(t *testing.T) {
	isolateStorageLayout(t)
	resetAccountsCache()
	t.Cleanup(resetAccountsCache)

	oldDir := GetDataDir()
	SetAccountsCached([]map[string]interface{}{{"email": "old@example.com"}})
	FlushAccountsSync()

	newDir := t.TempDir()
	if err := SaveJSON(filepath.Join(newDir, "accounts.json"), []map[string]interface{}{{"email": "new@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := SetDataDirPath(newDir); err != nil {
		t.Fatal(err)
	}
	accounts := GetAccountsCached()
	if len(accounts) != 1 || accounts[0]["email"] != "new@example.com" {
		t.Fatalf("account cache was not reloaded from new directory: %v", accounts)
	}
	assertFileContent(t, filepath.Join(oldDir, "accounts.json"), `[{"email":"old@example.com"}]`)
}

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
