package model

type (
	Quota struct {
		Id     int    `json:"id"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		Value  int64  `json:"value"`
		Period string `json:"period"`
	}
	QuotaDetail struct {
		Quota    *Quota            `json:"quota"`
		Usage    int64             `json:"usage"`
		IsExceed bool              `json:"is_exceed"`
		Flow     []*QuotaFlowBrief `json:"flow"`
	}
	QuotaFlow struct {
		QuotaFlowBrief
		Id         int    `json:"id"`
		Quota      int    `json:"quota"`
		Target     string `json:"target"`
		TargetType string `json:"target_type"`
	}
	QuotaFlowBrief struct {
		Usage       int64 `json:"usage"`
		RecordStart int64 `json:"record_start"`
		RecordEnd   int64 `json:"record_end"`
	}
)
