package auth

const (
	TypeSimplePassword = "simple_password"
	TypeRSA            = "rsa"
)

type Config struct {
	Type string `json:"type"`

	// password
	Password string `json:"password,omitempty"`
	Md5Salt  string `json:"md5_salt,omitempty"`

	// rsa
	PrivateKey string `json:"private_key,omitempty"`
	PublicKey  string `json:"public_key,omitempty"`
}
