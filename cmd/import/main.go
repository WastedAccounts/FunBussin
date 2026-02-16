// Command to test importing contacts from testdata/import_test.json into Mailchimp.
//
// Usage (from project root):
//
//	go run ./cmd/import
//	go run ./cmd/import -file testdata/import_test.json
//
// Requires: MAILCHIMP_API_KEY, MAILCHIMP_SERVER, MAILCHIMP_LIST_ID in .env
package main

import (
	"flag"
	"os"

	"funbussin/backend/appinit"
	"funbussin/logging"
	"funbussin/mailchimp"
)

func main() {
	file := flag.String("file", "testdata/import_test.json", "path to JSON file with contacts")
	flag.Parse()

	appinit.Init()

	apiKey := os.Getenv("MAILCHIMP_API_KEY")
	server := os.Getenv("MAILCHIMP_SERVER")
	listID := os.Getenv("MAILCHIMP_LIST_ID")
	if apiKey == "" || listID == "" {
		logging.GetLogger().Fatal("cmd.import", "MAILCHIMP_API_KEY and MAILCHIMP_LIST_ID are required (MAILCHIMP_SERVER optional if in API key)")
	}

	client, err := mailchimp.NewClient(mailchimp.Config{APIKey: apiKey, Server: server})
	if err != nil {
		logging.GetLogger().Fatalf("cmd.import", "%v", err)
	}

	if err := appinit.InitMailchimp(client); err != nil {
		logging.GetLogger().Fatalf("cmd.import", "%v", err)
	}

	if err := client.ImportFromFile(listID, *file, "subscribed"); err != nil {
		logging.GetLogger().Fatalf("cmd.import", "%v", err)
	}
}
