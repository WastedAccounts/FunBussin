// Package main runs FunBussin: syncs waiver data from Smartwaiver to Mailchimp.
// Supports -import (file), -sync (Smartwaiver fetch + Mailchimp import), and -sync -interval (repeated runs).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"funbussin/backend/appinit"
	"funbussin/logging"
	"funbussin/mailchimp"
	"funbussin/smartwaiver"
)

func main() {
	// Import: load contacts from JSON file and send to Mailchimp
	doImport := flag.Bool("import", false, "import contacts from testdata/import_test.json to Mailchimp")
	importFile := flag.String("file", "testdata/import_test.json", "path to JSON file (used with -import)")

	// Sync: fetch waivers from Smartwaiver, convert to contacts, import to Mailchimp
	doSync := flag.Bool("sync", false, "fetch waivers from Smartwaiver and import to Mailchimp")
	syncInterval := flag.String("interval", "", "with -sync: run every N (e.g. 15m); run until stopped")
	syncLimit := flag.Int("limit", 100, "max waivers to fetch (used with -sync)")
	syncAll := flag.Bool("all", false, "ignore 24h filter, fetch all waivers (for debugging)")
	syncDryRun := flag.Bool("dry-run", false, "with -sync: fetch only, do not import to Mailchimp")

	flag.Parse()

	// Load .env and initialize logging
	appinit.Init()

	// Smartwaiver client (required for sync, optional for import-only)
	apiKey := os.Getenv("SMARTWAIVER_API_KEY")
	if apiKey == "" && !*doImport {
		logging.GetLogger().Fatal("main", "SMARTWAIVER_API_KEY environment variable is required")
	}

	var swClient *smartwaiver.Client
	if apiKey != "" {
		var err error
		swClient, err = smartwaiver.NewClient(smartwaiver.Config{APIKey: apiKey})
		if err != nil {
			logging.GetLogger().Fatalf("main", "%v", err)
		}
		if err := appinit.InitSmartwaiver(swClient); err != nil {
			logging.GetLogger().Fatalf("main", "%v", err)
		}
	}

	// Mailchimp client (optional for sync-only dry run; required for import)
	mcKey := os.Getenv("MAILCHIMP_API_KEY")
	mcServer := os.Getenv("MAILCHIMP_SERVER")
	if mcKey != "" {
		mcClient, err := mailchimp.NewClient(mailchimp.Config{APIKey: mcKey, Server: mcServer})
		if err != nil {
			logging.GetLogger().Fatalf("main", "%v", err)
		}
		if err := appinit.InitMailchimp(mcClient); err != nil {
			logging.GetLogger().Fatalf("main", "%v", err)
		}
	}

	// Dispatch to import or sync
	if *doImport {
		runImport(*importFile)
		return
	}

	if *doSync {
		// With -interval, run in a loop until SIGINT/SIGTERM
		interval, _ := time.ParseDuration(*syncInterval)
		if interval > 0 {
			runSyncLoop(swClient, *syncLimit, *syncAll, *syncDryRun, interval)
		} else {
			runSync(swClient, *syncLimit, *syncAll, *syncDryRun)
		}
		return
	}
}

// runSyncLoop runs sync repeatedly at the given interval until SIGINT or SIGTERM.
func runSyncLoop(swClient *smartwaiver.Client, limit int, all, dryRun bool, interval time.Duration) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logging.GetLogger().ServiceLog("main.runSyncLoop", "Running sync every %s (Ctrl+C to stop)", interval)
	for {
		runSync(swClient, limit, all, dryRun)
		select {
		case <-ctx.Done():
			logging.GetLogger().ServiceLog("main.runSyncLoop", "Stopping (received signal)")
			return
		case <-time.After(interval):
			// next run
		}
	}
}

