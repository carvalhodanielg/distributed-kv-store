package store

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// setupTestWAL cria um arquivo de log temporário para testes
func setupTestWAL(t *testing.T) string {
	logFile := "test_walog.ndjson"
	os.Remove(logFile) // Remove se existir
	return logFile
}

// cleanupTestWAL remove o arquivo de log de teste
func cleanupTestWAL(t *testing.T, logFile string) {
	os.Remove(logFile)
}

// readLastLogEntry lê a última entrada do arquivo de log
func readLastLogEntry(t *testing.T, logFile string) *WalLog {
	file, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	var lastEntry WalLog
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry WalLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}
		lastEntry = entry
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	return &lastEntry
}

// readAllLogEntries lê todas as entradas do arquivo de log
func readAllLogEntries(t *testing.T, logFile string) []WalLog {
	file, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	var entries []WalLog
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry WalLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	return entries
}

func TestOperation_String(t *testing.T) {
	tests := []struct {
		operation Operation
		expected  string
	}{
		{Write, "Write"},
		{Delete, "Delete"},
		{Operation(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.operation.String()
			if result != tt.expected {
				t.Errorf("Operation.String() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestOperation_MarshalJSON(t *testing.T) {
	tests := []struct {
		operation Operation
		expected  string
	}{
		{Write, `"Write"`},
		{Delete, `"Delete"`},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			data, err := tt.operation.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() failed: %v", err)
			}

			result := string(data)
			if result != tt.expected {
				t.Errorf("Operation.MarshalJSON() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestWalLog_Structure(t *testing.T) {
	log := WalLog{
		Operation: Write,
		Key:       "test_key",
		Value:     "test_value",
		Timestamp: time.Now().Unix(),
	}

	if log.Operation != Write {
		t.Error("WalLog.Operation not set correctly")
	}

	if log.Key != "test_key" {
		t.Error("WalLog.Key not set correctly")
	}

	if log.Value != "test_value" {
		t.Error("WalLog.Value not set correctly")
	}

	if log.Timestamp <= 0 {
		t.Error("WalLog.Timestamp not set correctly")
	}
}

func TestLogWrite(t *testing.T) {
	logFile := setupTestWAL(t)
	defer cleanupTestWAL(t, logFile)

	// Temporariamente substitui o nome do arquivo de log
	originalLogFile := "walog.ndjson"

	// Testa LogWrite
	testKey := "test_key"
	testValue := "test_value"

	LogWrite(testKey, testValue)

	// Verifica se o arquivo foi criado
	if _, err := os.Stat(originalLogFile); os.IsNotExist(err) {
		t.Fatal("Log file was not created")
	}

	// Lê a entrada do log
	entries := readAllLogEntries(t, originalLogFile)
	if len(entries) == 0 {
		t.Fatal("No log entries found")
	}

	lastEntry := entries[len(entries)-1]

	// Verifica os campos da entrada
	if lastEntry.Operation != Write {
		t.Errorf("Expected operation Write, got %v", lastEntry.Operation)
	}

	if lastEntry.Key != testKey {
		t.Errorf("Expected key %s, got %s", testKey, lastEntry.Key)
	}

	if lastEntry.Value != testValue {
		t.Errorf("Expected value %s, got %s", testValue, lastEntry.Value)
	}

	if lastEntry.Timestamp <= 0 {
		t.Error("Timestamp should be positive")
	}

	// Verifica se o timestamp é recente (dentro dos últimos 5 segundos)
	now := time.Now().Unix()
	if now-lastEntry.Timestamp > 5 {
		t.Error("Timestamp is too old")
	}

	// Limpa o arquivo de log original
	os.Remove(originalLogFile)
}

func TestLogDelete(t *testing.T) {
	originalLogFile := "walog.ndjson"

	// Testa LogDelete
	testKey := "test_key_to_delete"

	LogDelete(testKey)

	// Verifica se o arquivo foi criado
	if _, err := os.Stat(originalLogFile); os.IsNotExist(err) {
		t.Fatal("Log file was not created")
	}

	// Lê a entrada do log
	entries := readAllLogEntries(t, originalLogFile)
	if len(entries) == 0 {
		t.Fatal("No log entries found")
	}

	lastEntry := entries[len(entries)-1]

	// Verifica os campos da entrada
	if lastEntry.Operation != Delete {
		t.Errorf("Expected operation Delete, got %v", lastEntry.Operation)
	}

	if lastEntry.Key != testKey {
		t.Errorf("Expected key %s, got %s", testKey, lastEntry.Key)
	}

	if lastEntry.Value != "" {
		t.Errorf("Expected empty value, got %s", lastEntry.Value)
	}

	if lastEntry.Timestamp <= 0 {
		t.Error("Timestamp should be positive")
	}

	// Limpa o arquivo de log
	os.Remove(originalLogFile)
}

func TestLogWrite_MultipleEntries(t *testing.T) {
	originalLogFile := "walog.ndjson"

	// Faz múltiplas operações de log
	testData := []struct {
		key   string
		value string
	}{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	for _, data := range testData {
		LogWrite(data.key, data.value)
	}

	// Lê todas as entradas
	entries := readAllLogEntries(t, originalLogFile)

	if len(entries) != len(testData) {
		t.Errorf("Expected %d entries, got %d", len(testData), len(entries))
	}

	// Verifica cada entrada
	for i, entry := range entries {
		expected := testData[i]

		if entry.Operation != Write {
			t.Errorf("Entry %d: expected operation Write, got %v", i, entry.Operation)
		}

		if entry.Key != expected.key {
			t.Errorf("Entry %d: expected key %s, got %s", i, expected.key, entry.Key)
		}

		if entry.Value != expected.value {
			t.Errorf("Entry %d: expected value %s, got %s", i, expected.value, entry.Value)
		}
	}

	// Limpa o arquivo de log
	os.Remove(originalLogFile)
}

func TestLogWrite_AppendMode(t *testing.T) {
	originalLogFile := "walog.ndjson"

	// Primeira operação
	LogWrite("key1", "value1")

	// Segunda operação (deve ser appendada)
	LogWrite("key2", "value2")

	// Lê todas as entradas
	entries := readAllLogEntries(t, originalLogFile)

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Verifica se ambas as entradas estão presentes
	foundKey1 := false
	foundKey2 := false

	for _, entry := range entries {
		if entry.Key == "key1" && entry.Value == "value1" {
			foundKey1 = true
		}
		if entry.Key == "key2" && entry.Value == "value2" {
			foundKey2 = true
		}
	}

	if !foundKey1 {
		t.Error("First log entry not found")
	}

	if !foundKey2 {
		t.Error("Second log entry not found")
	}

	// Limpa o arquivo de log
	os.Remove(originalLogFile)
}

func TestLogWrite_SpecialCharacters(t *testing.T) {
	originalLogFile := "walog.ndjson"

	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{"empty_key", "", "value"},
		{"empty_value", "key", ""},
		{"special_chars", "key!@#$%", "value!@#$%"},
		{"unicode", "key_中文", "value_中文"},
		{"newlines", "key\nwith\nnewlines", "value\nwith\nnewlines"},
		{"quotes", "key\"with\"quotes", "value\"with\"quotes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			LogWrite(tc.key, tc.value)

			entries := readAllLogEntries(t, originalLogFile)
			if len(entries) == 0 {
				t.Fatal("No log entries found")
			}

			lastEntry := entries[len(entries)-1]

			if lastEntry.Key != tc.key {
				t.Errorf("Key mismatch: expected %q, got %q", tc.key, lastEntry.Key)
			}

			if lastEntry.Value != tc.value {
				t.Errorf("Value mismatch: expected %q, got %q", tc.value, lastEntry.Value)
			}
		})
	}

	// Limpa o arquivo de log
	os.Remove(originalLogFile)
}

func TestLogWrite_JSONFormat(t *testing.T) {
	originalLogFile := "walog.ndjson"

	LogWrite("test_key", "test_value")

	// Lê o arquivo como texto
	file, err := os.Open(originalLogFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("No line found in log file")
	}

	line := scanner.Text()

	// Verifica se é um JSON válido
	var entry WalLog
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("Log entry is not valid JSON: %v", err)
	}

	// Verifica se contém os campos esperados
	if !strings.Contains(line, `"Operation":"Write"`) {
		t.Error("Log entry does not contain Operation field")
	}

	if !strings.Contains(line, `"Key":"test_key"`) {
		t.Error("Log entry does not contain Key field")
	}

	if !strings.Contains(line, `"Value":"test_value"`) {
		t.Error("Log entry does not contain Value field")
	}

	if !strings.Contains(line, `"Timestamp"`) {
		t.Error("Log entry does not contain Timestamp field")
	}

	// Limpa o arquivo de log
	os.Remove(originalLogFile)
}
