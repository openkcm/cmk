package oauth2

import "golang.org/x/oauth2"

// ExportToken - it a method  intended to be used for testing purposes only.
// It returns the token stored in the TokenInjector.
func (h *TokenInjector) ExportToken() *oauth2.Token {
	return h.token
}

// SetToken - it a method  intended to be used for testing purposes only.
// It sets the token stored in the TokenInjector, which will trigger token reuse from the beginning.
func (h *TokenInjector) SetToken(token *oauth2.Token) {
	h.token = token
}