// runSync fetches waivers from Smartwaiver, converts to contacts, and imports to Mailchimp.
// Skips Mailchimp import when dryRun is true.
func runSync(swClient *smartwaiver.Client, limit int, all, dryRun bool) {
	if swClient == nil {
		logging.GetLogger().Fatal("main.runSync", "SMARTWAIVER_API_KEY is required")
	}
	// Build list options: filter by last 24h unless -all
	opts := smartwaiver.ListWaiversOptions{Limit: limit}
	if !all {
		cutoff := time.Now().Add(-24 * time.Hour)
		opts.FilterSince = &cutoff
		if limit < 300 {
			opts.Limit = 300 // fetch more when filtering client-side (API max)
		}
		logging.GetLogger().Info("Fetching up to %d waivers from Smartwaiver (last 24h, client-side filter)...", opts.Limit)
		logging.GetLogger().Info("  cutoff=%s (local)", cutoff.Format(time.RFC3339))
	} else {
		logging.GetLogger().Info("Fetching up to %d waivers from Smartwaiver (no date filter, newest first)...", limit)
	}
	// Fetch and convert waivers to Mailchimp contacts
	contacts, err := swClient.FetchContacts(opts)
	if err != nil {
		logging.GetLogger().Fatalf("main.runSync", "%v", err)
	}
	logging.GetLogger().Info("Converted %d contacts", len(contacts))
	// Dry run: output JSON and exit without importing
	if dryRun {
		logging.GetLogger().Info("Dry run: skipping Mailchimp import")
		pretty, err := json.MarshalIndent(contacts, "", "  ")
		if err != nil {
			logging.GetLogger().Fatalf("main.runSync", "marshal: %v", err)
		}
		os.Stdout.Write(pretty)
		return
	}
	// Validate Mailchimp env vars and create client
	mcKey := os.Getenv("MAILCHIMP_API_KEY")
	mcServer := os.Getenv("MAILCHIMP_SERVER")
	mcListID := os.Getenv("MAILCHIMP_LIST_ID")
	var missing []string
	if mcKey == "" {
		missing = append(missing, "MAILCHIMP_API_KEY")
	}
	if mcListID == "" {
		missing = append(missing, "MAILCHIMP_LIST_ID")
	}
	if len(missing) > 0 {
		logging.GetLogger().Fatalf("main.runSync", "import requires %v (set in .env)", missing)
	}
	mcClient, err := mailchimp.NewClient(mailchimp.Config{APIKey: mcKey, Server: mcServer})
	if err != nil {
		logging.GetLogger().Fatalf("main.runSync", "%v", err)
	}
	if err := appinit.InitMailchimp(mcClient); err != nil {
		logging.GetLogger().Fatalf("main.runSync", "%v", err)
	}
	logging.GetLogger().Info("Importing %d contacts to Mailchimp list %s", len(contacts), mcListID)
	if err := mcClient.ImportContacts(mcListID, contacts, "subscribed"); err != nil {
		logging.GetLogger().Fatalf("main.runSync", "%v", err)
	}
}

// runImport reads contacts from a JSON file and imports them to Mailchimp.
func runImport(file string) {
	// Validate required Mailchimp env vars
	mcKey := os.Getenv("MAILCHIMP_API_KEY")
	mcServer := os.Getenv("MAILCHIMP_SERVER")
	mcListID := os.Getenv("MAILCHIMP_LIST_ID")
	var missing []string
	if mcKey == "" {
		missing = append(missing, "MAILCHIMP_API_KEY")
	}
	if mcListID == "" {
		missing = append(missing, "MAILCHIMP_LIST_ID")
	}
	if len(missing) > 0 {
		logging.GetLogger().Fatalf("main.runImport", "import requires %v (set in .env or environment; run from project root so .env loads)", missing)
	}
	logging.GetLogger().ServiceLog("main.runImport", "Using MAILCHIMP_LIST_ID=%s", mcListID)
	// Create client and import contacts from file
	client, err := mailchimp.NewClient(mailchimp.Config{APIKey: mcKey, Server: mcServer})
	if err != nil {
		logging.GetLogger().Fatalf("main.runImport", "%v", err)
	}
	if err := appinit.InitMailchimp(client); err != nil {
		logging.GetLogger().Fatalf("main.runImport", "%v", err)
	}
	if err := client.ImportFromFile(mcListID, file, "subscribed"); err != nil {
		logging.GetLogger().Fatalf("main.runImport", "%v", err)
	}
}
