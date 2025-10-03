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

func (o Operation) String() string {
	switch o {
	case Write:
		return "Write"
	case Delete:
		return "Delete"
	default:
		return "Unknown"
	}
}

func (o Operation) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.String())
}

type WalLog struct {
	Operation Operation `json:"Operation"`
	Key       string    `json:"Key"`
	Value     string    `json:"Value"`
	Timestamp int64     `json:"Timestamp"` //Unix timestamp
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
