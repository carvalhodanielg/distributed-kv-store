package store

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/carvalhodanielg/kvstore/internal/constants"
	bolt "go.etcd.io/bbolt"
)

// setupTestDB cria um banco de dados temporário para testes
func setupTestDB(t *testing.T) *bolt.DB {
	dbPath := "test_store.db"

	// Remove arquivo se existir
	os.Remove(dbPath)

	db, err := bolt.Open(dbPath, constants.DBFilePermission, nil)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(constants.BucketStore))
		return err
	})

	if err != nil {
		t.Fatalf("failed to create bucket in test db: %v", err)
	}

	return db
}

// cleanupTestDB remove o banco de dados de teste
func cleanupTestDB(t *testing.T, db *bolt.DB) {
	if db != nil {
		db.Close()
	}
	os.Remove("test_store.db")
}

func TestNewKVStore(t *testing.T) {
	store := NewKVStore()

	if store == nil {
		t.Fatal("NewKVStore() returned nil")
	}

	if store.store == nil {
		t.Fatal("store map is nil")
	}

	if store.watchers == nil {
		t.Fatal("watchers map is nil")
	}

	if len(store.store) != 0 {
		t.Fatal("store should be empty initially")
	}

	if len(store.watchers) != 0 {
		t.Fatal("watchers should be empty initially")
	}
}

func TestKVStore_Put(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	Init(db)
	store := NewKVStore()

	tests := []struct {
		key   string
		value string
	}{
		{"key1", "value1"},
		{"key2", "value2"},
		{"", "empty_key"},
		{"empty_value", ""},
		{"special_chars", "!@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			store.Put(tt.key, tt.value)

			// Verifica se foi salvo na memória
			if store.store[tt.key] != tt.value {
				t.Errorf("Put() failed to store in memory. Expected %s, got %s", tt.value, store.store[tt.key])
			}

			// Verifica se foi salvo no banco
			db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(constants.BucketStore))
				if b == nil {
					t.Fatal("bucket not found")
				}

				storedValue := b.Get([]byte(tt.key))
				if string(storedValue) != tt.value {
					t.Errorf("Put() failed to store in database. Expected %s, got %s", tt.value, string(storedValue))
				}
				return nil
			})
		})
	}
}

func TestKVStore_Get(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	Init(db)
	store := NewKVStore()

	// Testa chave inexistente
	value := store.Get("nonexistent")
	if value != "" {
		t.Errorf("Get() for nonexistent key should return empty string, got %s", value)
	}

	// Testa chaves existentes
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"":     "empty_key",
	}

	for key, expectedValue := range testData {
		store.Put(key, expectedValue)
		actualValue := store.Get(key)
		if actualValue != expectedValue {
			t.Errorf("Get() failed. Expected %s, got %s", expectedValue, actualValue)
		}
	}
}

func TestKVStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	Init(db)
	store := NewKVStore()

	// Adiciona dados de teste
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range testData {
		store.Put(key, value)
	}

	// Testa deleção de chave existente
	store.Delete("key1")

	// Verifica se foi removido da memória
	if store.store["key1"] != "" {
		t.Error("Delete() failed to remove from memory")
	}

	// Verifica se foi removido do banco
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(constants.BucketStore))
		if b == nil {
			t.Fatal("bucket not found")
		}

		storedValue := b.Get([]byte("key1"))
		if storedValue != nil {
			t.Error("Delete() failed to remove from database")
		}
		return nil
	})

	// Verifica se outras chaves ainda existem
	if store.Get("key2") != "value2" {
		t.Error("Delete() removed wrong key")
	}

	// Testa deleção de chave inexistente (não deve causar erro)
	store.Delete("nonexistent")
}

func TestKVStore_GetAll(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	Init(db)
	store := NewKVStore()

	// Testa store vazio
	all := store.GetAll()
	if len(all) != 0 {
		t.Errorf("GetAll() on empty store should return empty map, got %v", all)
	}

	// Adiciona dados de teste
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range testData {
		store.Put(key, value)
	}

	// Testa GetAll com dados
	all = store.GetAll()
	if len(all) != len(testData) {
		t.Errorf("GetAll() returned wrong number of items. Expected %d, got %d", len(testData), len(all))
	}

	for key, expectedValue := range testData {
		if all[key] != expectedValue {
			t.Errorf("GetAll() returned wrong value for key %s. Expected %s, got %s", key, expectedValue, all[key])
		}
	}
}

func TestKVStore_PutFromDb(t *testing.T) {
	store := NewKVStore()

	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	for key, value := range testData {
		store.PutFromDb(key, value)
	}

	// Verifica se foi salvo apenas na memória
	for key, expectedValue := range testData {
		if store.store[key] != expectedValue {
			t.Errorf("PutFromDb() failed. Expected %s, got %s", expectedValue, store.store[key])
		}
	}
}

