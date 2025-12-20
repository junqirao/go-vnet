package config

type (
	AuthType   string
	AuthMethod string
)

const (
	AuthTypeRSA            = "rsa"
	AuthTypeSimplePassword = "password"
)

const (
	AuthMethodIO   = "io"
	AuthMethodHTTP = "http"
)

type Auth struct {
	Type   AuthType   `json:"type"`
	Method AuthMethod `json:"method"`

	// password
	Password string `json:"password,omitempty"`
	Md5Salt  string `json:"md5_salt,omitempty"`

	// rsa
	PrivateKey string `json:"private_key,omitempty"`
	PublicKey  string `json:"public_key,omitempty"`
}
