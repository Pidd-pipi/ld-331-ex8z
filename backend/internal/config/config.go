package config

import (
	"fmt"
	"os"
)

type Config struct{ Port, DBDriver, DBHost, DBPort, DBName, DBUser, DBPassword, JWTSecret string }

func Load() Config {
	return Config{Port: getenv("PORT", "19931"), DBDriver: getenv("DB_DRIVER", "sqlite"), DBHost: getenv("DB_HOST", "127.0.0.1"), DBPort: getenv("DB_PORT", "3306"), DBName: getenv("DB_NAME", "scheduler"), DBUser: getenv("DB_USER", "scheduler"), DBPassword: getenv("DB_PASSWORD", "scheduler_pass"), JWTSecret: getenv("JWT_SECRET", "development-secret-change-me")}
}
func (c Config) DSN() string {
	if c.DBDriver == "mysql" {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
	}
	return "scheduler.db"
}
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
