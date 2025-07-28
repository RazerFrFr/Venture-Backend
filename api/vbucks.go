package api

import (
	"VentureBackend/utils"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

func AddVbucksApiRoute(router *gin.Engine) {
	router.GET("/api/venturebackend/vbucks", GetVbucks)
}

func GetVbucks(c *gin.Context) {
	apikey := c.Query("apikey")
	username := c.Query("username")
	reason := c.Query("reason")

	if apikey == "" || apikey != os.Getenv("bApiKey") {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "401", "error": "Invalid or missing API key."})
		return
	}

	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "400", "error": "Missing username."})
		return
	}
	if reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "400", "error": "Missing reason."})
		return
	}

	var reasons Reasons
	raw := os.Getenv("REASONS")
	if err := json.Unmarshal([]byte(raw), &reasons); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "500", "error": "Invalid REASONS in env"})
		return
	}

	addValue, ok := reasons.Vbucks[reason]
	if !ok {
		valid := []string{}
		for k := range reasons.Vbucks {
			valid = append(valid, k)
		}
		c.JSON(http.StatusBadRequest, gin.H{"code": "400", "error": "Invalid reason. Allowed values: " + strings.Join(valid, ", ")})
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

	commonCore, ok := profileDoc.Profiles["common_core"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "404", "error": "Common core profile not found."})
		return
	}

	items := commonCore["items"].(map[string]interface{})
	mtx, ok := items["Currency:MtxPurchased"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "404", "error": "V-Bucks item missing."})
		return
	}

	currentQuantity := int(mtx["quantity"].(float64))
	newQuantity := currentQuantity + addValue
	mtx["quantity"] = newQuantity

	purchaseID := utils.GenerateRandomID()
	items[purchaseID] = map[string]interface{}{
		"templateId": "GiftBox:GB_MakeGood",
		"attributes": map[string]interface{}{
			"fromAccountId": "[Administrator]",
			"lootList": []map[string]interface{}{
				{
					"itemType": "Currency:MtxGiveaway",
					"itemGuid": "Currency:MtxGiveaway",
					"quantity": addValue,
				},
			},
			"params": map[string]interface{}{
				"userMessage": "Thanks For Playing Razer Hosting!",
			},
			"giftedOn": time.Now().UTC().Format(time.RFC3339),
		},
		"quantity": 1,
	}

	applyProfileChanges := []map[string]interface{}{
		{
			"changeType": "itemQuantityChanged",
			"itemId":     "Currency:MtxPurchased",
			"quantity":   newQuantity,
		},
		{
			"changeType": "itemAdded",
			"itemId":     purchaseID,
			"templateId": "GiftBox:GB_MakeGood",
		},
	}

	commonCore["rvn"] = int(commonCore["rvn"].(float64)) + 1
	commonCore["commandRevision"] = int(commonCore["commandRevision"].(float64)) + 1

	filter := bson.M{"accountId": user.AccountID}
	update := bson.M{"$set": bson.M{"profiles.common_core": commonCore}}

	_, err = utils.ProfileCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "500", "error": "Failed to update profile in DB."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":        commonCore["rvn"],
		"profileCommandRevision": commonCore["commandRevision"],
		"profileChanges":         applyProfileChanges,
		"newQuantityCommonCore":  newQuantity,
	})
}
