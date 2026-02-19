package appinit

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"funbussin/logging"
	"funbussin/mailchimp"
	"funbussin/smartwaiver"
)

// Init loads environment variables from .env and performs app initialization.
func Init() {
	logging.GetLogger().SetLogLevel(5)
	logging.GetLogger().SetLogFileNames("./logs/activity.log", "./logs/error.log", "./logs/service.log")
	logging.GetLogger().ServiceLog("appinit.Init", "Logging system initialized")

	if err := godotenv.Load(); err != nil {
		logging.GetLogger().ServiceLog("appinit.Init.godotenv", "No .env file found. Will use system environment variables")
	} else {
		logging.GetLogger().ServiceLog("appinit.Init.godotenv", ".env file found. Using variables stored here")
	}

	if os.Getenv("TZ") == "" {
		os.Setenv("TZ", "America/New_York")
	}
}

// InitSmartwaiver verifies Smartwaiver API connectivity (ping and version).
func InitSmartwaiver(client *smartwaiver.Client) error {
	pong, err := client.Ping()
	if err != nil {
		return fmt.Errorf("smartwaiver ping failed: %w", err)
	}
	logging.GetLogger().ServiceLog("appinit.InitSmartwaiver", "Ping: %s", pong)

	version, err := client.Version()
	if err != nil {
		return fmt.Errorf("smartwaiver version failed: %w", err)
	}
	logging.GetLogger().ServiceLog("appinit.InitSmartwaiver", "API version: %s", version)
	return nil
}

// InitMailchimp verifies Mailchimp API connectivity (ping).
func InitMailchimp(client *mailchimp.Client) error {
	health, err := client.Ping()
	if err != nil {
		return fmt.Errorf("mailchimp ping failed: %w", err)
	}
	logging.GetLogger().ServiceLog("appinit.InitMailchimp", "Ping: %s", health)
	return nil
}
