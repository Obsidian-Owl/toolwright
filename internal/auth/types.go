package auth

import "time"

// StoredToken holds the OAuth token data persisted to disk for a single provider.
type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Scopes       []string  `json:"scopes"`
}

// IsExpired reports whether the token has expired. A zero Expiry means the
// token never expires and IsExpired returns false in that case.
func (t StoredToken) IsExpired() bool {
	if t.Expiry.IsZero() {
		return false
	}
	return t.Expiry.Before(time.Now())
}

// TokenFile is the top-level structure written to the token cache file on disk.
type TokenFile struct {
	Version int                    `json:"version"`
	Tokens  map[string]StoredToken `json:"tokens"`
}
