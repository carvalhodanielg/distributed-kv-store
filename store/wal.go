package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type Operation uint8

const (
	Write  Operation = iota
	Delete Operation = iota
)

type WalLog struct {
	Operation Operation
	Key       string
	Value     string
	Timestamp int64 //Unix timestamp
}

// Função deve ser privada
func appendLogToFile(wallog WalLog) {
	data, err := json.Marshal(wallog)
	fmt.Println(string(data))

	if err != nil {
		log.Fatalf("Erro ao converter para json %v", err)
	}

	file, error := os.OpenFile("walog.ndjson", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if error != nil {
		panic(error)
	}

	defer file.Close()

	if _, err := file.Write(append(data, '\n')); err != nil {
		panic(err)
	}

}

func LogWrite(key, value string) {
	appendLogToFile(WalLog{Operation: Write, Key: key, Value: value, Timestamp: time.Now().Unix()})
}

func LogDelete(key string) {
	appendLogToFile(WalLog{Operation: Delete, Key: key, Value: "", Timestamp: time.Now().Unix()})
}
