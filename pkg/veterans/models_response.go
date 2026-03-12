package veterans

type TokenResponse struct {
	TokenType string `json:"token_type"`
	Scope     string `json:"scope"`
	ExpiresIn int    `json:"expires_in"`
}

type DisabilityRatingResponse struct {
	Data Data `json:"data"`
}

type Data struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes Attributes `json:"attributes"`
}

type Attributes struct {
	CombinedDisabilityRating int                     `json:"combined_disability_rating"`
	CombinedEffectiveDate    string                  `json:"combined_effective_date"`
	LegalEffectiveDate       string                  `json:"legal_effective_date"`
	IndividualRatings        []IndividualRatings `json:"individual_ratings"`
}

type IndividualRatings struct {
	Decision               string `json:"decision"`
	DisabilityRatingID     string `json:"disability_rating_id"`
	EffectiveDate          string `json:"effective_date"`
	RatingEndDate          string `json:"rating_end_date"`
	RatingPercentage       int    `json:"rating_percentage"`
	DiagnosticTypeCode     string `json:"diagnostic_type_code"`
	HyphDiagnosticTypeCode string `json:"hyph_diagnostic_type_code"`
	DiagnosticTypeName     string `json:"diagnostic_type_name"`
	DiagnosticText         string `json:"diagnostic_text"`
	StaticInd              bool   `json:"static_ind"`
}
