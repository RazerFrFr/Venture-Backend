package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Profiles struct {
	ID        primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	AccountID string                 `bson:"accountId" json:"accountId"`
	Profiles  map[string]interface{} `bson:"profiles" json:"profiles"`
}
