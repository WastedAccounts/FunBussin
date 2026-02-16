package main

import (
	"encoding/json"
	"flag"
	"os"
	"time"

	"funbussin/backend/appinit"
	"funbussin/logging"
	"funbussin/mailchimp"
	"funbussin/smartwaiver"
)

func main() {
	doImport := flag.Bool("import", false, "import contacts from testdata/import_test.json to Mailchimp")
	importFile := flag.String("file", "testdata/import_test.json", "path to JSON file (used with -import)")
	doSync := flag.Bool("sync", false, "fetch waivers from Smartwaiver and import to Mailchimp")
	syncLimit := flag.Int("limit", 100, "max waivers to fetch (used with -sync)")
	syncAll := flag.Bool("all", false, "ignore 24h filter, fetch all waivers (for debugging)")
	syncDryRun := flag.Bool("dry-run", false, "with -sync: fetch only, do not import to Mailchimp")
	flag.Parse()

	appinit.Init()

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

	if *doImport {
		runImport(*importFile)
		return
	}

	if *doSync {
		runSync(swClient, *syncLimit, *syncAll, *syncDryRun)
		return
	}
}

func runSync(swClient *smartwaiver.Client, limit int, all, dryRun bool) {
	if swClient == nil {
		logging.GetLogger().Fatal("main.runSync", "SMARTWAIVER_API_KEY is required")
	}
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
	contacts, err := swClient.FetchContacts(opts)
	if err != nil {
		logging.GetLogger().Fatalf("main.runSync", "%v", err)
	}
	logging.GetLogger().Info("Converted %d contacts", len(contacts))
	if dryRun {
		logging.GetLogger().Info("Dry run: skipping Mailchimp import")
		pretty, err := json.MarshalIndent(contacts, "", "  ")
		if err != nil {
			logging.GetLogger().Fatalf("main.runSync", "marshal: %v", err)
		}
		os.Stdout.Write(pretty)
		return
	}
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

func runImport(file string) {
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
	logging.GetLogger().Info("Using MAILCHIMP_LIST_ID=%s", mcListID)
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
