package store

import (
	"fmt"
	"sync"
)

type KVStore struct {
	mu    sync.Mutex
	store map[string]string
}

func (kv *KVStore) Put(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.store == nil {
		kv.store = make(map[string]string)
	}

	kv.store[key] = value
	fmt.Printf("[PUT] key=%s, value=%s\n", key, value)
	// return key, value

}

func (kv *KVStore) Get(key string) string {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.store == nil {
		return ""
	}
	return kv.store[key]
}
