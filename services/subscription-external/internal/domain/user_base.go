package domain

type UserBase struct {
	Id     int    `json:"id"`
	Msisdn string `json:"msisdn"`
	Type   string `json:"type"`
}
