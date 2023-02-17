package utils

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"
)

func TestGetPort(t *testing.T) {
	// Test the case where the PORT environment variable is set
	os.Setenv("PORT", "1234")
	port := GetPort()
	if port != "1234" {
		t.Errorf("Expected port to be %q, but got %q", "1234", port)
	}

	// Test the case where the PORT environment variable is not set
	os.Unsetenv("PORT")
	port = GetPort()
	if port != "8080" {
		t.Errorf("Expected port to be %q, but got %q", "8080", port)
	}
}

func TestWriteLog(t *testing.T) {
	var buf bytes.Buffer

	log.SetOutput(&buf)

	tests := []struct {
		level    string
		message  string
		expected string
	}{
		{
			level:    "INFO",
			message:  "Test log message",
			expected: "Test log message",
		},
		{
			level:    "WARNING",
			message:  "Another test log message",
			expected: "Another test log message",
		},
	}

	for _, tt := range tests {
		WriteLog(tt.level, tt.message)

		var logEntry Log
		err := json.NewDecoder(&buf).Decode(&logEntry)
		if err != nil {
			t.Errorf("Failed to decode log entry: %v", err)
		}

		if logEntry.Level != tt.level {
			t.Errorf("Expected log level %s, but got %s", tt.level, logEntry.Level)
		}

		if !strings.Contains(logEntry.Message, tt.expected) {
			t.Errorf("Expected log message to contain %q, but got %q", tt.expected, logEntry.Message)
		}

		buf.Reset() // clear the buffer before the next test
	}
}
