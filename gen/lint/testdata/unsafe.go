package unsafefix

// Purposefully unsafe defc schema for lint tests.
type Unsafe interface {
	// GetUser query
	// SELECT * FROM users WHERE name = '{{.name}}'
	GetUser(name string) error

	// CallSite GET https://api.example.com/users/{{.userID}}?token={{.tok}}
	CallSite(userID string, tok string) error

	// Headers GET https://api.example.com/ping
	// X-Thing: {{.hdr}}
	Headers(hdr string) error
}
