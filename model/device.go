package model

type Device struct {
	Id        string `json:"id"`
	Owner     string `json:"owner"`
	CIDR      string `json:"cidr"`
	Signature string `json:"signature"`
}
