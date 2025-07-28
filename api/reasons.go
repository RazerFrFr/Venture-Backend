package api

type Reasons struct {
	Vbucks map[string]int `json:"Vbucks"`
	XP     map[string]int `json:"XP"`
}
