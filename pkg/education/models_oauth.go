package education

type OAuthRequest struct {
	Headers OAuthReqHeaders
	Body    OAuthReqBody
}

type OAuthReqHeaders struct {
	ContentType  string
	CacheControl string
	Accept       string
}

type OAuthReqBody struct {
	TokenType   string
	ExpiresIn   string
	AccessToken string
	Scope       string
}

type OAuthResponse struct {
	Headers OAuthResHeaders
	Body    OAuthResBody
}

type OAuthResHeaders struct {
	ContentType string
}

type OAuthResBody struct {
	TokenType   string
	ExpiresIn   string
	AccessToken string
	Scope       string
}
