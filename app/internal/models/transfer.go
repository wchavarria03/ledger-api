package models

type TransferMatch struct {
	From Transaction `json:"from"`
	To   Transaction `json:"to"`
}
