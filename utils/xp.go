package utils

import (
	"VentureBackend/static/models"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type LevelRequirement struct {
	Level        int `json:"level"`
	XpRequired   int `json:"xpRequired"`
	BookXpReward int `json:"bookXpReward"`
	XpReward     int `json:"xpReward"`
}

var LEVEL_REQUIREMENTS []LevelRequirement
var rewardsData struct {
	FreeRewards []map[string]int `json:"freeRewards"`
	PaidRewards []map[string]int `json:"paidRewards"`
}

func InitLevelData(battlePassSeason int) error {
	data, err := ioutil.ReadFile("./static/XPRequirements/ch1.json")
	if err != nil {
		Error.Logf("Failed to read XP requirements file: %v", err)
		return err
	}
	if err := json.Unmarshal(data, &LEVEL_REQUIREMENTS); err != nil {
		Error.Logf("Failed to parse XP requirements: %v", err)
		return err
	}
	rewardFile := "./static/responses/BattlePass/Season" + strconv.Itoa(battlePassSeason) + ".json"
	rewardBytes, err := ioutil.ReadFile(rewardFile)
	if err != nil {
		Error.Logf("Failed to read rewards file: %v", err)
		return err
	}
	if err := json.Unmarshal(rewardBytes, &rewardsData); err != nil {
		Error.Logf("Failed to parse rewards data: %v", err)
		return err
	}
	return nil
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

func initializeAthena(athena map[string]interface{}) map[string]interface{} {
	if athena == nil {
		athena = make(map[string]interface{})
	}
	if _, ok := athena["stats"]; !ok {
		athena["stats"] = make(map[string]interface{})
	}
	stats := athena["stats"].(map[string]interface{})
	if _, ok := stats["attributes"]; !ok {
		stats["attributes"] = map[string]interface{}{
			"xp":         0.0,
			"level":      1.0,
			"book_xp":    0.0,
			"book_level": 0.0,
		}
	}
	if _, ok := athena["items"]; !ok {
		athena["items"] = make(map[string]interface{})
	}
	if _, ok := athena["rvn"]; !ok {
		athena["rvn"] = 0
	}
	if _, ok := athena["commandRevision"]; !ok {
		athena["commandRevision"] = 0
	}
	return athena
}

func CheckAndLevelUp(profile *models.Profiles) {
	season, _ := strconv.Atoi(os.Getenv("SEASON"))
	InitLevelData(season)
	if profile == nil {
		Error.Log("CheckAndLevelUp: profile is nil")
		return
	}
	athena, ok := profile.Profiles["athena"].(map[string]interface{})
	if !ok {
		Error.Log("CheckAndLevelUp: athena profile not found")
		return
	}
	athena = initializeAthena(athena)
	stats := athena["stats"].(map[string]interface{})
	attributes := stats["attributes"].(map[string]interface{})
	levelsGained := handleLevelUps(attributes, athena)
	bookXP := toInt(attributes["book_xp"])
	bookLevel := toInt(attributes["book_level"])
	if bookXP >= 10 && bookLevel < 100 {
		levelsGained += handleBookLevelUps(attributes, athena, profile, levelsGained)
	}
	if levelsGained > 0 {
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = ProfileCollection.UpdateOne(ctx, bson.M{"accountId": profile.AccountID},
		bson.M{"$set": bson.M{
			"profiles.athena":      athena,
			"profiles.common_core": profile.Profiles["common_core"],
		}})
}

func handleLevelUps(attributes map[string]interface{}, athena map[string]interface{}) int {
	levelsGained := 0
	xp := toInt(attributes["xp"])
	level := toInt(attributes["level"])
	for {
		var req *LevelRequirement
		for _, r := range LEVEL_REQUIREMENTS {
			if r.Level == level {
				req = &r
				break
			}
		}
		if req == nil || xp < req.XpRequired {
			break
		}
		xp -= req.XpRequired
		level++
		levelsGained++
		athena["rvn"] = toInt(athena["rvn"]) + 1
		athena["commandRevision"] = toInt(athena["commandRevision"]) + 1
		bookLevel := toInt(attributes["book_level"])
		if bookLevel >= 100 {
			giveXp(req.XpReward, attributes)
		} else {
			leveledUp(attributes, req.BookXpReward)
		}
		if level >= 100 {
			xp = 0
			break
		}
	}
	attributes["xp"] = float64(xp)
	attributes["level"] = float64(level)
	return levelsGained
}

func handleBookLevelUps(attributes map[string]interface{}, athena map[string]interface{}, profile *models.Profiles, levelsGained int) int {
	bookXP := toInt(attributes["book_xp"])
	toGain := bookXP / 10
	attributes["book_xp"] = float64(bookXP - (toGain * 10))
	bookLevel := toInt(attributes["book_level"]) + toGain
	attributes["book_level"] = float64(bookLevel)
	claimAllRewards(bookLevel, attributes, athena, toGain, profile)
	athena["rvn"] = toInt(athena["rvn"]) + 1
	athena["commandRevision"] = toInt(athena["commandRevision"]) + 1
	return toGain
}

func giveXp(reward int, attributes map[string]interface{}) {
	xp := toInt(attributes["xp"])
	attributes["xp"] = float64(xp + reward)
}

func claimAllRewards(bookLevel int, attributes map[string]interface{}, athena map[string]interface{}, levelsGained int, profile *models.Profiles) {
	for lvl := bookLevel - levelsGained + 1; lvl <= bookLevel; lvl++ {
		claimRewards(lvl, attributes, athena, profile)
	}
}

func claimRewards(bookLevel int, attributes map[string]interface{}, athena map[string]interface{}, profile *models.Profiles) {
	freeRewards := rewardsData.FreeRewards
	paidRewards := rewardsData.PaidRewards
	if bookLevel < 1 || bookLevel > len(freeRewards) {
		Error.Logf("Invalid book level %d for rewards", bookLevel)
		return
	}
	freeReward := freeRewards[bookLevel-1]
	var paidReward map[string]int
	bookPurchased, ok := attributes["book_purchased"].(bool)
	if ok && bookPurchased && bookLevel-1 < len(paidRewards) {
		paidReward = paidRewards[bookLevel-1]
	}
	if freeReward != nil {
		applyRewards(athena, freeReward, profile)
	}
	if paidReward != nil {
		applyRewards(athena, paidReward, profile)
	}
}

func applyRewards(athena map[string]interface{}, rewards map[string]int, profile *models.Profiles) {
	athena = initializeAthena(athena)
	lootList := []map[string]interface{}{}
	commonCore, ok1 := profile.Profiles["common_core"].(map[string]interface{})
	profile0, ok2 := profile.Profiles["profile0"].(map[string]interface{})
	if !ok1 || !ok2 {
		Error.Log("applyRewards: missing common_core or profile0")
		return
	}
	if len(rewards) == 0 {
		Error.Logf("applyRewards: no rewards for account %s", profile.AccountID)
		return
	}
	for item, quantity := range rewards {
		itemLower := strings.ToLower(item)
		switch itemLower {
		case "currency:mtxgiveaway":
			itemsMap := commonCore["items"].(map[string]interface{})
			if _, ok := itemsMap["Currency:MtxPurchased"]; !ok {
				itemsMap["Currency:MtxPurchased"] = map[string]interface{}{"quantity": 0}
				profile0["items"].(map[string]interface{})["Currency:MtxPurchased"] = map[string]interface{}{"quantity": 0}
			}
			commonCoreItems := commonCore["items"].(map[string]interface{})
			profile0Items := profile0["items"].(map[string]interface{})
			commonCoreItems["Currency:MtxPurchased"].(map[string]interface{})["quantity"] =
				toInt(commonCoreItems["Currency:MtxPurchased"].(map[string]interface{})["quantity"]) + quantity
			profile0Items["Currency:MtxPurchased"].(map[string]interface{})["quantity"] =
				toInt(profile0Items["Currency:MtxPurchased"].(map[string]interface{})["quantity"]) + quantity
		case "token:athenaseasonxpboost":
			attrs := athena["stats"].(map[string]interface{})["attributes"].(map[string]interface{})
			attrs["season_match_boost"] = float64(toInt(attrs["season_match_boost"])) + float64(quantity)
		case "token:athenaseasonfriendxpboost":
			attrs := athena["stats"].(map[string]interface{})["attributes"].(map[string]interface{})
			attrs["season_friend_match_boost"] = float64(toInt(attrs["season_friend_match_boost"])) + float64(quantity)
		case "token:athenanextseasonxpboost":
			attrs := athena["stats"].(map[string]interface{})["attributes"].(map[string]interface{})
			attrs["next_season_boost"] = float64(toInt(attrs["next_season_boost"])) + float64(quantity)
		case "accountresource:athenaseasonalxp":
			attrs := athena["stats"].(map[string]interface{})["attributes"].(map[string]interface{})
			attrs["xp"] = float64(toInt(attrs["xp"])) + float64(quantity)
		default:
			if strings.Contains(itemLower, "cosmeticvarianttoken:") {
				return
			}
			handleItemRewards(item, quantity, athena)
		}
		lootList = append(lootList, map[string]interface{}{
			"itemType": item,
			"itemGuid": item,
			"quantity": quantity,
		})
	}
	if len(lootList) > 0 {
		GiftBoxID := GenerateRandomID()
		GiftBox := map[string]interface{}{
			"templateId": "GiftBox:gb_battlepass",
			"attributes": map[string]interface{}{
				"max_level_bonus": 0,
				"fromAccountId":   "",
				"lootList":        lootList,
			},
		}
		commonCore["items"].(map[string]interface{})[GiftBoxID] = GiftBox
	}
	profile.Profiles["common_core"] = commonCore
}

func handleItemRewards(item string, quantity int, athena map[string]interface{}) {
	items := athena["items"].(map[string]interface{})
	itemLower := strings.ToLower(item)
	for _, v := range items {
		m := v.(map[string]interface{})
		if strings.ToLower(m["templateId"].(string)) == itemLower {
			return
		}
	}
	itemId := GenerateRandomID()
	items[itemId] = map[string]interface{}{
		"templateId": item,
		"attributes": map[string]interface{}{
			"max_level_bonus": 0,
			"level":           1,
			"item_seen":       false,
			"xp":              0,
			"variants":        []interface{}{},
			"favorite":        false,
		},
		"quantity": quantity,
	}
}

func leveledUp(attributes map[string]interface{}, amount int) {
	attributes["book_xp"] = float64(toInt(attributes["book_xp"])) + float64(amount)
}
