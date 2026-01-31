package auth

const (
	TypeSimplePassword = "simple_password"
	TypeRSA            = "rsa"
)

type Config struct {
	Type string `yaml:"type" json:"type"`

	// password
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// rsa
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	PublicKey  string `yaml:"public_key,omitempty" json:"public_key,omitempty"`
}

func NewEncoderFromConfig(cfg Config) {

}
