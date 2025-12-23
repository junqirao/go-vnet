package model

type Device struct {
	Id        string `json:"id"`
	Owner     string `json:"owner"`
	CIDR      string `json:"cidr"`
	MTU       int    `json:"mtu"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Signature string `json:"signature"`
}
