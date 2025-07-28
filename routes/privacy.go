package routes

import (
	"context"
	"net/http"
	"os"

	"VentureBackend/static/tokens"
	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var ProfileCollection *mongo.Collection

func RegisterPrivacyRoutes(router *gin.Engine) {
	db := os.Getenv("DB_NAME")
	ProfileCollection = utils.MongoClient.Database(db).Collection("profiles")
	r := router.Group("/fortnite/api/game/v2/privacy/account/:accountId", tokens.VerifyToken())
	r.GET("", getPrivacy)
	r.POST("", postPrivacy)
}

func getPrivacy(c *gin.Context) {
	accountId := c.Param("accountId")
	if accountId == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	profile, err := utils.FindProfileByAccountID(accountId)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	athena, ok := profile.Profiles["athena"].(map[string]interface{})
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	stats, ok := athena["stats"].(map[string]interface{})
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	attributes, ok := stats["attributes"].(map[string]interface{})
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	optOut, _ := attributes["optOutOfPublicLeaderboards"].(bool)

	c.JSON(http.StatusOK, gin.H{
		"accountId":                  profile.AccountID,
		"optOutOfPublicLeaderboards": optOut,
	})
}

func postPrivacy(c *gin.Context) {
	accountId := c.Param("accountId")
	if accountId == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	profile, err := utils.FindProfileByAccountID(accountId)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var body struct {
		OptOutOfPublicLeaderboards bool `json:"optOutOfPublicLeaderboards"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if athena, ok := profile.Profiles["athena"].(map[string]interface{}); ok {
		if stats, ok := athena["stats"].(map[string]interface{}); ok {
			if attributes, ok := stats["attributes"].(map[string]interface{}); ok {
				attributes["optOutOfPublicLeaderboards"] = body.OptOutOfPublicLeaderboards
			}
		}
	}

	_, err = ProfileCollection.UpdateOne(context.Background(), bson.M{
		"accountId": accountId,
	}, bson.M{
		"$set": bson.M{"profiles": profile.Profiles},
	})
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accountId":                  profile.AccountID,
		"optOutOfPublicLeaderboards": body.OptOutOfPublicLeaderboards,
	})
}
