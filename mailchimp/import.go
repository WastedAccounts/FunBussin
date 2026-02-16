package mailchimp

import (
	"encoding/json"
	"fmt"
	"os"

	"funbussin/logging"
)

// ImportFromFile reads contacts from a JSON file and adds/updates them in the given list.
// The JSON file must be an array of Contact objects.
func (c *Client) ImportFromFile(listID string, path string, status string) error {
	if path == "" {
		return fmt.Errorf("import path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	var contacts []Contact
	if err := json.Unmarshal(data, &contacts); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}
	logging.GetLogger().ActivityLogging("mailchimp.ImportFromFile", fmt.Sprintf("Importing %d contacts from %s to list %s", len(contacts), path, listID))
	logging.GetLogger().Info("Importing %d contacts from %s to list %s", len(contacts), path, listID)
	return c.ImportContacts(listID, contacts, status)
}

// ImportContacts adds/updates the given contacts in the Mailchimp list.
func (c *Client) ImportContacts(listID string, contacts []Contact, status string) error {
	if status == "" {
		status = "subscribed"
	}
	for i, ct := range contacts {
		if ct.EmailAddress == "" {
			logging.GetLogger().ActivityLogging("mailchimp.ImportContacts", fmt.Sprintf("[%d] skip: missing email", i+1))
			logging.GetLogger().Info("  [%d] skip: missing email", i+1)
			continue
		}
		if err := c.AddOrUpdateMember(listID, ct, status); err != nil {
			logging.GetLogger().ErrorLogging(5, "mailchimp.ImportContacts", fmt.Sprintf("[%d] %s: %v", i+1, ct.EmailAddress, err))
			logging.GetLogger().Info("  [%d] %s: %v", i+1, ct.EmailAddress, err)
			continue
		}
		logging.GetLogger().ActivityLogging("mailchimp.ImportContacts", fmt.Sprintf("[%d] %s: ok", i+1, ct.EmailAddress))
		logging.GetLogger().Info("  [%d] %s: ok", i+1, ct.EmailAddress)
	}
	return nil
}
