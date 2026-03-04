package config

import (
	"os"
	"strings"
)

type Config struct {
	ServerPort      string
	MongoURI        string
	MongoDatabase   string
	MongoCollection string
	LogLevel        string
	LogFormat       string
}

func Load() Config {
	return Config{
		ServerPort:      getEnv("APP_PORT", "8080"),
		MongoURI:        getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:   getEnv("MONGO_DB", "appdb"),
		MongoCollection: getEnv("MONGO_COLLECTION", "tasks"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		LogFormat:       getEnv("LOG_FORMAT", "text"),
	}
}

func (c Config) HTTPAddress() string {
	if strings.HasPrefix(c.ServerPort, ":") {
		return c.ServerPort
	}

	return ":" + c.ServerPort
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
