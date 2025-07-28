package models

type Mobile struct {
	AccountID string `bson:"accountId" json:"accountId"`
	Email     string `bson:"email" json:"email"`
	Password  string `bson:"password" json:"password"`
}
