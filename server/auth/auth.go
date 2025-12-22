package auth

const (
	TypeSimplePassword = "simple_password"
	TypeRSA            = "rsa"
)

type Config struct {
	Type string `json:"type"`

	// password
	Password string `json:"password,omitempty"`

	// rsa
	PrivateKey string `json:"private_key,omitempty"`
	PublicKey  string `json:"public_key,omitempty"`
}

func NewEncoderFromConfig(cfg Config) {

}