func TestKVStore_Watch(t *testing.T) {
	store := NewKVStore()

	// Testa criação de watcher
	watcher := store.Watch("test_key")
	if watcher == nil {
		t.Fatal("Watch() returned nil")
	}

	if watcher.Key != "test_key" {
		t.Errorf("Watch() set wrong key. Expected test_key, got %s", watcher.Key)
	}

	if watcher.Events == nil {
		t.Fatal("Watch() created watcher with nil Events channel")
	}

	// Verifica se o watcher foi adicionado à lista
	if len(store.watchers["test_key"]) != 1 {
		t.Errorf("Watch() failed to add watcher to list. Expected 1, got %d", len(store.watchers["test_key"]))
	}

	// Testa múltiplos watchers para a mesma chave
	store.Watch("test_key")
	if len(store.watchers["test_key"]) != 2 {
		t.Errorf("Watch() failed to add second watcher. Expected 2, got %d", len(store.watchers["test_key"]))
	}

	// Testa watchers para chaves diferentes
	store.Watch("other_key")
	if len(store.watchers["other_key"]) != 1 {
		t.Errorf("Watch() failed to add watcher for different key. Expected 1, got %d", len(store.watchers["other_key"]))
	}
}

func TestKVStore_Unwatch(t *testing.T) {
	store := NewKVStore()

	// Cria watchers
	watcher1 := store.Watch("test_key")
	store.Watch("test_key")
	store.Watch("other_key")

	// Verifica estado inicial
	if len(store.watchers["test_key"]) != 2 {
		t.Errorf("Expected 2 watchers for test_key, got %d", len(store.watchers["test_key"]))
	}

	// Remove primeiro watcher
	store.Unwatch(watcher1)

	// Verifica se foi removido
	if len(store.watchers["test_key"]) != 1 {
		t.Errorf("Unwatch() failed to remove watcher. Expected 1, got %d", len(store.watchers["test_key"]))
	}

	// Verifica se o canal foi fechado
	select {
	case <-watcher1.Events:
		// Canal foi fechado, isso é esperado
	default:
		t.Error("Unwatch() should close the Events channel")
	}

	// Verifica se outros watchers não foram afetados
	if len(store.watchers["other_key"]) != 1 {
		t.Error("Unwatch() affected wrong watchers")
	}

	// Remove watcher inexistente (não deve causar erro)
	store.Unwatch(&KVWatcher{Key: "nonexistent", Events: make(chan string)})
}

func TestKVStore_WatchNotifications(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	Init(db)
	store := NewKVStore()

	// Cria watcher
	watcher := store.Watch("test_key")

	// Canal para receber notificações
	notifications := make([]string, 0)
	done := make(chan bool)

	go func() {
		for event := range watcher.Events {
			notifications = append(notifications, event)
		}
		done <- true
	}()

	// Faz algumas operações PUT
	store.Put("test_key", "value1")
	store.Put("test_key", "value2")
	store.Put("other_key", "value3") // Não deve gerar notificação

	// Aguarda um pouco para as notificações chegarem
	time.Sleep(100 * time.Millisecond)

	// Remove o watcher
	store.Unwatch(watcher)

	// Aguarda o canal ser fechado
	<-done

	// Verifica se recebeu as notificações corretas
	expectedNotifications := 2
	if len(notifications) != expectedNotifications {
		t.Errorf("Expected %d notifications, got %d", expectedNotifications, len(notifications))
	}

	// Verifica conteúdo das notificações
	for i, notification := range notifications {
		expectedValue := "value" + string(rune('1'+i))
		expectedMessage := "Key test_key updated to " + expectedValue
		if notification != expectedMessage {
			t.Errorf("Notification %d: expected %s, got %s", i, expectedMessage, notification)
		}
	}
}

func TestKVStore_Concurrency(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	Init(db)
	store := NewKVStore()

	// Testa concorrência com múltiplas goroutines
	numGoroutines := 10
	numOperations := 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)

				store.Put(key, value)
				retrieved := store.Get(key)
				if retrieved != value {
					t.Errorf("Concurrency test failed: expected %s, got %s", value, retrieved)
				}
			}
			done <- true
		}(i)
	}

	// Aguarda todas as goroutines terminarem
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verifica se todos os dados foram salvos corretamente
	all := store.GetAll()
	expectedCount := numGoroutines * numOperations
	if len(all) != expectedCount {
		t.Errorf("Concurrency test: expected %d items, got %d", expectedCount, len(all))
	}
}
