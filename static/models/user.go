package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Created       time.Time          `bson:"created" json:"created"`
	Banned        bool               `bson:"banned" json:"banned"`
	DiscordID     *string            `bson:"discordId,omitempty" json:"discordId,omitempty"`
	AccountID     string             `bson:"accountId" json:"accountId"`
	Username      string             `bson:"username" json:"username"`
	Email         string             `bson:"email" json:"email"`
	Password      string             `bson:"password" json:"password"`
	MatchmakingID string             `bson:"matchmakingId" json:"matchmakingId"`
	IsServer      bool               `bson:"isServer" json:"isServer"`
	AcceptedEULA  bool               `bson:"acceptedEULA" json:"acceptedEULA"`
}
