package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

func AddXPApiRoute(router *gin.Engine) {
	router.GET("/api/venturebackend/xp", GetXP)
}

func toInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func GetXP(c *gin.Context) {
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

	baseXP, ok := reasons.XP[reason]
	if !ok {
		valid := []string{}
		for k := range reasons.XP {
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

	athena, ok := profileDoc.Profiles["athena"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "404", "error": "Athena profile not found."})
		return
	}

	stats, ok := athena["stats"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "500", "error": "Invalid stats structure."})
		return
	}

	attributes, ok := stats["attributes"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "500", "error": "Invalid attributes structure."})
		return
	}

	level := toInt(attributes["level"])
	if level >= 100 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "User reached level 100."})
		return
	}

	finalXP := float64(baseXP)
	if boost := toFloat64(attributes["season_match_boost"]); boost > 0 {
		multiplier := 1 + boost/100
		finalXP *= multiplier
	}

	attributes["xp"] = toFloat64(attributes["xp"]) + finalXP

	athena["rvn"] = toInt(athena["rvn"]) + 1
	athena["commandRevision"] = toInt(athena["commandRevision"]) + 1

	filter := bson.M{"accountId": user.AccountID}
	update := bson.M{"$set": bson.M{"profiles.athena": athena}}
	_, err = utils.ProfileCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "500", "error": "Failed to update profile in DB."})
		return
	}

	utils.CheckAndLevelUp(profileDoc)

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":        athena["rvn"],
		"profileCommandRevision": athena["commandRevision"],
		"newXP":                  attributes["xp"],
	})
}
