package model

type Quota struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Value  int    `json:"value"`
	Period string `json:"period"`
}
