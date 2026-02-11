package veterans

type TokenResponse struct {
	TokenType string `json:"token_type"`
	Scope     string `json:"scope"`
	ExpiresIn int    `json:"expires_in"`
}

// 200 Status Code - Disability Rating retrieved successfully
type DisabilityRatingResponse_200 struct {
	Data Data_200 `json:"data"`
}

type Data_200 struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes Attributes_200 `json:"attributes"`
}

type Attributes_200 struct {
	CombinedDisabilityRating int                     `json:"combined_disability_rating"`
	CombinedEffectiveDate    string                  `json:"combined_effective_date"`
	LegalEffectiveDate       string                  `json:"legal_effective_date"`
	IndividualRatings        []IndividualRatings_200 `json:"individual_ratings"`
}

type IndividualRatings_200 struct {
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

// 400 Status Code - Bad Request
type DisabilityRatingResponse_400 struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

// 401 Status Code - Not Authorized
type DisabilityRatingResponse_401 struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Path   string `json:"path"`
}

// 403 Status Code - Forbidden
type DisabilityRatingResponse_403 struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Path   string `json:"path"`
}

// 404 Status Code - Not Found
type DisabilityRatingResponse_404 struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

// Status Code 413 - Payload too large
type DisabilityRatingResponse_413 struct {
	Message string `json:"message"`
}

// Status Code 429 - Too many requests
type DisabilityRatingResponse_429 struct {
	Message string `json:"message"`
}

// Status Code 500 - Internal Server Error
type DisabilityRatingResponse_500 struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

// Status Code 502 - Bad Gateway. Also, could indicate Person Not Found in MPI backend.
type DisabilityRatingResponse_502 struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

// Status Code 503 - Service Unavailable
type DisabilityRatingResponse_503 struct {
	Errors []Errors_503 `json:"errors"`
}
type Errors_503 struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

// Status Code 504 - Gateway Timeout
type DisabilityRatingResponse_504 struct {
	Message string `json:"message"`
}
