package utils

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type Log struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

func GetPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // set a default port if PORT is not set
	}
	return port
}

func WriteLog(level string, message string) {
	logData := Log{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
	}

	logBytes, err := json.Marshal(logData)
	if err != nil {
		log.Fatal(err)
	}
	// remove timestamp prefix
	log.SetFlags(0)
	log.Println(string(logBytes))

}
