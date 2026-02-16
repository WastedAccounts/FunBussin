package smartwaiver

import (
	"encoding/json"
	"sort"
	"time"

	"funbussin/logging"
	"funbussin/mailchimp"
)

// parseCreatedOn parses Smartwaiver's createdOn string. Tries common formats.
func parseCreatedOn(s string) (time.Time, bool) {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// FetchContacts fetches waivers from Smartwaiver and converts them to Mailchimp contacts.
// Each waiver becomes one contact (parent/guardian + top 3 children).
// Waivers are processed newest-first (by CreatedOn descending).
// When opts.FilterSince is set, filter client-side to waivers created on/after that time (avoids API fromDts issues).
func (c *Client) FetchContacts(opts ListWaiversOptions) ([]mailchimp.Contact, error) {
	// Don't use API fromDts when we're filtering client-side
	apiOpts := opts
	if opts.FilterSince != nil {
		apiOpts.FromDts = ""
		apiOpts.ToDts = ""
	}
	list, err := c.ListWaivers(apiOpts)
	if err != nil {
		return nil, err
	}
	// Sort newest first (CreatedOn descending; ISO 8601 strings sort correctly)
	sort.Slice(list.Waivers, func(i, j int) bool {
		return list.Waivers[i].CreatedOn > list.Waivers[j].CreatedOn
	})
	// Client-side filter: only waivers created on/after FilterSince
	if opts.FilterSince != nil {
		cutoff := *opts.FilterSince
		before := len(list.Waivers)
		filtered := list.Waivers[:0]
		for _, w := range list.Waivers {
			t, ok := parseCreatedOn(w.CreatedOn)
			if !ok || t.Before(cutoff) {
				continue
			}
			filtered = append(filtered, w)
		}
		list.Waivers = filtered
		logging.GetLogger().Info("  API returned %d waivers, %d in last 24h (client-side filter)", before, len(list.Waivers))
	} else {
		logging.GetLogger().Info("  API returned %d waivers", len(list.Waivers))
	}
	var contacts []mailchimp.Contact
	for i, sum := range list.Waivers {
		full, err := c.GetWaiver(sum.WaiverID)
		if err != nil {
			logging.GetLogger().ErrorLogging(5, "smartwaiver.FetchContacts", "GetWaiver "+sum.WaiverID+": "+err.Error())
			continue
		}
		var w Waiver
		if err := json.Unmarshal(full.Waiver, &w); err != nil {
			logging.GetLogger().ErrorLogging(5, "smartwaiver.FetchContacts", "parse waiver "+sum.WaiverID+": "+err.Error())
			continue
		}
		ct := w.ToContact()
		if ct == nil {
			if w.Email == "" {
				logging.GetLogger().ActivityLogging("smartwaiver.FetchContacts", "skip waiver "+sum.WaiverID+": no email")
			} else {
				logging.GetLogger().ActivityLogging("smartwaiver.FetchContacts", "skip waiver "+sum.WaiverID+": marketing not allowed")
			}
			continue
		}
		contacts = append(contacts, *ct)
		logging.GetLogger().Info("  [%d] %s: converted (guardian + %d participants)", i+1, ct.EmailAddress, len(w.Participants))
	}
	return contacts, nil
}

// ToContact converts a Smartwaiver waiver into a Mailchimp contact.
// Parent = guardian; children = first 3 participants.
// Skips waivers with no email or MarketingAllowed=false.
func (w *Waiver) ToContact() *mailchimp.Contact {
	if w.Email == "" {
		return nil
	}
	if !w.MarketingAllowed {
		return nil
	}
	ct := &mailchimp.Contact{
		EmailAddress:     w.Email,
		MarketingAllowed: w.MarketingAllowed,
		Tags:             []string{"SmartWaiver"},
	}
	if w.Guardian != nil {
		ct.FirstName = w.Guardian.FirstName
		ct.LastName = w.Guardian.LastName
		ct.Phone = w.Guardian.Phone
	}
	for i, p := range w.Participants {
		if i >= 3 {
			break
		}
		switch i {
		case 0:
			ct.Child1FirstName = p.FirstName
			ct.Child1LastName = p.LastName
			ct.Child1Bday = p.DOB
		case 1:
			ct.Child2FirstName = p.FirstName
			ct.Child2Bday = p.DOB
		case 2:
			ct.Child3FirstName = p.FirstName
			ct.Child3Bday = p.DOB
		}
	}
	return ct
}
