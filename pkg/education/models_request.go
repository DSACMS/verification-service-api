package education

type Request struct {
	AccountID        string            `json:"accountId"`
	OrganizationName string            `json:"organizationName,omitempty"`
	CaseReferenceID  string            `json:"caseReferenceId"`
	ContactEmail     string            `json:"contactEmail,omitempty"`
	DateOfBirth      string            `json:"dateOfBirth"`
	LastName         string            `json:"lastName"`
	FirstName        string            `json:"firstName"`
	SSN              string            `json:"ssn"`
	IdentityDetails  []IdentityDetails `json:"identityDetails"`
	EndClient        string            `json:"endClient"`
	PreviousNames    []PreviousName    `json:"previousNames,omitempty"`
}

type IdentityDetails struct {
	ElementName  string `json:"elementName"`
	ElementValue string `json:"elementValue"`
}

type PreviousName struct {
	FirstName  string `json:"firstName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
}
