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

}

func (kv *KVStore) Get(key string) string {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.store == nil {
		return ""
	}
	return kv.store[key]
}

func main() {

	kv := &KVStore{}

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)

			kv.Put(key, value)
		}(i)

	}

	wg.Wait()

	fmt.Println("FIM da execução")

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		fmt.Printf("%s => %s\n", key, kv.Get(key))
	}

}
