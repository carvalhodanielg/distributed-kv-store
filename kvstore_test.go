package store_test

import (
	"testing"

	"github.com/carvalhodanielg/kvstore/store"
)

func TestKVStore(t *testing.T) {

	kv := &store.KVStore{}

	kv.Put("niceMan", "daniel")

	kv.Put("handsomeMan", "daniel too")

	niceMan := kv.Get("niceMan")
	handsomeMan := kv.Get("handsomeMan")

	if niceMan != "daniel" {
		t.Errorf("Should be daniel, got %v", niceMan)
	}

	if handsomeMan != "daniel too" {
		t.Errorf("Should be daniel too, got %v", handsomeMan)
	}

}
