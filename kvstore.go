package main

import "fmt"

type KVStore struct {
	store map[string]string
}

func (kv *KVStore) Put(key, value string) {
	if kv.store == nil {
		kv.store = make(map[string]string)
	}

	kv.store[key] = value

}

func (kv *KVStore) Get(key string) string {
	if kv.store == nil {
		return ""
	}
	return kv.store[key]
}

func main() {

	kv := &KVStore{}

	kv.Put("winner", "daniel")
	kv.Put("runnerup", "jhenifer")

	fmt.Println("winner:", kv.Get("winner"))
	fmt.Println("runnerup:", kv.Get("runnerup"))

}
