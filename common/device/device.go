package device

type Device struct {
	Config
	Id    string `json:"id"`
	Owner string `json:"owner"`
}
