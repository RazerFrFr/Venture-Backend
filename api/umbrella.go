package api

import (
	"VentureBackend/utils"
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func AddUmbrellaApiRoute(router *gin.Engine) {
	router.GET("/api/venturebackend/umbrella", GetUmbrella)
}

func GetUmbrella(c *gin.Context) {
	apikey := c.Query("apikey")
	username := c.Query("username")

	if apikey == "" || apikey != os.Getenv("bApiKey") {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "401", "error": "Invalid or missing API key."})
		return
	}

	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "400", "error": "Missing username."})
		return
	}

	user, err := utils.FindUserByUsername(username)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "User not found."})
		return
	}

	profileDoc, err := utils.FindProfileByAccountID(user.AccountID)
	if err != nil || profileDoc == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "404", "error": "Profile not found."})
		return
	}

	athena, ok := profileDoc.Profiles["athena"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "404", "error": "Athena profile not found."})
		return
	}

	commonCore, ok := profileDoc.Profiles["common_core"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "404", "error": "Common core profile not found."})
		return
	}

	cosmeticId := os.Getenv("UMBRELLA")

	items := athena["items"].(map[string]interface{})
	if _, exists := items[cosmeticId]; exists {
		c.JSON(http.StatusBadRequest, gin.H{"code": "400", "error": "User already owns this cosmetic."})
		return
	}

	purchaseID := uuid.New().String()
	lootList := []map[string]interface{}{
		{
			"itemType": cosmeticId,
			"itemGuid": cosmeticId,
			"quantity": 1,
		},
	}

	items[cosmeticId] = map[string]interface{}{
		"templateId": cosmeticId,
		"attributes": map[string]interface{}{
			"creation_time":   time.Now().UTC().Format(time.RFC3339),
			"max_level_bonus": 0,
			"level":           1,
			"item_seen":       false,
		},
		"quantity": 1,
	}

	commonItems := commonCore["items"].(map[string]interface{})
	commonItems[purchaseID] = map[string]interface{}{
		"templateId": "GiftBox:GB_MakeGood",
		"attributes": map[string]interface{}{
			"fromAccountId": "[Administrator]",
			"lootList":      lootList,
			"params": map[string]interface{}{
				"userMessage": "Thanks For Playing Razer Hosting!",
			},
			"giftedOn": time.Now().UTC().Format(time.RFC3339),
		},
		"quantity": 1,
	}

	athena["rvn"] = toInt(athena["rvn"]) + 1
	athena["commandRevision"] = toInt(athena["commandRevision"]) + 1
	commonCore["rvn"] = toInt(commonCore["rvn"]) + 1
	commonCore["commandRevision"] = toInt(commonCore["commandRevision"]) + 1

	filter := bson.M{"accountId": user.AccountID}
	update := bson.M{
		"$set": bson.M{
			"profiles.athena":      athena,
			"profiles.common_core": commonCore,
		},
	}

	_, err = utils.ProfileCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "500", "error": "Failed to update profile in DB."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                "Successfully added the cosmetic \"" + cosmeticId + "\" to user \"" + username + "\"",
		"profileRevision":        athena["rvn"],
		"profileCommandRevision": athena["commandRevision"],
		"changes": []map[string]interface{}{
			{"changeType": "itemAdded", "itemId": cosmeticId, "templateId": cosmeticId},
			{"changeType": "itemAdded", "itemId": purchaseID, "templateId": "GiftBox:GB_MakeGood"},
		},
	})
}
