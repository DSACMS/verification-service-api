package veterans

type DisabilityRatingRequest struct {
	// Required

	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	BirthDate          string `json:"birth_date"`
	StreetAddressLine1 string `json:"street_address_line1"`
	City               string `json:"city"`
	State              string `json:"state"`
	Country            string `json:"country"`
	Zipcode            string `json:"zipcode"`

	// Optional

	Gender             string `json:"gender,omitempty"`
	MiddleName         string `json:"middle_name,omitempty"`
	StreetAddressLine2 string `json:"street_address_line2,omitempty"`
	StreetAddressLine3 string `json:"street_address_line3,omitempty"`
}
