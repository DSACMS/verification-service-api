package veterans

type DisabilityRatingRequest struct {
	// Required in prod
	// Only BirthDate and ZipCode required in sandbox

	FirstName          string `json:"first_name,omitempty"`
	LastName           string `json:"last_name,omitempty"`
	BirthDate          string `json:"birth_date"`
	StreetAddressLine1 string `json:"street_address_line1,omitempty"`
	City               string `json:"city,omitempty"`
	State              string `json:"state,omitempty"`
	Country            string `json:"country,omitempty"`
	ZipCode            string `json:"zipcode"`

	// Optional in prod

	Gender             string `json:"gender,omitempty"`
	MiddleName         string `json:"middle_name,omitempty"`
	StreetAddressLine2 string `json:"street_address_line2,omitempty"`
	StreetAddressLine3 string `json:"street_address_line3,omitempty"`
}
