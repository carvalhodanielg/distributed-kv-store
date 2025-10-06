package store

import (
	"fmt"
	"sync"

	"github.com/carvalhodanielg/kvstore/internal/constants"
	bolt "go.etcd.io/bbolt"
)

type KVWatcher struct {
	Key    string
	Events chan string
}

type KVStore struct {
	mu       sync.RWMutex
	store    map[string]string
	watchers map[string][]*KVWatcher
	// db       *bolt.DB
}

var db *bolt.DB

func Init(d *bolt.DB) {
	db = d
}

func NewKVStore() *KVStore {
	return &KVStore{
		store:    make(map[string]string),
		watchers: make(map[string][]*KVWatcher),
	}
}

func (kv *KVStore) GetAll() map[string]string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	return kv.store

}

func (kv *KVStore) Delete(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	//log -> memoria -> db
	LogDelete(key)
	delete(kv.store, key)
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(constants.BucketStore))
		err := b.Delete([]byte(key))
		return err
	})
}

// Function that put data in memory after restart. It does not write to log or db
func (kv *KVStore) PutFromDb(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.store == nil {
		kv.store = make(map[string]string)
	}

	//escreve apenas em memória
	kv.store[key] = value

}

func (kv *KVStore) Put(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.store == nil {
		kv.store = make(map[string]string)
	}

	//escreve no log -> memória -> banco
	LogWrite(key, value)
	kv.store[key] = value

	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(constants.BucketStore))
		err := b.Put([]byte(key), []byte(value))
		return err
	})

	if wlist, ok := kv.watchers[key]; ok {

		for _, w := range wlist {
			select {
			case w.Events <- fmt.Sprintf("Key %s updated to %s", key, value):
			default:
				fmt.Printf("Envio não foi feito pro canal")
			}
		}
	}

	fmt.Printf("[PUT] key=%s, value=%s\n", key, value)
}

func (kv *KVStore) Get(key string) string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	if kv.store == nil {
		return ""
	}

	//tratar isso aqui caso nao exista em memoria
	//e exista suspeita de desatualização em relação ao db
	return kv.store[key]
}

// Esse Watch vai receber uma key, criar um watcher pra quem chamou
// e fará o append do watcher na slice de watchers da store
// logo depois retorna o watcher específico para a key fornecida
// assim, quem chamou o watch pode acompanhar as atualizações daquela key.
func (kv *KVStore) Watch(key string) *KVWatcher {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	w := &KVWatcher{
		Key:    key,
		Events: make(chan string, 10),
	}

	kv.watchers[key] = append(kv.watchers[key], w)

	return w
}

func (kv *KVStore) Unwatch(watcherToUnwatch *KVWatcher) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	watchersList := kv.watchers[watcherToUnwatch.Key]

	for i, watcher := range watchersList {
		if watcher == watcherToUnwatch {
			kv.watchers[watcherToUnwatch.Key] = append(watchersList[:i], watchersList[i+1:]...)
			close(watcherToUnwatch.Events)
			break
		}
	}
}
