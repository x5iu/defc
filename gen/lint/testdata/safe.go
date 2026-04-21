package safefix

// Safe defc schema using approved helpers; lint should report none.
type Safe interface {
	// GetUser query
	// SELECT * FROM users WHERE name = {{ bind .name }}
	GetUser(name string) error

	// CallSite GET https://api.example.com/users/{{ pathseg .userID }}?token={{ query .tok }}
	CallSite(userID string, tok string) error

	// Headers GET https://api.example.com/ping
	// X-Thing: {{ header .hdr }}
	Headers(hdr string) error
}
