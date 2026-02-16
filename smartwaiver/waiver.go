package smartwaiver

// Waiver is the full waiver structure from GET v4/waivers/{waiverId}.
// Maps to the Smartwaiver API response; guardian = parent, participants = children.
type Waiver struct {
	WaiverID       string        `json:"waiverId"`
	TemplateID     string        `json:"templateId"`
	Title          string        `json:"title"`
	Email          string        `json:"email"`
	MarketingAllowed bool        `json:"marketingAllowed"`
	Guardian       *Guardian     `json:"guardian"`
	Participants   []Participant `json:"participants"`
}

// Guardian is the parent/guardian on the waiver.
type Guardian struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone"`
}

// Participant is a child/participant on the waiver.
type Participant struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	DOB       string `json:"dob"`
}
