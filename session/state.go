package session

import (
	"time"

	"github.com/sardanioss/httpcloak/transport"
)

const SessionStateVersion = 2

// SessionState represents the complete saveable session state
type SessionState struct {
	Version         int                                  `json:"version"`
	Preset          string                               `json:"preset"`
	ForceHTTP3      bool                                 `json:"force_http3"`
	ECHConfigDomain string                               `json:"ech_config_domain,omitempty"`
	CreatedAt       time.Time                            `json:"created_at"`
	UpdatedAt       time.Time                            `json:"updated_at"`
	Cookies         []CookieState                        `json:"cookies"`
	TLSSessions     map[string]transport.TLSSessionState `json:"tls_sessions"`
	// ECHConfigs stores ECH configurations per domain (base64 encoded)
	// This is essential for session resumption - the same ECH config must be used
	// when resuming as was used when creating the session ticket
	ECHConfigs map[string]string `json:"ech_configs,omitempty"`
}

// CookieState represents a serializable cookie
type CookieState struct {
	Domain   string     `json:"domain"`
	Path     string     `json:"path"`
	Name     string     `json:"name"`
	Value    string     `json:"value"`
	Expires  *time.Time `json:"expires,omitempty"`
	Secure   bool       `json:"secure,omitempty"`
	HttpOnly bool       `json:"http_only,omitempty"`
}
