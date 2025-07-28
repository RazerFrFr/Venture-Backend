package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"VentureBackend/static/models"
	"VentureBackend/static/profiles"
	"VentureBackend/static/tokens"
	"VentureBackend/utils"
	"VentureBackend/ws/xmpp"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var GiftReceived sync.Map

func RegisterMCPRoutes(router *gin.Engine) {
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/SetAffiliateName", SetAffiliateName)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/GiftCatalogEntry", tokens.VerifyToken(), GiftCatalogEntry)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/RemoveGiftBox", tokens.VerifyToken(), RemoveGiftBox)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/RefundMtxPurchase", tokens.VerifyToken(), RefundMtxPurchase)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/PurchaseCatalogEntry", tokens.VerifyToken(), PurchaseCatalogEntry)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/MarkItemSeen", tokens.VerifyToken(), MarkItemSeen)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/SetItemFavoriteStatusBatch", tokens.VerifyToken(), SetItemFavoriteStatusBatch)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/SetBattleRoyaleBanner", tokens.VerifyToken(), SetBattleRoyaleBanner)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/EquipBattleRoyaleCustomization", tokens.VerifyToken(), EquipBattleRoyaleCustomization)
	router.POST("/fortnite/api/game/v2/profile/:accountId/client/:operation", tokens.VerifyToken(), MCPHandler)
	router.POST("/fortnite/api/game/v2/profile/:accountId/dedicated_server/:operation", DedicatedServerHandler)
}

func SetAffiliateName(c *gin.Context) {
	utils.CreateError(c,
		"errors.com.epicgames.fortnite.sac_disabled",
		"SAC Codes are disabled",
		nil,
		12801,
		"Bad Request",
		400)
}

func GiftCatalogEntry(c *gin.Context) {
	accountId := c.Param("accountId")
	profileId := c.Query("profileId")
	rvnQuery := c.Query("rvn")

	userProfiles, err := utils.FindProfileByAccountID(accountId)
	if err != nil || userProfiles == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.not_found",
			"Profile not found",
			[]string{accountId},
			12804,
			"Not Found",
			404)
		return
	}

	if !profiles.ValidateProfile(profileId, userProfiles) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileId),
			[]string{profileId},
			12813,
			"Forbidden",
			403)
		return
	}

	if profileId != "common_core" {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_command",
			fmt.Sprintf("GiftCatalogEntry is not valid on %s profile", profileId),
			[]string{"GiftCatalogEntry", profileId},
			12801,
			"Bad Request",
			400)
		return
	}

	profileRaw := userProfiles.Profiles[profileId]
	profile := ConvertMapToProfile(profileRaw)

	var body struct {
		OfferId            string   `json:"offerId"`
		ReceiverAccountIds []string `json:"receiverAccountIds"`
		GiftWrapTemplateId string   `json:"giftWrapTemplateId"`
		PersonalMessage    string   `json:"personalMessage"`
	}

	if err := c.BindJSON(&body); err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.invalid_payload",
			"Invalid request body",
			nil,
			12800,
			"Bad Request",
			400)
		return
	}

	missing := checkFields([]string{"offerId", "receiverAccountIds", "giftWrapTemplateId"}, map[string]interface{}{
		"offerId":            body.OfferId,
		"receiverAccountIds": body.ReceiverAccountIds,
		"giftWrapTemplateId": body.GiftWrapTemplateId,
	})
	if len(missing) > 0 {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			fmt.Sprintf("Validation Failed. [%s] field(s) is missing.", strings.Join(missing, ", ")),
			missing,
			1040,
			"Bad Request",
			400)
		return
	}

	if len(body.PersonalMessage) > 100 {
		utils.CreateError(c,
			"errors.com.epicgames.string.length_check",
			"The personalMessage you provided is longer than 100 characters.",
			nil,
			16027,
			"Bad Request",
			400)
		return
	}

	validGiftBoxes := []string{"GiftBox:gb_default", "GiftBox:gb_giftwrap1", "GiftBox:gb_giftwrap2", "GiftBox:gb_giftwrap3"}
	if !contains(validGiftBoxes, body.GiftWrapTemplateId) {
		utils.CreateError(c,
			"errors.com.epicgames.giftbox.invalid",
			"The giftbox you provided is invalid.",
			nil,
			16027,
			"Bad Request",
			400)
		return
	}

	if len(body.ReceiverAccountIds) < 1 || len(body.ReceiverAccountIds) > 5 {
		utils.CreateError(c,
			"errors.com.epicgames.item.quantity.range_check",
			"You need to at least gift to 1 person and can not gift to more than 5 people.",
			nil,
			16027,
			"Bad Request",
			400)
		return
	}

	if checkIfDuplicateExists(body.ReceiverAccountIds) {
		utils.CreateError(c,
			"errors.com.epicgames.array.duplicate_found",
			"There are duplicate accountIds in receiverAccountIds.",
			nil,
			16027,
			"Bad Request",
			400)
		return
	}

	var friends struct {
		List struct {
			Accepted []struct {
				AccountId string `json:"accountId"`
			} `json:"accepted"`
		} `json:"list"`
	}
	_ = utils.FriendsCollection.FindOne(context.TODO(), bson.M{"accountId": accountId}).Decode(&friends)
	acceptedFriends := map[string]bool{}
	for _, f := range friends.List.Accepted {
		acceptedFriends[f.AccountId] = true
	}

	for _, receiverId := range body.ReceiverAccountIds {
		if _, ok := acceptedFriends[receiverId]; !ok && receiverId != accountId {
			utils.CreateError(c,
				"errors.com.epicgames.friends.no_relationship",
				fmt.Sprintf("User %s is not friends with %s", accountId, receiverId),
				[]string{accountId, receiverId},
				28004,
				"Forbidden",
				403)
			return
		}
	}

	_, findOfferId := utils.GetOfferID(body.OfferId)
	if findOfferId == nil {
		utils.CreateError(c,
			"errors.com.epicgames.fortnite.id_invalid",
			fmt.Sprintf("Offer ID (id: '%s') not found", body.OfferId),
			[]string{body.OfferId},
			16027,
			"Bad Request",
			400)
		return
	}

	applyProfileChanges := []map[string]interface{}{}
	notifications := []interface{}{}

	price := getIntFromMap(findOfferId.Prices[0], "finalPrice") * len(body.ReceiverAccountIds)
	currencyType := strings.ToLower(getString(findOfferId.Prices[0], "currencyType"))

	if currencyType == "mtxcurrency" && price > 0 {
		items := ensureMap(profile, "items")
		paid := false

		for itemId, itemRaw := range items {
			item, _ := itemRaw.(map[string]interface{})
			tid := strings.ToLower(getString(item, "templateId"))
			if !strings.HasPrefix(tid, "currency:mtx") {
				continue
			}

			attr := ensureMap(item, "attributes")
			platform := strings.ToLower(getString(attr, "platform"))
			stats := ensureMap(ensureMap(profile, "stats"), "attributes")
			currentPlatform := strings.ToLower(getString(stats, "current_mtx_platform"))

			if platform != "shared" && platform != currentPlatform {
				continue
			}

			quantity := getIntFromMap(item, "quantity")
			if quantity < price {
				utils.CreateError(c,
					"errors.com.epicgames.currency.mtx.insufficient",
					fmt.Sprintf("You can not afford this item (%d), you only have %d.", price, quantity),
					[]string{strconv.Itoa(price), strconv.Itoa(quantity)},
					1040,
					"Bad Request",
					400)
				return
			}

			item["quantity"] = quantity - price
			applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
				"changeType": "itemQuantityChanged",
				"itemId":     itemId,
				"quantity":   item["quantity"],
			})
			paid = true
			break
		}

		if !paid {
			utils.CreateError(c,
				"errors.com.epicgames.currency.mtx.insufficient",
				"You can not afford this item.",
				nil,
				1040,
				"Bad Request",
				400)
			return
		}
	}

	for _, receiverId := range body.ReceiverAccountIds {
		receiverProfiles, err := utils.FindProfileByAccountID(receiverId)
		if err != nil || receiverProfiles == nil {
			continue
		}
		athena := ConvertMapToProfile(receiverProfiles.Profiles["athena"])
		commonCore := ConvertMapToProfile(receiverProfiles.Profiles["common_core"])

		if attr := ensureMap(ensureMap(commonCore, "stats"), "attributes"); !attr["allowed_to_receive_gifts"].(bool) {
			utils.CreateError(c,
				"errors.com.epicgames.user.gift_disabled",
				fmt.Sprintf("User %s has disabled receiving gifts.", receiverId),
				[]string{receiverId},
				28004,
				"Forbidden",
				403)
			return
		}

		giftBoxId := utils.GenerateRandomID()
		giftBoxItem := map[string]interface{}{
			"templateId": body.GiftWrapTemplateId,
			"attributes": map[string]interface{}{
				"fromAccountId": accountId,
				"lootList":      []interface{}{},
				"params": map[string]interface{}{
					"userMessage": body.PersonalMessage,
				},
				"level":    1,
				"giftedOn": time.Now().UTC().Format(time.RFC3339),
			},
			"quantity": 1,
		}

		itemsAthena := ensureMap(athena, "items")
		itemsCore := ensureMap(commonCore, "items")

		for _, grant := range findOfferId.ItemGrants {
			templateId, _ := grant["templateId"].(string)
			newId := utils.GenerateRandomID()
			item := map[string]interface{}{
				"templateId": templateId,
				"attributes": map[string]interface{}{
					"item_seen": false,
					"variants":  []interface{}{},
				},
				"quantity": 1,
			}
			itemsAthena[newId] = item

			loot := map[string]interface{}{
				"itemType":    templateId,
				"itemGuid":    newId,
				"itemProfile": "athena",
				"quantity":    1,
			}

			giftBoxItemAttrs := giftBoxItem["attributes"].(map[string]interface{})
			giftBoxItemAttrs["lootList"] = append(
				giftBoxItemAttrs["lootList"].([]interface{}),
				loot,
			)
		}

		itemsCore[giftBoxId] = giftBoxItem

		receiverProfiles.Profiles["athena"] = athena
		receiverProfiles.Profiles["common_core"] = commonCore
		filter := bson.M{"accountId": receiverId}
		update := bson.M{"$set": bson.M{"profiles.athena": athena, "profiles.common_core": commonCore}}
		_, _ = utils.ProfileCollection.UpdateOne(context.TODO(), filter, update)

		xmpp.SendXmppMessageToId(map[string]interface{}{
			"type":      "com.epicgames.gift.received",
			"payload":   map[string]interface{}{},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}, receiverId)
	}

	profile["rvn"] = getIntFromMap(profile, "rvn") + 1
	profile["commandRevision"] = getIntFromMap(profile, "commandRevision") + 1
	profile["updated"] = time.Now().UTC().Format(time.RFC3339)
	filter := bson.M{"accountId": accountId}
	update := bson.M{"$set": bson.M{fmt.Sprintf("profiles.%s", profileId): profile}}
	_, _ = utils.ProfileCollection.UpdateOne(context.TODO(), filter, update)

	queryRevision, _ := strconv.Atoi(rvnQuery)
	baseRevision := getIntFromMap(profile, "rvn")
	profileRevisionCheck := getIntFromMap(profile, "commandRevision")
	if queryRevision != profileRevisionCheck {
		applyProfileChanges = []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(200, gin.H{
		"profileRevision":            getIntFromMap(profile, "rvn"),
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"notifications":              notifications,
		"profileCommandRevision":     getIntFromMap(profile, "commandRevision"),
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"responseVersion":            1,
	})
}

func RemoveGiftBox(c *gin.Context) {
	accountId := c.Param("accountId")
	profileId := c.Query("profileId")
	rvnQuery := c.Query("rvn")

	profiles, err := utils.FindProfileByAccountID(accountId)
	if err != nil {
		utils.CreateError(c, "errors.com.epicgames.profiles.not_found", "Profiles not found", nil, 12800, "Not Found", 404)
		return
	}

	profileRaw, ok := profiles.Profiles[profileId]
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileId),
			[]string{profileId}, 12813, "Forbidden", 403,
		)
		return
	}

	profile, ok := profileRaw.(map[string]interface{})
	if !ok {
		utils.CreateError(c, "errors.com.epicgames.profiles.invalid_format", "Profile data invalid", nil, 12814, "Internal Server Error", 500)
		return
	}

	if profileId != "athena" && profileId != "common_core" && profileId != "profile0" {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_command",
			fmt.Sprintf("RemoveGiftBox is not valid on %s profile", profileId),
			[]string{"RemoveGiftBox", profileId}, 12801, "Bad Request", 400,
		)
		return
	}

	var reqBody struct {
		GiftBoxItemId  string   `json:"giftBoxItemId"`
		GiftBoxItemIds []string `json:"giftBoxItemIds"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		utils.CreateError(c, "errors.com.epicgames.bad_request", "Invalid request body", nil, 40000, "Bad Request", 400)
		return
	}

	itemsRaw, ok := profile["items"]
	if !ok {
		utils.CreateError(c, "errors.com.epicgames.profiles.not_found", "Profile items missing", nil, 12800, "Not Found", 404)
		return
	}

	items, ok := itemsRaw.(map[string]interface{})
	if !ok {
		utils.CreateError(c, "errors.com.epicgames.profiles.invalid_format", "Profile items format invalid", nil, 12814, "Internal Server Error", 500)
		return
	}

	applyProfileChanges := []map[string]interface{}{}
	itemRemoved := false

	if reqBody.GiftBoxItemId != "" {
		itemRaw, exists := items[reqBody.GiftBoxItemId]
		if !exists {
			utils.CreateError(c,
				"errors.com.epicgames.fortnite.id_invalid",
				fmt.Sprintf("Item (id: '%s') not found", reqBody.GiftBoxItemId),
				[]string{reqBody.GiftBoxItemId}, 16027, "Bad Request", 400,
			)
			return
		}
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			utils.CreateError(c, "errors.com.epicgames.profiles.invalid_format", "Item format invalid", nil, 12814, "Internal Server Error", 500)
			return
		}
		templateId, _ := item["templateId"].(string)
		if !strings.HasPrefix(templateId, "GiftBox:") {
			utils.CreateError(c,
				"errors.com.epicgames.fortnite.id_invalid",
				"The specified item id is not a giftbox.",
				[]string{reqBody.GiftBoxItemId}, 16027, "Bad Request", 400,
			)
			return
		}

		delete(items, reqBody.GiftBoxItemId)
		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "itemRemoved",
			"itemId":     reqBody.GiftBoxItemId,
		})
		itemRemoved = true
	}

	if len(reqBody.GiftBoxItemIds) > 0 {
		for _, giftBoxItemId := range reqBody.GiftBoxItemIds {
			itemRaw, exists := items[giftBoxItemId]
			if !exists {
				continue
			}
			item, ok := itemRaw.(map[string]interface{})
			if !ok {
				continue
			}
			templateId, _ := item["templateId"].(string)
			if !strings.HasPrefix(templateId, "GiftBox:") {
				continue
			}

			delete(items, giftBoxItemId)
			applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
				"changeType": "itemRemoved",
				"itemId":     giftBoxItemId,
			})
			itemRemoved = true
		}
	}

	if itemRemoved {
		rvn := getIntFromMap(profile, "rvn")
		commandRevision := getIntFromMap(profile, "commandRevision")
		profile["rvn"] = rvn + 1
		profile["commandRevision"] = commandRevision + 1
		profile["updated"] = time.Now().UTC().Format(time.RFC3339)
		profile["items"] = items

		updateField := fmt.Sprintf("profiles.%s", profileId)
		_, err := ProfileCollection.UpdateOne(c.Request.Context(),
			bson.M{"accountId": accountId},
			bson.M{"$set": bson.M{updateField: profile}})
		if err != nil {
			utils.CreateError(c, "errors.com.epicgames.profiles.update_failed", "Failed to update profile", nil, 12815, "Internal Server Error", 500)
			return
		}
	}

	baseRevision := getIntFromMap(profile, "rvn")
	profileRevisionCheck := baseRevision
	if memoryBuild := utils.GetVersionInfo(c.Request).Build; memoryBuild >= 12.20 {
		profileRevisionCheck = getIntFromMap(profile, "commandRevision")
	}

	queryRevision := -1
	if rvnQuery != "" {
		if parsed, err := strconv.Atoi(rvnQuery); err == nil {
			queryRevision = parsed
		}
	}

	if queryRevision != profileRevisionCheck {
		applyProfileChanges = []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":            baseRevision,
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     getIntFromMap(profile, "commandRevision"),
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"responseVersion":            1,
	})
}

func RefundMtxPurchase(c *gin.Context) {
	accountID := c.Param("accountId")
	profileID := c.Query("profileId")
	queryRVNStr := c.Query("rvn")

	var body struct {
		PurchaseId string `json:"purchaseId"`
	}
	if err := c.BindJSON(&body); err != nil || body.PurchaseId == "" {
		utils.CreateError(c,
			"errors.com.epicgames.common.unsupported_client",
			"Invalid or missing purchaseId",
			nil, 18007, "Bad Request", 400)
		return
	}

	profileDoc, err := utils.FindProfileByAccountID(accountID)
	if err != nil || profileDoc == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Failed to load profiles",
			nil, 12813, "Forbidden", 403)
		return
	}

	if !profiles.ValidateProfile(profileID, profileDoc) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Invalid profile "+profileID,
			[]string{profileID}, 12813, "Forbidden", 403)
		return
	}

	rawProfile, ok1 := profileDoc.Profiles[profileID].(map[string]interface{})
	rawAthena, ok2 := profileDoc.Profiles["athena"].(map[string]interface{})
	if !ok1 || !ok2 {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Profile data malformed",
			nil, 12813, "Not Found", 404)
		return
	}

	profile := rawProfile
	athena := rawAthena

	memory := utils.GetVersionInfo(c.Request)
	baseRVN := getIntFromInterfaceSafe(profile["rvn"], 0)
	queryRVN := getIntFromInterfaceSafe(queryRVNStr, -1)

	profileRevisionCheck := baseRVN
	if memory.Build >= 12.20 {
		profileRevisionCheck = getIntFromInterfaceSafe(profile["commandRevision"], 0)
	}

	multiUpdate := []map[string]interface{}{
		{
			"profileRevision":            getIntFromInterfaceSafe(athena["rvn"], 0),
			"profileId":                  "athena",
			"profileChangesBaseRevision": getIntFromInterfaceSafe(athena["rvn"], 0),
			"profileChanges":             []interface{}{},
			"profileCommandRevision":     getIntFromInterfaceSafe(athena["commandRevision"], 0),
		},
	}

	applyProfileChanges := []interface{}{}
	itemGuids := []string{}

	stats := ensureMap(profile, "stats")
	attributes := ensureMap(stats, "attributes")
	mtxHistory := ensureMap(attributes, "mtx_purchase_history")

	refundsUsed := getIntFromInterfaceSafe(mtxHistory["refundsUsed"], 0)
	refundCredits := getIntFromInterfaceSafe(mtxHistory["refundCredits"], 0)

	purchasesRaw, ok := mtxHistory["purchases"]
	if !ok {
		purchasesRaw = []interface{}{}
	}

	var purchases []interface{}
	switch v := purchasesRaw.(type) {
	case primitive.A:
		purchases = []interface{}(v)
	case []interface{}:
		purchases = v
	default:
		purchases = []interface{}{}
	}

	for _, p := range purchases {
		purchase, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if toString(purchase["purchaseId"]) == body.PurchaseId {
			purchase["refundDate"] = time.Now().UTC().Format(time.RFC3339)
			mtxHistory["refundsUsed"] = refundsUsed + 1
			mtxHistory["refundCredits"] = refundCredits - 1

			items := ensureMap(profile, "items")

			lootResultVal, exists := purchase["lootResult"]
			var lootResult []interface{}
			if exists {
				switch v := lootResultVal.(type) {
				case primitive.A:
					lootResult = []interface{}(v)
				case []interface{}:
					lootResult = v
				default:
					lootResult = []interface{}{}
				}
			}

			for _, loot := range lootResult {
				lootMap, ok := loot.(map[string]interface{})
				if !ok {
					continue
				}
				itemGuids = append(itemGuids, toString(lootMap["itemGuid"]))
			}

			currentPlatform := strings.ToLower(toString(attributes["current_mtx_platform"]))

			for key, val := range items {
				item, ok := val.(map[string]interface{})
				if !ok {
					continue
				}
				templateId := strings.ToLower(toString(item["templateId"]))
				if strings.HasPrefix(templateId, "currency:mtx") {
					itemAttrs := ensureMap(item, "attributes")
					platform := strings.ToLower(toString(itemAttrs["platform"]))
					if platform == currentPlatform || platform == "shared" {
						refundAmount := getIntFromInterfaceSafe(purchase["totalMtxPaid"], 0)
						item["quantity"] = getIntFromInterfaceSafe(item["quantity"], 0) + refundAmount
						items[key] = item

						applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
							"changeType": "itemQuantityChanged",
							"itemId":     key,
							"quantity":   item["quantity"],
						})
						break
					}
				}
			}
			break
		}
	}

	if len(itemGuids) > 0 {
		athenaItems := ensureMap(athena, "items")
		for _, guid := range itemGuids {
			delete(athenaItems, guid)
			multiUpdate[0]["profileChanges"] = append(
				multiUpdate[0]["profileChanges"].([]interface{}),
				map[string]interface{}{
					"changeType": "itemRemoved",
					"itemId":     guid,
				},
			)
		}
	}

	if len(applyProfileChanges) > 0 {
		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       "mtx_purchase_history",
			"value":      mtxHistory,
		})

		multiUpdate[0]["profileRevision"] = getIntFromInterfaceSafe(athena["rvn"], 0) + 1
		multiUpdate[0]["profileCommandRevision"] = getIntFromInterfaceSafe(athena["commandRevision"], 0) + 1

		profile["rvn"] = baseRVN + 1
		profile["commandRevision"] = getIntFromInterfaceSafe(profile["commandRevision"], 0) + 1
		athena["rvn"] = getIntFromInterfaceSafe(athena["rvn"], 0) + 1
		athena["commandRevision"] = getIntFromInterfaceSafe(athena["commandRevision"], 0) + 1

		now := time.Now().UTC().Format(time.RFC3339)
		profile["updated"] = now
		athena["updated"] = now

		_, err = ProfileCollection.UpdateOne(c.Request.Context(), bson.M{"accountId": accountID}, bson.M{
			"$set": bson.M{
				fmt.Sprintf("profiles.%s", profileID): profile,
				"profiles.athena":                     athena,
			},
		})
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.modules.profiles.update_failed",
				"Failed to update profiles",
				nil, 50001, "Internal Server Error", 500)
			return
		}
	}

	if queryRVN != profileRevisionCheck {
		applyProfileChanges = []interface{}{
			map[string]interface{}{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":            profile["rvn"],
		"profileId":                  profileID,
		"profileChangesBaseRevision": baseRVN,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     profile["commandRevision"],
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"multiUpdate":                multiUpdate,
		"responseVersion":            1,
	})
}

func PurchaseCatalogEntry(c *gin.Context) {
	accountID := c.Param("accountId")
	profileID := c.Query("profileId")

	rvnQueryStr := c.Query("rvn")
	rvnQuery, _ := strconv.Atoi(rvnQueryStr)

	profileDoc, err := utils.FindProfileByAccountID(accountID)
	if err != nil || profileDoc == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileID),
			[]string{profileID}, 12813, "Forbidden", 403)
		return
	}

	if !profiles.ValidateProfile(profileID, profileDoc) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileID),
			[]string{profileID}, 12813, "Forbidden", 403)
		return
	}

	profile := profileDoc.Profiles[profileID].(map[string]interface{})
	athena := profileDoc.Profiles["athena"].(map[string]interface{})

	if profileID != "common_core" && profileID != "profile0" {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_command",
			fmt.Sprintf("PurchaseCatalogEntry is not valid on %s profile", profileID),
			[]string{"PurchaseCatalogEntry", profileID},
			12801, "Bad Request", 400)
		return
	}

	multiUpdate := []map[string]interface{}{
		{
			"profileRevision":            getIntFromInterfaceSafe(athena["rvn"], 0),
			"profileId":                  "athena",
			"profileChangesBaseRevision": getIntFromInterfaceSafe(athena["rvn"], 0),
			"profileChanges":             []interface{}{},
			"profileCommandRevision":     getIntFromInterfaceSafe(athena["commandRevision"], 0),
		},
	}

	memory := utils.GetVersionInfo(c.Request)
	notifications := []interface{}{}
	applyProfileChanges := []interface{}{}

	initialAthenaRvn := getIntFromInterfaceSafe(athena["rvn"], 0)
	initialAthenaCmdRev := getIntFromInterfaceSafe(athena["commandRevision"], 0)
	initialProfileRvn := getIntFromInterfaceSafe(profile["rvn"], 0)
	initialProfileCmdRev := getIntFromInterfaceSafe(profile["commandRevision"], 0)

	baseRevision := initialProfileRvn
	profileRevisionCheck := initialProfileCmdRev
	if memory.Build < 12.20 {
		profileRevisionCheck = baseRevision
	}

	var body struct {
		OfferID          string `json:"offerId"`
		PurchaseQuantity int    `json:"purchaseQuantity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.OfferID == "" {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. [offerId] field(s) is missing.",
			[]string{"[offerId]"}, 1040, "Bad Request", 400)
		return
	}
	if body.PurchaseQuantity < 1 {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. 'purchaseQuantity' is less than 1.",
			[]string{"purchaseQuantity"}, 1040, "Bad Request", 400)
		return
	}

	if profile["items"] == nil {
		profile["items"] = map[string]interface{}{}
	}
	if athena["items"] == nil {
		athena["items"] = map[string]interface{}{}
	}

	offer, found := utils.GetOfferID(body.OfferID)
	if found == nil {
		utils.CreateError(c,
			"errors.com.epicgames.fortnite.id_invalid",
			fmt.Sprintf("Offer ID (id: '%s') not found", body.OfferID),
			[]string{body.OfferID}, 16027, "Bad Request", 400)
		return
	}

	battlePassSeason := os.Getenv("SEASON")
	seasonStr := fmt.Sprintf("Season%s", battlePassSeason)

	if fmt.Sprint(memory.Season) == battlePassSeason {
		filePath := filepath.Join(".", "static", "responses", "BattlePass", seasonStr+".json")
		data, err := os.ReadFile(filePath)
		if err == nil {
			var battlePassData map[string]interface{}
			if json.Unmarshal(data, &battlePassData) == nil {
				battlePassOfferID, _ := battlePassData["battlePassOfferId"].(string)
				battleBundleOfferID, _ := battlePassData["battleBundleOfferId"].(string)
				tierOfferID, _ := battlePassData["tierOfferId"].(string)

				if body.OfferID == battlePassOfferID || body.OfferID == battleBundleOfferID || body.OfferID == tierOfferID {
					priceMap := found.Prices[0]
					finalPrice := int(priceMap["finalPrice"].(float64)) * body.PurchaseQuantity

					HandleAllBattlePassPurchases(c, accountID, profileID, profile, found, finalPrice, &applyProfileChanges)

					if body.OfferID == battlePassOfferID || body.OfferID == battleBundleOfferID {
						HandleBattlePassAndBundlePurchases(c, accountID, profileID, profile, athena, battlePassData, body.OfferID, &multiUpdate, &applyProfileChanges)
					}
					if body.OfferID == tierOfferID {
						HandleTierPurchases(c, profile, athena, battlePassData, &multiUpdate, &applyProfileChanges, body.PurchaseQuantity)
					}
				}
			}
		}
	}

	if regexp.MustCompile(`^BR(Daily|Weekly|Season)Storefront$`).MatchString(offer) {
		_ = handleStorefrontPurchases(found, profile, athena, multiUpdate, &notifications, &applyProfileChanges)
	}

	if len(multiUpdate[0]["profileChanges"].([]interface{})) > 0 &&
		initialAthenaRvn == getIntFromInterfaceSafe(athena["rvn"], 0) &&
		initialAthenaCmdRev == getIntFromInterfaceSafe(athena["commandRevision"], 0) {
		athena["rvn"] = initialAthenaRvn + 1
		athena["commandRevision"] = initialAthenaCmdRev + 1
		athena["updated"] = time.Now().UTC().Format(time.RFC3339)
		multiUpdate[0]["profileRevision"] = athena["rvn"]
		multiUpdate[0]["profileChangesBaseRevision"] = athena["rvn"]
		multiUpdate[0]["profileCommandRevision"] = athena["commandRevision"]
	}

	if len(applyProfileChanges) > 0 &&
		initialProfileRvn == getIntFromInterfaceSafe(profile["rvn"], 0) &&
		initialProfileCmdRev == getIntFromInterfaceSafe(profile["commandRevision"], 0) {
		profile["rvn"] = initialProfileRvn + 1
		profile["commandRevision"] = initialProfileCmdRev + 1
		profile["updated"] = time.Now().UTC().Format(time.RFC3339)
	}

	if len(applyProfileChanges) > 0 || len(multiUpdate[0]["profileChanges"].([]interface{})) > 0 {
		_, err = ProfileCollection.UpdateOne(c.Request.Context(), bson.M{"accountId": accountID}, bson.M{
			"$set": bson.M{
				fmt.Sprintf("profiles.%s", profileID): profile,
				"profiles.athena":                     athena,
			},
		})
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.modules.profiles.update_failed",
				"Failed to update profile",
				nil, 50001, "Internal Server Error", 500)
			return
		}
	}

	if rvnQuery != profileRevisionCheck {
		applyProfileChanges = []interface{}{
			map[string]interface{}{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":            getIntFromInterfaceSafe(profile["rvn"], 0),
		"profileId":                  profileID,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"notifications":              notifications,
		"profileCommandRevision":     getIntFromInterfaceSafe(profile["commandRevision"], 0),
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"multiUpdate":                multiUpdate,
		"responseVersion":            1,
	})
}

func HandleAllBattlePassPurchases(
	c *gin.Context,
	accountId, profileId string,
	profile map[string]interface{},
	findOfferId *utils.CatalogEntry,
	totalPrice int,
	applyProfileChanges *[]interface{},
) error {
	priceMap := findOfferId.Prices[0]
	currencyType := getString(priceMap, "currencyType")

	if totalPrice == 0 || !strings.EqualFold(currencyType, "MtxCurrency") {
		return nil
	}

	paid := false
	items := ConvertMapToProfile(ConvertMapToProfile(profile)["items"])
	attributes := ConvertMapToProfile(ConvertMapToProfile(profile)["stats"])["attributes"]
	currentPlatform := strings.ToLower(getString(ConvertMapToProfile(attributes), "current_mtx_platform"))

	for itemId, itemRaw := range items {
		item := ConvertMapToProfile(itemRaw)
		templateId := strings.ToLower(getString(item, "templateId"))
		if !strings.HasPrefix(templateId, "currency:mtx") {
			continue
		}

		itemAttributes := ConvertMapToProfile(item["attributes"])
		platform := strings.ToLower(getString(itemAttributes, "platform"))

		if platform != currentPlatform && platform != "shared" {
			continue
		}

		quantity := int(getIntFromMap(item, "quantity"))
		if quantity < totalPrice {
			return fmt.Errorf("currency.mtx.insufficient: you cannot afford this item (%d), you only have %d", totalPrice, quantity)
		}

		item["quantity"] = quantity - totalPrice
		items[itemId] = item

		*applyProfileChanges = append(*applyProfileChanges, map[string]interface{}{
			"changeType": "itemQuantityChanged",
			"itemId":     itemId,
			"quantity":   item["quantity"],
		})

		paid = true
		break
	}

	if !paid {
		return fmt.Errorf("currency.mtx.insufficient: you cannot afford this item (%d)", totalPrice)
	}

	return nil
}

func HandleBattlePassAndBundlePurchases(
	c *gin.Context,
	accountId, profileId string,
	profile, athena, battlePass map[string]interface{},
	offerId string,
	MultiUpdate *[]map[string]interface{},
	ApplyProfileChanges *[]interface{},
) {
	lootList := []map[string]interface{}{}

	athenaStats := ConvertMapToProfile(athena["stats"])
	athenaAttributes := ConvertMapToProfile(athenaStats["attributes"])

	bookLevel := getIntFromMap(athenaAttributes, "book_level")
	athenaAttributes["book_purchased"] = true

	athenaStats["attributes"] = athenaAttributes
	athena["stats"] = athenaStats
	items := ConvertMapToProfile(profile["items"])

	seasonNumber := 0
	for i := 0; i < len(offerId); i++ {
		if offerId[i] >= '0' && offerId[i] <= '9' {
			start := i
			for i < len(offerId) && offerId[i] >= '0' && offerId[i] <= '9' {
				i++
			}
			seasonNumber, _ = strconv.Atoi(offerId[start:i])
			break
		}
	}

	tokenKey := fmt.Sprintf("Token:Athena_S%d_NoBattleBundleOption_Token", seasonNumber)
	tokenData := map[string]interface{}{
		"templateId": fmt.Sprintf("Token:athena_s%d_nobattlebundleoption_token", seasonNumber),
		"attributes": map[string]interface{}{
			"max_level_bonus": 0,
			"level":           1,
			"item_seen":       true,
			"xp":              0,
			"favorite":        false,
		},
		"quantity": 1,
	}

	items[tokenKey] = tokenData
	profile["items"] = items

	*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
		"changeType": "itemAdded",
		"itemId":     tokenKey,
		"item":       tokenData,
	})

	battleBundleOfferId, _ := battlePass["battleBundleOfferId"].(string)
	if offerId == battleBundleOfferId {
		bookLevel += 25
		if bookLevel > 100 {
			bookLevel = 100
		}
	}
	athenaAttributes["book_level"] = bookLevel
	athenaStats["attributes"] = athenaAttributes
	athena["stats"] = athenaStats

	freeRewardsRaw, _ := battlePass["freeRewards"]
	paidRewardsRaw, _ := battlePass["paidRewards"]

	freeRewardsList, _ := freeRewardsRaw.([]interface{})
	paidRewardsList, _ := paidRewardsRaw.([]interface{})

	for i := 0; i < bookLevel; i++ {
		var freeTier map[string]interface{}
		var paidTier map[string]interface{}

		if i < len(freeRewardsList) {
			freeTier, _ = freeRewardsList[i].(map[string]interface{})
		} else {
			freeTier = map[string]interface{}{}
		}
		if i < len(paidRewardsList) {
			paidTier, _ = paidRewardsList[i].(map[string]interface{})
		} else {
			paidTier = map[string]interface{}{}
		}

		for item, rawAmount := range freeTier {
			amount := int(rawAmount.(float64))
			itemLower := strings.ToLower(item)

			if itemLower == "token:athenaseasonxpboost" {
				val := getIntFromMap(athenaAttributes, "season_match_boost") + amount
				athenaAttributes["season_match_boost"] = val
				(*MultiUpdate)[0]["profileChanges"] = append(
					(*MultiUpdate)[0]["profileChanges"].([]interface{}),
					map[string]interface{}{
						"changeType": "statModified",
						"name":       "season_match_boost",
						"value":      val,
					},
				)
			}
			if itemLower == "token:athenaseasonfriendxpboost" {
				val := getIntFromMap(athenaAttributes, "season_friend_match_boost") + amount
				athenaAttributes["season_friend_match_boost"] = val
				(*MultiUpdate)[0]["profileChanges"] = append(
					(*MultiUpdate)[0]["profileChanges"].([]interface{}),
					map[string]interface{}{
						"changeType": "statModified",
						"name":       "season_friend_match_boost",
						"value":      val,
					},
				)
			}

			if strings.HasPrefix(itemLower, "currency:mtx") {
				profileItems := ConvertMapToProfile(profile["items"])
				profileStats := ConvertMapToProfile(profile["stats"])
				profileAttributes := ConvertMapToProfile(profileStats["attributes"])
				currentPlatform := strings.ToLower(getString(profileAttributes, "current_mtx_platform"))

				for key, itemRaw := range profileItems {
					profileItem := ConvertMapToProfile(itemRaw)
					templateId := strings.ToLower(getString(profileItem, "templateId"))
					if strings.HasPrefix(templateId, "currency:mtx") {
						platform := strings.ToLower(getString(ConvertMapToProfile(profileItem["attributes"]), "platform"))
						if platform == currentPlatform || platform == "shared" {
							quantity := getIntFromMap(profileItem, "quantity")
							profileItem["quantity"] = quantity + amount
							profileItems[key] = profileItem
							profile["items"] = profileItems
							break
						}
					}
				}
			}

			if strings.HasPrefix(itemLower, "homebasebanner") {
				profileItems := ConvertMapToProfile(profile["items"])
				ItemExists := false
				for key, itemRaw := range profileItems {
					profileItem := ConvertMapToProfile(itemRaw)
					if strings.ToLower(getString(profileItem, "templateId")) == itemLower {
						profileItemAttributes := ConvertMapToProfile(profileItem["attributes"])
						profileItemAttributes["item_seen"] = false
						profileItem["attributes"] = profileItemAttributes
						profileItems[key] = profileItem
						ItemExists = true
						*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
							"changeType":     "itemAttrChanged",
							"itemId":         key,
							"attributeName":  "item_seen",
							"attributeValue": false,
						})
					}
				}
				if !ItemExists {
					itemID := utils.GenerateRandomID()
					item := map[string]interface{}{
						"templateId": item,
						"attributes": map[string]interface{}{"item_seen": false},
						"quantity":   1,
					}
					profileItems[itemID] = item
					*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
						"changeType": "itemAdded",
						"itemId":     itemID,
						"item":       item,
					})
					profile["items"] = profileItems
				}
			}

			if strings.HasPrefix(itemLower, "athena") {
				athenaItems := ConvertMapToProfile(athena["items"])
				ItemExists := false
				for key, itemRaw := range athenaItems {
					athenaItem := ConvertMapToProfile(itemRaw)
					if strings.ToLower(getString(athenaItem, "templateId")) == itemLower {
						athenaItemAttributes := ConvertMapToProfile(athenaItem["attributes"])
						athenaItemAttributes["item_seen"] = false
						athenaItem["attributes"] = athenaItemAttributes
						athenaItems[key] = athenaItem
						ItemExists = true
						(*MultiUpdate)[0]["profileChanges"] = append(
							(*MultiUpdate)[0]["profileChanges"].([]interface{}),
							map[string]interface{}{
								"changeType":     "itemAttrChanged",
								"itemId":         key,
								"attributeName":  "item_seen",
								"attributeValue": false,
							},
						)
					}
				}
				if !ItemExists {
					itemID := utils.GenerateRandomID()
					item := map[string]interface{}{
						"templateId": item,
						"attributes": map[string]interface{}{
							"max_level_bonus": 0,
							"level":           1,
							"item_seen":       false,
							"xp":              0,
							"variants":        []interface{}{},
							"favorite":        false,
						},
						"quantity": amount,
					}
					athenaItems[itemID] = item
					(*MultiUpdate)[0]["profileChanges"] = append(
						(*MultiUpdate)[0]["profileChanges"].([]interface{}),
						map[string]interface{}{
							"changeType": "itemAdded",
							"itemId":     itemID,
							"item":       item,
						},
					)
					athena["items"] = athenaItems
				}
			}

			lootList = append(lootList, map[string]interface{}{
				"itemType": item,
				"itemGuid": item,
				"quantity": amount,
			})
		}

		for item, rawAmount := range paidTier {
			amount := int(rawAmount.(float64))
			itemLower := strings.ToLower(item)

			if itemLower == "token:athenaseasonxpboost" {
				val := getIntFromMap(athenaAttributes, "season_match_boost") + amount
				athenaAttributes["season_match_boost"] = val
				(*MultiUpdate)[0]["profileChanges"] = append(
					(*MultiUpdate)[0]["profileChanges"].([]interface{}),
					map[string]interface{}{
						"changeType": "statModified",
						"name":       "season_match_boost",
						"value":      val,
					},
				)
			}
			if itemLower == "token:athenaseasonfriendxpboost" {
				val := getIntFromMap(athenaAttributes, "season_friend_match_boost") + amount
				athenaAttributes["season_friend_match_boost"] = val
				(*MultiUpdate)[0]["profileChanges"] = append(
					(*MultiUpdate)[0]["profileChanges"].([]interface{}),
					map[string]interface{}{
						"changeType": "statModified",
						"name":       "season_friend_match_boost",
						"value":      val,
					},
				)
			}

			if strings.HasPrefix(itemLower, "currency:mtx") {
				profileItems := ConvertMapToProfile(profile["items"])
				profileStats := ConvertMapToProfile(profile["stats"])
				profileAttributes := ConvertMapToProfile(profileStats["attributes"])
				currentPlatform := strings.ToLower(getString(profileAttributes, "current_mtx_platform"))
				for key, itemRaw := range profileItems {
					profileItem := ConvertMapToProfile(itemRaw)
					templateId := strings.ToLower(getString(profileItem, "templateId"))
					if strings.HasPrefix(templateId, "currency:mtx") {
						platform := strings.ToLower(getString(ConvertMapToProfile(profileItem["attributes"]), "platform"))
						if platform == currentPlatform || platform == "shared" {
							qty := getIntFromMap(profileItem, "quantity")
							profileItem["quantity"] = qty + amount
							profileItems[key] = profileItem
							profile["items"] = profileItems
							break
						}
					}
				}
			}

			if strings.HasPrefix(itemLower, "homebasebanner") {
				profileItems := ConvertMapToProfile(profile["items"])
				ItemExists := false
				for key, itemRaw := range profileItems {
					profileItem := ConvertMapToProfile(itemRaw)
					if strings.ToLower(getString(profileItem, "templateId")) == itemLower {
						profileItemAttributes := ConvertMapToProfile(profileItem["attributes"])
						profileItemAttributes["item_seen"] = false
						profileItem["attributes"] = profileItemAttributes
						profileItems[key] = profileItem
						ItemExists = true
						*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
							"changeType":     "itemAttrChanged",
							"itemId":         key,
							"attributeName":  "item_seen",
							"attributeValue": false,
						})
					}
				}
				if !ItemExists {
					itemID := utils.GenerateRandomID()
					item := map[string]interface{}{
						"templateId": item,
						"attributes": map[string]interface{}{"item_seen": false},
						"quantity":   1,
					}
					profileItems[itemID] = item
					*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
						"changeType": "itemAdded",
						"itemId":     itemID,
						"item":       item,
					})
					profile["items"] = profileItems
				}
			}

			if strings.HasPrefix(itemLower, "athena") {
				athenaItems := ConvertMapToProfile(athena["items"])
				ItemExists := false
				for key, itemRaw := range athenaItems {
					athenaItem := ConvertMapToProfile(itemRaw)
					if strings.ToLower(getString(athenaItem, "templateId")) == itemLower {
						athenaItemAttributes := ConvertMapToProfile(athenaItem["attributes"])
						athenaItemAttributes["item_seen"] = false
						athenaItem["attributes"] = athenaItemAttributes
						athenaItems[key] = athenaItem
						ItemExists = true
						(*MultiUpdate)[0]["profileChanges"] = append(
							(*MultiUpdate)[0]["profileChanges"].([]interface{}),
							map[string]interface{}{
								"changeType":     "itemAttrChanged",
								"itemId":         key,
								"attributeName":  "item_seen",
								"attributeValue": false,
							},
						)
					}
				}
				if !ItemExists {
					itemID := utils.GenerateRandomID()
					item := map[string]interface{}{
						"templateId": item,
						"attributes": map[string]interface{}{
							"max_level_bonus": 0,
							"level":           1,
							"item_seen":       false,
							"xp":              0,
							"variants":        []interface{}{},
							"favorite":        false,
						},
						"quantity": amount,
					}
					athenaItems[itemID] = item
					(*MultiUpdate)[0]["profileChanges"] = append(
						(*MultiUpdate)[0]["profileChanges"].([]interface{}),
						map[string]interface{}{
							"changeType": "itemAdded",
							"itemId":     itemID,
							"item":       item,
						},
					)
					athena["items"] = athenaItems
				}
			}

			lootList = append(lootList, map[string]interface{}{
				"itemType": item,
				"itemGuid": item,
				"quantity": amount,
			})
		}
	}

	giftBoxID := utils.GenerateRandomID()
	var giftBoxTemplate string
	if 8 <= 4 {
		giftBoxTemplate = "GiftBox:gb_battlepass"
	} else {
		giftBoxTemplate = "GiftBox:gb_battlepasspurchased"
	}
	giftBox := map[string]interface{}{
		"templateId": giftBoxTemplate,
		"attributes": map[string]interface{}{
			"max_level_bonus": 0,
			"fromAccountId":   "",
			"lootList":        lootList,
		},
	}
	if 8 > 2 {
		profileItems := ConvertMapToProfile(profile["items"])
		profileItems[giftBoxID] = giftBox
		profile["items"] = profileItems
		*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
			"changeType": "itemAdded",
			"itemId":     giftBoxID,
			"item":       giftBox,
		})
	}

	(*MultiUpdate)[0]["profileChanges"] = append(
		(*MultiUpdate)[0]["profileChanges"].([]interface{}),
		map[string]interface{}{
			"changeType": "statModified",
			"name":       "book_purchased",
			"value":      true,
		},
	)
	(*MultiUpdate)[0]["profileChanges"] = append(
		(*MultiUpdate)[0]["profileChanges"].([]interface{}),
		map[string]interface{}{
			"changeType": "statModified",
			"name":       "book_level",
			"value":      bookLevel,
		},
	)
}

func HandleTierPurchases(
	c *gin.Context,
	profile, athena map[string]interface{},
	BattlePass map[string]interface{},
	MultiUpdate *[]map[string]interface{},
	ApplyProfileChanges *[]interface{},
	purchaseQuantity int,
) {
	lootList := []map[string]interface{}{}

	athenaStats := ConvertMapToProfile(athena["stats"])
	athenaAttributes := ConvertMapToProfile(athenaStats["attributes"])

	startingTier := getIntFromMap(athenaAttributes, "book_level")
	if purchaseQuantity < 1 {
		purchaseQuantity = 1
	}

	endingTier := startingTier + purchaseQuantity
	athenaAttributes["book_level"] = endingTier

	athenaStats["attributes"] = athenaAttributes
	athena["stats"] = athenaStats

	profileItems := ConvertMapToProfile(profile["items"])
	profileStats := ConvertMapToProfile(profile["stats"])
	profileAttributes := ConvertMapToProfile(profileStats["attributes"])

	freeRewardsList, _ := BattlePass["freeRewards"].([]interface{})
	paidRewardsList, _ := BattlePass["paidRewards"].([]interface{})

	for i := startingTier; i < endingTier; i++ {
		var freeTier, paidTier map[string]interface{}

		if i < len(freeRewardsList) {
			freeTier, _ = freeRewardsList[i].(map[string]interface{})
		} else {
			freeTier = map[string]interface{}{}
		}
		if i < len(paidRewardsList) {
			paidTier, _ = paidRewardsList[i].(map[string]interface{})
		} else {
			paidTier = map[string]interface{}{}
		}

		processTier := func(tier map[string]interface{}) {
			for item, rawAmount := range tier {
				amount := int(rawAmount.(float64))
				itemLower := strings.ToLower(item)
				ItemExists := false

				if itemLower == "token:athenaseasonxpboost" {
					currentVal := getIntFromMap(athenaAttributes, "season_match_boost")
					athenaAttributes["season_match_boost"] = currentVal + amount

					(*MultiUpdate)[0]["profileChanges"] = append(
						(*MultiUpdate)[0]["profileChanges"].([]interface{}),
						map[string]interface{}{
							"changeType": "statModified",
							"name":       "season_match_boost",
							"value":      athenaAttributes["season_match_boost"],
						},
					)
				}
				if itemLower == "token:athenaseasonfriendxpboost" {
					currentVal := getIntFromMap(athenaAttributes, "season_friend_match_boost")
					athenaAttributes["season_friend_match_boost"] = currentVal + amount

					(*MultiUpdate)[0]["profileChanges"] = append(
						(*MultiUpdate)[0]["profileChanges"].([]interface{}),
						map[string]interface{}{
							"changeType": "statModified",
							"name":       "season_friend_match_boost",
							"value":      athenaAttributes["season_friend_match_boost"],
						},
					)
				}

				if strings.HasPrefix(itemLower, "currency:mtx") {
					currentPlatform := strings.ToLower(getString(profileAttributes, "current_mtx_platform"))
					for key, itemRaw := range profileItems {
						profileItem := ConvertMapToProfile(itemRaw)
						templateId := strings.ToLower(getString(profileItem, "templateId"))
						if strings.HasPrefix(templateId, "currency:mtx") {
							itemAttributes := ConvertMapToProfile(profileItem["attributes"])
							platform := strings.ToLower(getString(itemAttributes, "platform"))
							if platform == currentPlatform || platform == "shared" {
								qty := getIntFromMap(profileItem, "quantity")
								profileItem["quantity"] = qty + amount
								profileItems[key] = profileItem
								break
							}
						}
					}
				}

				if strings.HasPrefix(itemLower, "homebasebanner") {
					for key, itemRaw := range profileItems {
						profileItem := ConvertMapToProfile(itemRaw)
						if strings.ToLower(getString(profileItem, "templateId")) == itemLower {
							itemAttributes := ConvertMapToProfile(profileItem["attributes"])
							itemAttributes["item_seen"] = false
							profileItem["attributes"] = itemAttributes
							profileItems[key] = profileItem
							ItemExists = true
							*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
								"changeType":     "itemAttrChanged",
								"itemId":         key,
								"attributeName":  "item_seen",
								"attributeValue": false,
							})
						}
					}
					if !ItemExists {
						itemID := utils.GenerateRandomID()
						item := map[string]interface{}{
							"templateId": item,
							"attributes": map[string]interface{}{"item_seen": false},
							"quantity":   1,
						}
						profileItems[itemID] = item
						*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
							"changeType": "itemAdded",
							"itemId":     itemID,
							"item":       item,
						})
					}
				}

				if strings.HasPrefix(itemLower, "athena") {
					athenaItems := ConvertMapToProfile(athena["items"])
					for key, itemRaw := range athenaItems {
						athenaItem := ConvertMapToProfile(itemRaw)
						if strings.ToLower(getString(athenaItem, "templateId")) == itemLower {
							itemAttributes := ConvertMapToProfile(athenaItem["attributes"])
							itemAttributes["item_seen"] = false
							athenaItem["attributes"] = itemAttributes
							athenaItems[key] = athenaItem
							ItemExists = true
							(*MultiUpdate)[0]["profileChanges"] = append(
								(*MultiUpdate)[0]["profileChanges"].([]interface{}),
								map[string]interface{}{
									"changeType":     "itemAttrChanged",
									"itemId":         key,
									"attributeName":  "item_seen",
									"attributeValue": false,
								},
							)
						}
					}
					if !ItemExists {
						itemID := utils.GenerateRandomID()
						item := map[string]interface{}{
							"templateId": item,
							"attributes": map[string]interface{}{
								"max_level_bonus": 0,
								"level":           1,
								"item_seen":       false,
								"xp":              0,
								"variants":        []interface{}{},
								"favorite":        false,
							},
							"quantity": amount,
						}
						athenaItems[itemID] = item
						(*MultiUpdate)[0]["profileChanges"] = append(
							(*MultiUpdate)[0]["profileChanges"].([]interface{}),
							map[string]interface{}{
								"changeType": "itemAdded",
								"itemId":     itemID,
								"item":       item,
							},
						)
						athena["items"] = athenaItems
					}
				}

				lootList = append(lootList, map[string]interface{}{
					"itemType": item,
					"itemGuid": item,
					"quantity": amount,
				})
			}
		}

		processTier(freeTier)
		processTier(paidTier)
	}

	giftBoxID := utils.GenerateRandomID()
	giftBox := map[string]interface{}{
		"templateId": "GiftBox:gb_battlepass",
		"attributes": map[string]interface{}{
			"max_level_bonus": 0,
			"fromAccountId":   "",
			"lootList":        lootList,
		},
	}

	profileItems[giftBoxID] = giftBox
	profile["items"] = profileItems
	*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
		"changeType": "itemAdded",
		"itemId":     giftBoxID,
		"item":       giftBox,
	})

	(*MultiUpdate)[0]["profileChanges"] = append(
		(*MultiUpdate)[0]["profileChanges"].([]interface{}),
		map[string]interface{}{
			"changeType": "statModified",
			"name":       "book_level",
			"value":      athenaAttributes["book_level"],
		},
	)
}

func handleStorefrontPurchases(
	findOfferId *utils.CatalogEntry,
	profile, athena map[string]interface{},
	MultiUpdate []map[string]interface{},
	Notifications *[]interface{},
	ApplyProfileChanges *[]interface{},
) error {

	loot := map[string]interface{}{
		"type":    "CatalogPurchase",
		"primary": true,
		"lootResult": map[string]interface{}{
			"items": []interface{}{},
		},
	}
	*Notifications = append(*Notifications, loot)

	notification := (*Notifications)[0].(map[string]interface{})
	lootResult := notification["lootResult"].(map[string]interface{})

	for _, grant := range findOfferId.ItemGrants {
		templateId := grant["templateId"].(string)

		for _, v := range athena["items"].(map[string]interface{}) {
			item := v.(map[string]interface{})
			if strings.EqualFold(item["templateId"].(string), templateId) {
				return errors.New("errors.com.epicgames.offer.already_owned")
			}
		}

		ID := utils.GenerateRandomID()
		newItem := map[string]interface{}{
			"templateId": templateId,
			"attributes": map[string]interface{}{
				"item_seen": false,
				"variants":  []interface{}{},
			},
			"quantity": 1,
		}

		athena["items"].(map[string]interface{})[ID] = newItem

		athenaChanges := MultiUpdate[0]["profileChanges"].([]interface{})
		athenaChanges = append(athenaChanges, map[string]interface{}{
			"changeType": "itemAdded",
			"itemId":     ID,
			"item":       newItem,
		})
		MultiUpdate[0]["profileChanges"] = athenaChanges

		lootItems := lootResult["items"].([]interface{})
		lootItems = append(lootItems, map[string]interface{}{
			"itemType":    templateId,
			"itemGuid":    ID,
			"itemProfile": "athena",
			"quantity":    1,
		})
		lootResult["items"] = lootItems
	}

	if len(findOfferId.Prices) == 0 {
		return nil
	}

	priceInfo := findOfferId.Prices[0]
	if strings.ToLower(priceInfo["currencyType"].(string)) == "mtxcurrency" {
		finalPrice := int(priceInfo["finalPrice"].(float64))

		if finalPrice > 0 {
			paid := false
			attr := profile["stats"].(map[string]interface{})["attributes"].(map[string]interface{})
			currentPlatform := strings.ToLower(attr["current_mtx_platform"].(string))

			for key, item := range profile["items"].(map[string]interface{}) {
				itemMap := item.(map[string]interface{})
				if !strings.HasPrefix(strings.ToLower(itemMap["templateId"].(string)), "currency:mtx") {
					continue
				}

				platform := strings.ToLower(itemMap["attributes"].(map[string]interface{})["platform"].(string))
				if platform != currentPlatform && platform != "shared" {
					continue
				}

				quantity := getIntFromInterfaceSafe(itemMap["quantity"], 0)
				if quantity < finalPrice {
					return fmt.Errorf("errors.com.epicgames.currency.mtx.insufficient: have %d, need %d", quantity, finalPrice)
				}

				itemMap["quantity"] = quantity - finalPrice

				*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
					"changeType": "itemQuantityChanged",
					"itemId":     key,
					"quantity":   itemMap["quantity"],
				})

				paid = true
				break
			}

			if !paid {
				return fmt.Errorf("errors.com.epicgames.currency.mtx.insufficient: need %d", finalPrice)
			}
		}

		attr := profile["stats"].(map[string]interface{})["attributes"].(map[string]interface{})
		mtxHistory, ok := attr["mtx_purchase_history"].(map[string]interface{})
		if !ok || mtxHistory == nil {
			mtxHistory = make(map[string]interface{})
			mtxHistory["purchases"] = []interface{}{}
			attr["mtx_purchase_history"] = mtxHistory
		}

		purchases := toInterfaceSlice(mtxHistory["purchases"])
		purchaseId := utils.GenerateRandomID()

		purchases = append(purchases, map[string]interface{}{
			"purchaseId":         purchaseId,
			"offerId":            "v2:/" + purchaseId,
			"purchaseDate":       time.Now().UTC().Format(time.RFC3339),
			"freeRefundEligible": false,
			"fulfillments":       []interface{}{},
			"lootResult":         lootResult["items"],
			"totalMtxPaid":       finalPrice,
			"metadata":           map[string]interface{}{},
			"gameContext":        "",
		})

		mtxHistory["purchases"] = purchases

		*ApplyProfileChanges = append(*ApplyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       "mtx_purchase_history",
			"value":      mtxHistory,
		})
	}

	return nil
}

func MarkItemSeen(c *gin.Context) {
	accountID := c.Param("accountId")
	profileID := c.Query("profileId")
	queryRVNStr := c.Query("rvn")

	var body map[string]interface{}
	if err := c.BindJSON(&body); err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.common.unsupported_client",
			"Invalid JSON",
			nil, 18007, "Bad Request", 400)
		return
	}

	userProfiles, err := utils.FindProfileByAccountID(accountID)
	if err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Failed to load profiles",
			nil, 12813, "Forbidden", 403)
		return
	}

	if !profiles.ValidateProfile(profileID, userProfiles) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Unable to find template configuration for profile "+profileID,
			[]string{profileID}, 12813, "Forbidden", 403)
		return
	}

	profileMap := userProfiles.Profiles
	rawProfile, ok := profileMap[profileID]
	if !ok || rawProfile == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Profile not found: "+profileID,
			nil, 12813, "Not Found", 404)
		return
	}

	profile := ConvertMapToProfile(rawProfile)

	stats := ensureMap(profile, "stats")
	attributes := ensureMap(stats, "attributes")

	memory := utils.GetVersionInfo(c.Request)
	if profileID == "athena" {
		attributes["season_num"] = memory.Season
	}

	missing := CheckFields([]string{"itemIds"}, body)
	if len(missing) > 0 {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. ["+strings.Join(missing, ", ")+"] field(s) is missing.",
			[]string{"[" + strings.Join(missing, ", ") + "]"}, 1040, "Bad Request", 400)
		return
	}

	itemIds, ok := body["itemIds"].([]interface{})
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"itemIds must be an array",
			nil, 1040, "Bad Request", 400)
		return
	}

	items := ensureMap(profile, "items")
	var changes []map[string]interface{}

	for _, rawID := range itemIds {
		id := toString(rawID)
		itemData, exists := items[id]
		if !exists {
			continue
		}
		item := ConvertMapToProfile(itemData)
		attrs := ensureMap(item, "attributes")
		attrs["item_seen"] = true

		changes = append(changes, map[string]interface{}{
			"changeType":     "itemAttrChanged",
			"itemId":         id,
			"attributeName":  "item_seen",
			"attributeValue": true,
		})

		items[id] = item
	}

	baseRVN := getIntFromInterfaceSafe(profile["rvn"], 0)
	cmdRevision := getIntFromInterfaceSafe(profile["commandRevision"], 0)
	queryRVN := getIntFromInterfaceSafe(queryRVNStr, -1)

	if len(changes) > 0 {
		profile["rvn"] = baseRVN + 1
		profile["commandRevision"] = cmdRevision + 1
		profile["updated"] = time.Now().Format(time.RFC3339)

		profileMap[profileID] = profile

		_, err = ProfileCollection.UpdateOne(context.TODO(), bson.M{"accountId": accountID}, bson.M{
			"$set": bson.M{
				fmt.Sprintf("profiles.%s", profileID): profile,
			},
		})
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.modules.profiles.update_failed",
				"Failed to update profile",
				nil, 50001, "Internal Server Error", 500)
			return
		}
	}

	profileChanges := changes
	if queryRVN != cmdRevision {
		profileChanges = []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":            profile["rvn"],
		"profileId":                  profileID,
		"profileChangesBaseRevision": baseRVN,
		"profileChanges":             profileChanges,
		"profileCommandRevision":     profile["commandRevision"],
		"serverTime":                 time.Now().Format(time.RFC3339),
		"responseVersion":            1,
	})
}

func SetItemFavoriteStatusBatch(c *gin.Context) {
	accountID := c.Param("accountId")
	profileID := c.Query("profileId")
	queryRVNStr := c.Query("rvn")

	var body map[string]interface{}
	if err := c.BindJSON(&body); err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.common.unsupported_client",
			"Invalid JSON",
			nil, 18007, "Bad Request", 400)
		return
	}

	userProfiles, err := utils.FindProfileByAccountID(accountID)
	if err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Failed to load profiles",
			nil, 12813, "Forbidden", 403)
		return
	}

	if !profiles.ValidateProfile(profileID, userProfiles) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Unable to find template configuration for profile "+profileID,
			[]string{profileID}, 12813, "Forbidden", 403)
		return
	}

	if profileID != "athena" {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_command",
			"SetItemFavoriteStatusBatch is not valid on "+profileID+" profile",
			[]string{"SetItemFavoriteStatusBatch", profileID}, 12801, "Bad Request", 400)
		return
	}

	profileMap := userProfiles.Profiles

	rawProfile, ok := profileMap[profileID]
	if !ok || rawProfile == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Profile not found: "+profileID,
			nil, 12813, "Not Found", 404)
		return
	}

	profile := ConvertMapToProfile(rawProfile)

	stats := ensureMap(profile, "stats")
	attributes := ensureMap(stats, "attributes")

	memory := utils.GetVersionInfo(c.Request)
	if memory.Build >= 12.20 {
		attributes["season_num"] = memory.Season
	}

	missing := CheckFields([]string{"itemIds", "itemFavStatus"}, body)
	if len(missing) > 0 {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. ["+strings.Join(missing, ", ")+"] field(s) is missing.",
			[]string{"[" + strings.Join(missing, ", ") + "]"}, 1040, "Bad Request", 400)
		return
	}

	itemIds, ok1 := body["itemIds"].([]interface{})
	itemFavStatus, ok2 := body["itemFavStatus"].([]interface{})
	if !ok1 || !ok2 {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"Invalid input types for itemIds or itemFavStatus",
			nil, 1040, "Bad Request", 400)
		return
	}

	items := ensureMap(profile, "items")
	var changes []map[string]interface{}

	for i := 0; i < len(itemIds) && i < len(itemFavStatus); i++ {
		id := toString(itemIds[i])
		if _, exists := items[id]; !exists {
			continue
		}

		itemData := ensureMap(items, id)
		attrs := ensureMap(itemData, "attributes")

		if fav, ok := itemFavStatus[i].(bool); ok {
			attrs["favorite"] = fav
			changes = append(changes, map[string]interface{}{
				"changeType":     "itemAttrChanged",
				"itemId":         id,
				"attributeName":  "favorite",
				"attributeValue": fav,
			})
		}
	}

	baseRVN := getIntFromInterfaceSafe(profile["rvn"], 0)
	cmdRevision := getIntFromInterfaceSafe(profile["commandRevision"], 0)
	queryRVN := getIntFromInterfaceSafe(queryRVNStr, -1)

	if len(changes) > 0 {
		profile["rvn"] = baseRVN + 1
		profile["commandRevision"] = cmdRevision + 1
		profile["updated"] = time.Now().Format(time.RFC3339)

		profileMap[profileID] = profile

		_, err := ProfileCollection.UpdateOne(context.TODO(), bson.M{"accountId": accountID}, bson.M{
			"$set": bson.M{
				fmt.Sprintf("profiles.%s", profileID): profile,
			},
		})
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.modules.profiles.update_failed",
				"Failed to update profile",
				nil, 50001, "Internal Server Error", 500)
			return
		}
	}

	profileChanges := changes
	if queryRVN != cmdRevision {
		profileChanges = []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(200, gin.H{
		"profileRevision":            profile["rvn"],
		"profileId":                  profileID,
		"profileChangesBaseRevision": baseRVN,
		"profileChanges":             profileChanges,
		"profileCommandRevision":     profile["commandRevision"],
		"serverTime":                 time.Now().Format(time.RFC3339),
		"responseVersion":            1,
	})
}

func SetBattleRoyaleBanner(c *gin.Context) {
	accountId := c.Param("accountId")
	profileId := c.Query("profileId")
	rvnQuery := c.Query("rvn")

	userProfiles, err := utils.FindProfileByAccountID(accountId)
	if err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.not_found",
			fmt.Sprintf("Profiles for account %s not found", accountId),
			nil, 12800, "Not Found", 404)
		return
	}

	if !profiles.ValidateProfile(profileId, userProfiles) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileId),
			[]string{profileId}, 12813, "Forbidden", 403)
		return
	}

	if profileId != "athena" {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_command",
			fmt.Sprintf("SetBattleRoyaleBanner is not valid on %s profile", profileId),
			[]string{"SetBattleRoyaleBanner", profileId}, 12801, "Bad Request", 400)
		return
	}

	profileRaw, ok := userProfiles.Profiles[profileId]
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Profile %s not found in profiles map", profileId),
			[]string{profileId}, 12813, "Forbidden", 403)
		return
	}

	profile := ConvertMapToProfile(profileRaw)
	if profile == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Invalid profile data structure",
			nil, 12814, "Internal Server Error", 500)
		return
	}

	memory := utils.GetVersionInfo(c.Request)

	if profileId == "athena" {
		statsMap := ensureMap(profile, "stats")
		profileStatsAttributes := ensureMap(statsMap, "attributes")
		profileStatsAttributes["season_num"] = memory.Season
	}

	var body struct {
		HomebaseBannerIconId  string `json:"homebaseBannerIconId"`
		HomebaseBannerColorId string `json:"homebaseBannerColorId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"Invalid request body",
			nil, 1040, "Bad Request", 400)
		return
	}

	missingFields := checkFields([]string{"homebaseBannerIconId", "homebaseBannerColorId"}, map[string]interface{}{
		"homebaseBannerIconId":  body.HomebaseBannerIconId,
		"homebaseBannerColorId": body.HomebaseBannerColorId,
	})
	if len(missingFields) > 0 {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			fmt.Sprintf("Validation Failed. [%s] field(s) is missing.", strings.Join(missingFields, ", ")),
			[]string{fmt.Sprintf("[%s]", strings.Join(missingFields, ", "))}, 1040, "Bad Request", 400)
		return
	}

	if body.HomebaseBannerIconId == "" {
		ValidationError(c, "homebaseBannerIconId", "a string")
		return
	}
	if body.HomebaseBannerColorId == "" {
		ValidationError(c, "homebaseBannerColorId", "a string")
		return
	}

	bannerProfileId := "common_core"
	if memory.Build < 3.5 {
		bannerProfileId = "profile0"
	}

	bannerProfileRaw, ok := userProfiles.Profiles[bannerProfileId]
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Profile %s not found in profiles map", bannerProfileId),
			[]string{bannerProfileId}, 12813, "Forbidden", 403)
		return
	}
	bannerProfile := ConvertMapToProfile(bannerProfileRaw)
	if bannerProfile == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Invalid banner profile data structure",
			nil, 12814, "Internal Server Error", 500)
		return
	}

	if bannerProfile["items"] == nil {
		bannerProfile["items"] = map[string]interface{}{}
	}
	itemsMap := bannerProfile["items"].(map[string]interface{})

	HomebaseBannerIconID := ""
	HomebaseBannerColorID := ""

	iconKey := strings.ToLower("HomebaseBannerIcon:" + body.HomebaseBannerIconId)
	colorKey := strings.ToLower("HomebaseBannerColor:" + body.HomebaseBannerColorId)

	for itemId, itemRaw := range itemsMap {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		templateId := strings.ToLower(toString(item["templateId"]))
		if templateId == iconKey {
			HomebaseBannerIconID = itemId
		}
		if templateId == colorKey {
			HomebaseBannerColorID = itemId
		}
		if HomebaseBannerIconID != "" && HomebaseBannerColorID != "" {
			break
		}
	}

	if HomebaseBannerIconID == "" {
		utils.CreateError(c,
			"errors.com.epicgames.fortnite.item_not_found",
			fmt.Sprintf("Banner template 'HomebaseBannerIcon:%s' not found in profile", body.HomebaseBannerIconId),
			[]string{"HomebaseBannerIcon:" + body.HomebaseBannerIconId}, 16006, "Bad Request", 400)
		return
	}
	if HomebaseBannerColorID == "" {
		utils.CreateError(c,
			"errors.com.epicgames.fortnite.item_not_found",
			fmt.Sprintf("Banner template 'HomebaseBannerColor:%s' not found in profile", body.HomebaseBannerColorId),
			[]string{"HomebaseBannerColor:" + body.HomebaseBannerColorId}, 16006, "Bad Request", 400)
		return
	}

	if profile["items"] == nil {
		profile["items"] = map[string]interface{}{}
	}
	profileItems := profile["items"].(map[string]interface{})

	stats := ensureMap(profile, "stats")
	statsAttributes := ensureMap(stats, "attributes")

	var loadouts []interface{}
	switch v := statsAttributes["loadouts"].(type) {
	case []interface{}:
		loadouts = v
	case primitive.A:
		loadouts = []interface{}(v)
	default:
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Active loadouts missing or invalid",
			nil, 12814, "Internal Server Error", 500)
		return
	}

	if len(loadouts) == 0 {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Active loadouts missing or invalid",
			nil, 12814, "Internal Server Error", 500)
		return
	}

	activeLoadoutIndex := getIntFromMap(statsAttributes, "active_loadout_index")
	if activeLoadoutIndex < 0 || activeLoadoutIndex >= len(loadouts) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Active loadout index out of range",
			nil, 12814, "Internal Server Error", 500)
		return
	}

	activeLoadoutId := toString(loadouts[activeLoadoutIndex])
	activeLoadout, ok := profileItems[activeLoadoutId].(map[string]interface{})
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_data",
			"Active loadout item not found",
			nil, 12814, "Internal Server Error", 500)
		return
	}
	attributes := ensureMap(activeLoadout, "attributes")

	statsAttributes["banner_icon"] = body.HomebaseBannerIconId
	statsAttributes["banner_color"] = body.HomebaseBannerColorId

	attributes["banner_icon_template"] = body.HomebaseBannerIconId
	attributes["banner_color_template"] = body.HomebaseBannerColorId

	ApplyProfileChanges := []map[string]interface{}{
		{
			"changeType": "statModified",
			"name":       "banner_icon",
			"value":      statsAttributes["banner_icon"],
		},
		{
			"changeType": "statModified",
			"name":       "banner_color",
			"value":      statsAttributes["banner_color"],
		},
	}

	profile["rvn"] = getIntFromMap(profile, "rvn") + 1
	profile["commandRevision"] = getIntFromMap(profile, "commandRevision") + 1
	profile["updated"] = time.Now().UTC().Format(time.RFC3339)

	filter := bson.M{"accountId": accountId}
	update := bson.M{"$set": bson.M{fmt.Sprintf("profiles.%s", profileId): profile}}

	if _, err := utils.ProfileCollection.UpdateOne(context.TODO(), filter, update); err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.internal.server_error",
			"Failed to update profile",
			nil, 50000, "Internal Server Error", 500)
		return
	}

	ProfileRevisionCheck := getIntFromMap(profile, "commandRevision")
	if memory.Build < 12.20 {
		ProfileRevisionCheck = getIntFromMap(profile, "rvn")
	}

	QueryRevisionInt := -1
	if rvnQuery != "" {
		if val, err := strconv.Atoi(rvnQuery); err == nil {
			QueryRevisionInt = val
		}
	}

	if QueryRevisionInt != ProfileRevisionCheck {
		ApplyProfileChanges = []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":            profile["rvn"],
		"profileId":                  profileId,
		"profileChangesBaseRevision": getIntFromMap(profile, "rvn") - 1,
		"profileChanges":             ApplyProfileChanges,
		"profileCommandRevision":     profile["commandRevision"],
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"responseVersion":            1,
	})
}

func EquipBattleRoyaleCustomization(c *gin.Context) {
	accountId := c.Param("accountId")
	profileId := c.Query("profileId")
	rvnQuery := c.Query("rvn")

	userProfiles, err := utils.FindProfileByAccountID(accountId)
	if err != nil || userProfiles == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.not_found",
			"Profile not found",
			[]string{accountId},
			12804,
			"Not Found",
			404)
		return
	}

	if !profiles.ValidateProfile(profileId, userProfiles) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileId),
			[]string{profileId},
			12813,
			"Forbidden",
			403)
		return
	}

	if profileId != "athena" {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.invalid_command",
			fmt.Sprintf("EquipBattleRoyaleCustomization is not valid on %s profile", profileId),
			[]string{"EquipBattleRoyaleCustomization", profileId},
			12801,
			"Bad Request",
			400)
		return
	}

	profileDataRaw, ok := userProfiles.Profiles[profileId]
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.operation_forbidden",
			fmt.Sprintf("Profile %s invalid format", profileId),
			[]string{profileId},
			12813,
			"Forbidden",
			403)
		return
	}

	profile := ConvertMapToProfile(profileDataRaw)
	memory := utils.GetVersionInfo(c.Request)

	stats := ensureMap(profile, "stats")
	attributes := ensureMap(stats, "attributes")

	if profileId == "athena" {
		attributes["season_num"] = memory.Season
	}

	var body struct {
		ItemToSlot      string                   `json:"itemToSlot"`
		SlotName        string                   `json:"slotName"`
		VariantUpdates  []map[string]interface{} `json:"variantUpdates"`
		IndexWithinSlot *int                     `json:"indexWithinSlot"`
	}
	if err := c.BindJSON(&body); err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.invalid_payload",
			"Invalid request body",
			nil,
			12800,
			"Bad Request",
			400)
		return
	}

	missingFields := checkFields([]string{"slotName"}, map[string]interface{}{
		"slotName": body.SlotName,
	})
	if len(missingFields) > 0 {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			fmt.Sprintf("Validation Failed. [%s] field(s) is missing.", strings.Join(missingFields, ", ")),
			missingFields,
			1040,
			"Bad Request",
			400)
		return
	}

	if body.SlotName == "" {
		utils.CreateError(c,
			"errors.com.epicgames.validation.validation_failed",
			"itemToSlot and slotName must be non-empty strings",
			[]string{"itemToSlot", "slotName"},
			1040,
			"Bad Request",
			400)
		return
	}

	items := ensureMap(profile, "items")

	specialCosmetics := []string{
		"AthenaCharacter:cid_random",
		"AthenaBackpack:bid_random",
		"AthenaPickaxe:pickaxe_random",
		"AthenaGlider:glider_random",
		"AthenaSkyDiveContrail:trails_random",
		"AthenaItemWrap:wrap_random",
		"AthenaMusicPack:musicpack_random",
		"AthenaLoadingScreen:lsid_random",
	}

	if _, exists := items[body.ItemToSlot]; !exists && body.ItemToSlot != "" {
		item := body.ItemToSlot
		if !contains(specialCosmetics, item) {
			utils.CreateError(c,
				"errors.com.epicgames.fortnite.id_invalid",
				fmt.Sprintf("Item (id: '%s') not found", item),
				[]string{item},
				16027,
				"Bad Request",
				400)
			return
		} else if !strings.HasPrefix(item, "Athena"+body.SlotName+":") {
			utils.CreateError(c,
				"errors.com.epicgames.fortnite.id_invalid",
				fmt.Sprintf("Cannot slot item of type %s in slot of category %s", strings.Split(item, ":")[0], body.SlotName),
				[]string{strings.Split(item, ":")[0], body.SlotName},
				16027,
				"Bad Request",
				400)
			return
		}
	}

	var applyProfileChanges []map[string]interface{}

	if itemDataRaw, exists := items[body.ItemToSlot]; exists {
		itemData, ok := itemDataRaw.(map[string]interface{})
		if !ok {
			itemData = make(map[string]interface{})
		}

		templateId, _ := itemData["templateId"].(string)
		if !strings.HasPrefix(templateId, "Athena"+body.SlotName+":") {
			utils.CreateError(c,
				"errors.com.epicgames.fortnite.id_invalid",
				fmt.Sprintf("Cannot slot item of type %s in slot of category %s", strings.Split(templateId, ":")[0], body.SlotName),
				[]string{strings.Split(templateId, ":")[0], body.SlotName},
				16027,
				"Bad Request",
				400)
			return
		}

		for _, v := range body.VariantUpdates {
			channel, ok1 := v["channel"].(string)
			active, ok2 := v["active"].(string)
			if !ok1 || !ok2 {
				continue
			}

			attributesMap, _ := itemData["attributes"].(map[string]interface{})

			variantsRaw, ok := attributesMap["variants"].([]interface{})
			if !ok {
				variantsRaw = []interface{}{}
			}

			found := false
			for i, variantRaw := range variantsRaw {
				variant, ok := variantRaw.(map[string]interface{})
				if !ok {
					continue
				}

				if ch, ok := variant["channel"].(string); ok && ch == channel {
					variantsRaw[i].(map[string]interface{})["active"] = active
					found = true
					break
				}
			}

			if !found {
				newVariant := map[string]interface{}{
					"channel": channel,
					"active":  active,
					"owned":   []interface{}{active},
				}
				variantsRaw = append(variantsRaw, newVariant)
			}

			attributesMap["variants"] = variantsRaw
		}

		if len(body.VariantUpdates) > 0 {
			applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
				"changeType":     "itemAttrChanged",
				"itemId":         body.ItemToSlot,
				"attributeName":  "variants",
				"attributeValue": itemData["attributes"].(map[string]interface{})["variants"],
			})
		}
	}

	slotNames := []string{"Character", "Backpack", "Pickaxe", "Glider", "SkyDiveContrail", "MusicPack", "LoadingScreen"}
	activeLoadoutIndex := getIntFromMap(attributes, "active_loadout_index")
	var loadoutsRaw []interface{}

	raw, ok := attributes["loadouts"]
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Missing loadouts",
			nil,
			12813,
			"Forbidden",
			403)
		return
	}

	switch v := raw.(type) {
	case primitive.A:
		loadoutsRaw = []interface{}(v)
	case []interface{}:
		loadoutsRaw = v
	case []string:
		for _, s := range v {
			loadoutsRaw = append(loadoutsRaw, s)
		}
	default:
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Invalid loadouts format, got %T", raw),
			nil,
			12813,
			"Forbidden",
			403)
		return
	}

	for _, item := range loadoutsRaw {
		_, ok := item.(string)
		if !ok {
			utils.CreateError(c,
				"errors.com.epicgames.modules.profiles.operation_forbidden",
				"Invalid loadout structure",
				nil,
				12813,
				"Forbidden",
				403)
			return
		}
	}

	if activeLoadoutIndex < 0 || activeLoadoutIndex >= len(loadoutsRaw) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Invalid active loadout index",
			nil,
			12813,
			"Forbidden",
			403)
		return
	}

	activeLoadoutId, ok := loadoutsRaw[activeLoadoutIndex].(string)
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Invalid active loadout ID",
			nil,
			12813,
			"Forbidden",
			403)
		return
	}

	activeLoadoutItemRaw, ok := items[activeLoadoutId]
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Active loadout item not found",
			nil,
			12813,
			"Forbidden",
			403)
		return
	}

	activeLoadoutItem, ok := activeLoadoutItemRaw.(map[string]interface{})
	if !ok {
		activeLoadoutItem = make(map[string]interface{})
	}

	attributesMap, ok := activeLoadoutItem["attributes"].(map[string]interface{})
	if !ok {
		attributesMap = make(map[string]interface{})
		activeLoadoutItem["attributes"] = attributesMap
	}

	lockerSlotsData, ok := attributesMap["locker_slots_data"].(map[string]interface{})
	if !ok {
		lockerSlotsData = make(map[string]interface{})
		attributesMap["locker_slots_data"] = lockerSlotsData
	}

	slotsMap, ok := lockerSlotsData["slots"].(map[string]interface{})
	if !ok {
		slotsMap = make(map[string]interface{})
		lockerSlotsData["slots"] = slotsMap
	}

	var favDanceRaw []interface{}
	switch v := attributes["favorite_dance"].(type) {
	case []interface{}:
		favDanceRaw = v
	case primitive.A:
		favDanceRaw = []interface{}(v)
	case nil:
		favDanceRaw = make([]interface{}, 6)
	default:
		utils.Backend.Logf("Unexpected type for favorite_dance: %T", v)
		favDanceRaw = make([]interface{}, 6)
	}
	if len(favDanceRaw) < 6 {
		tmp := make([]interface{}, 6)
		copy(tmp, favDanceRaw)
		favDanceRaw = tmp
	}
	attributes["favorite_dance"] = favDanceRaw

	var favWrapRaw []interface{}
	switch v := attributes["favorite_itemwraps"].(type) {
	case []interface{}:
		favWrapRaw = v
	case primitive.A:
		favWrapRaw = []interface{}(v)
	case nil:
		favWrapRaw = make([]interface{}, 8)
	default:
		utils.Backend.Logf("Unexpected type for favorite_itemwraps: %T", v)
		favWrapRaw = make([]interface{}, 8)
	}
	if len(favWrapRaw) < 8 {
		tmp := make([]interface{}, 8)
		copy(tmp, favWrapRaw)
		favWrapRaw = tmp
	}
	attributes["favorite_itemwraps"] = favWrapRaw

	baseRevision := getIntFromMap(profile, "rvn")
	profileRevisionCheck := baseRevision
	if memory.Build >= 12.20 {
		profileRevisionCheck = getIntFromMap(profile, "commandRevision")
	}

	queryRevision, err := strconv.Atoi(rvnQuery)
	if err != nil || rvnQuery == "" {
		queryRevision = -1
	}

	switch body.SlotName {
	case "Dance":
		danceSlotRaw, ok := slotsMap["Dance"].(map[string]interface{})
		if !ok {
			break
		}
		if body.IndexWithinSlot == nil {
			utils.CreateError(c,
				"errors.com.epicgames.validation.validation_failed",
				"indexWithinSlot is required for Dance slot",
				[]string{"indexWithinSlot"},
				1040,
				"Bad Request",
				400)
			return
		}
		idx := *body.IndexWithinSlot
		if idx < 0 || idx >= 6 {
			utils.CreateError(c,
				"errors.com.epicgames.validation.validation_failed",
				"indexWithinSlot out of range for Dance slot",
				[]string{"indexWithinSlot"},
				1040,
				"Bad Request",
				400)
			return
		}

		if len(favDanceRaw) < 6 {
			tmp := make([]interface{}, 6)
			copy(tmp, favDanceRaw)
			favDanceRaw = tmp
		}
		favDanceRaw[idx] = body.ItemToSlot
		attributes["favorite_dance"] = favDanceRaw

		itemsInterface, _ := danceSlotRaw["items"]
		var items primitive.A
		switch v := itemsInterface.(type) {
		case primitive.A:
			items = v
		case []interface{}:
			items = primitive.A(v)
		default:
			utils.Backend.Logf("Unexpected type for Dance slot items: %T", v)
			items = make(primitive.A, 6)
		}
		if len(items) < 6 {
			tmp := make(primitive.A, 6)
			copy(tmp, items)
			items = tmp
		}

		var itemTemplateId string
		if itemMap, ok := profile["items"].(map[string]interface{})[body.ItemToSlot].(map[string]interface{}); ok {
			if tplId, ok := itemMap["templateId"].(string); ok {
				itemTemplateId = tplId
			}
		}
		items[idx] = itemTemplateId
		danceSlotRaw["items"] = items

		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       "favorite_dance",
			"value":      favDanceRaw,
		})

	case "ItemWrap":
		itemWrapSlotRaw, ok := slotsMap["ItemWrap"].(map[string]interface{})
		if !ok {
			break
		}
		if body.IndexWithinSlot == nil {
			utils.CreateError(c,
				"errors.com.epicgames.validation.validation_failed",
				"indexWithinSlot is required for ItemWrap slot",
				[]string{"indexWithinSlot"},
				1040,
				"Bad Request",
				400)
			return
		}
		idx := *body.IndexWithinSlot

		if len(favWrapRaw) < 8 {
			tmp := make([]interface{}, 8)
			copy(tmp, favWrapRaw)
			favWrapRaw = tmp
		}

		itemsInterface, _ := itemWrapSlotRaw["items"]
		var items primitive.A
		switch v := itemsInterface.(type) {
		case primitive.A:
			items = v
		case []interface{}:
			items = primitive.A(v)
		default:
			utils.Backend.Logf("Unexpected type for ItemWrap slot items: %T", v)
			items = make(primitive.A, 8)
		}
		if len(items) < 8 {
			tmp := make(primitive.A, 8)
			copy(tmp, items)
			items = tmp
		}

		var itemTemplateId string
		if itemMap, ok := profile["items"].(map[string]interface{})[body.ItemToSlot].(map[string]interface{}); ok {
			if tplId, ok := itemMap["templateId"].(string); ok {
				itemTemplateId = tplId
			}
		}

		if idx == -1 {
			for i := 0; i < 8; i++ {
				favWrapRaw[i] = body.ItemToSlot
				items[i] = itemTemplateId
			}
		} else {
			if idx < 0 || idx >= 8 {
				utils.CreateError(c,
					"errors.com.epicgames.validation.validation_failed",
					"indexWithinSlot out of range for ItemWrap slot",
					[]string{"indexWithinSlot"},
					1040,
					"Bad Request",
					400)
				return
			}
			favWrapRaw[idx] = body.ItemToSlot
			items[idx] = itemTemplateId
		}

		attributes["favorite_itemwraps"] = favWrapRaw
		itemWrapSlotRaw["items"] = items

		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       "favorite_itemwraps",
			"value":      favWrapRaw,
		})

	default:
		if !contains(slotNames, body.SlotName) {
			break
		}
		if _, ok := slotsMap[body.SlotName]; !ok {
			slotsMap[body.SlotName] = map[string]interface{}{
				"items": []interface{}{},
			}
		}

		if body.SlotName == "Pickaxe" || body.SlotName == "Glider" {
			if body.ItemToSlot == "" {
				utils.CreateError(c,
					"errors.com.epicgames.fortnite.id_invalid",
					fmt.Sprintf("%s can not be empty.", body.SlotName),
					[]string{body.SlotName},
					16027,
					"Bad Request",
					400)
				return
			}
		}

		favKey := strings.ToLower("favorite_" + body.SlotName)
		if body.ItemToSlot == "" {
			attributes[favKey] = ""
			slotsMap[body.SlotName].(map[string]interface{})["items"] = []interface{}{}
		} else {
			slotData := slotsMap[body.SlotName].(map[string]interface{})
			itemsInterface := slotData["items"]
			var itemsArr []interface{}
			switch v := itemsInterface.(type) {
			case primitive.A:
				itemsArr = []interface{}(v)
			case []interface{}:
				itemsArr = v
			default:
				utils.Backend.Logf("Unexpected type for slot %s items: %T", body.SlotName, v)
				itemsArr = []interface{}{}
			}

			found := false
			for _, item := range itemsArr {
				if itemStr, ok := item.(string); ok && itemStr == body.ItemToSlot {
					found = true
					break
				}
			}

			if !found {
				itemsArr = append(itemsArr, body.ItemToSlot)
			}

			slotData["items"] = itemsArr
			attributes[favKey] = body.ItemToSlot
		}

		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       favKey,
			"value":      attributes[favKey],
		})
	}

	if len(applyProfileChanges) > 0 {
		profile["rvn"] = getIntFromMap(profile, "rvn") + 1
		profile["commandRevision"] = getIntFromMap(profile, "commandRevision") + 1
		profile["updated"] = time.Now().UTC().Format(time.RFC3339)

		filter := bson.M{"accountId": accountId}
		update := bson.M{"$set": bson.M{fmt.Sprintf("profiles.%s", profileId): profile}}

		_, err := utils.ProfileCollection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.modules.profiles.operation_forbidden",
				"Failed to update profile",
				nil,
				12813,
				"Forbidden",
				500)
			return
		}
	}

	if queryRevision != profileRevisionCheck {
		applyProfileChanges = []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profile,
			},
		}
	}

	c.JSON(200, gin.H{
		"profileRevision":            getIntFromMap(profile, "rvn"),
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     getIntFromMap(profile, "commandRevision"),
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"responseVersion":            1,
	})
}

func MCPHandler(c *gin.Context) {
	accountId := c.Param("accountId")
	operation := c.Param("operation")
	profileId := c.Query("profileId")
	rvnQuery := c.DefaultQuery("rvn", "-1")

	userProfiles, err := utils.FindProfileByAccountID(accountId)
	if err != nil || userProfiles == nil {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileId),
			[]string{profileId}, 12813, "Forbidden", 403)
		return
	}

	if !profiles.ValidateProfile(profileId, userProfiles) {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileId),
			[]string{profileId}, 12813, "Forbidden", 403)
		return
	}

	profileRaw, ok := userProfiles.Profiles[profileId]
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.operation_forbidden",
			fmt.Sprintf("Profile %s not found", profileId),
			[]string{profileId}, 12813, "Forbidden", 403)
		return
	}

	profile, ok := profileRaw.(map[string]interface{})
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.modules.userProfiles.operation_forbidden",
			fmt.Sprintf("Profile %s invalid format", profileId),
			[]string{profileId}, 12813, "Forbidden", 403)
		return
	}

	rvn := getIntFromMap(profile, "rvn")
	commandRevision := getIntFromMap(profile, "commandRevision")

	changed := false

	if profileId == "athena" {
		if statsRaw, ok := profile["stats"].(map[string]interface{}); ok {
			if attrsRaw, ok := statsRaw["attributes"].(map[string]interface{}); ok {
				lastAppliedLoadout, _ := attrsRaw["last_applied_loadout"].(string)
				loadoutsRaw, _ := attrsRaw["loadouts"].([]interface{})
				if lastAppliedLoadout == "" && len(loadoutsRaw) > 0 {
					if firstLoadout, ok := loadoutsRaw[0].(string); ok {
						attrsRaw["last_applied_loadout"] = firstLoadout
						changed = true
					}
				}
			}
		}
	}

	if changed {
		rvn++
		commandRevision++
		profile["rvn"] = rvn
		profile["commandRevision"] = commandRevision
		profile["updated"] = time.Now().UTC().Format(time.RFC3339)

		userProfiles.Profiles[profileId] = profile

		updateField := fmt.Sprintf("profiles.%s", profileId)
		_, _ = ProfileCollection.UpdateOne(
			c.Request.Context(),
			bson.M{"accountId": accountId},
			bson.M{"$set": bson.M{updateField: profile}},
		)
	}

	memory := utils.GetVersionInfo(c.Request)
	if profileId == "athena" {
		if statsRaw, ok := profile["stats"].(map[string]interface{}); ok {
			if attrsRaw, ok := statsRaw["attributes"].(map[string]interface{}); ok {
				attrsRaw["season_num"] = memory.Season
			}
		}
	}

	baseRevision := rvn
	profileRevisionCheck := rvn
	if memory.Build >= 12.20 {
		profileRevisionCheck = commandRevision
	}

	qRev, _ := strconv.Atoi(rvnQuery)

	var applyProfileChanges []gin.H
	if qRev != profileRevisionCheck {
		applyProfileChanges = []gin.H{{
			"changeType": "fullProfileUpdate",
			"profile":    profile,
		}}
	}

	switch operation {
	case "QueryProfile", "ClientQuestLogin", "RefreshExpeditions", "GetMcpTimeForLogin",
		"IncrementNamedCounterStat", "SetHardcoreModifier", "SetMtxPlatform", "BulkEquipBattleRoyaleCustomization":
	default:
		utils.CreateError(c,
			"errors.com.epicgames.fortnite.operation_not_found",
			fmt.Sprintf("Operation %s not valid", operation),
			[]string{operation}, 16035, "NotFound", 404)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":            rvn,
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     commandRevision,
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"multiUpdate":                []gin.H{},
		"responseVersion":            1,
	})
}

func DedicatedServerHandler(c *gin.Context) {
	accountID := c.Param("accountId")
	profileID := c.Query("profileId")
	queryRVN := c.Query("rvn")

	var profile models.Profiles
	err := utils.ProfileCollection.FindOne(c, bson.M{"accountId": accountID}).Decode(&profile)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	if !profiles.ValidateProfile(profileID, &profile) {
		utils.CreateError(
			c,
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			fmt.Sprintf("Unable to find template configuration for profile %s", profileID),
			[]string{profileID},
			12813,
			"Forbidden",
			403,
		)
		return
	}

	rawProfileData, ok := profile.Profiles[profileID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
		return
	}

	convertedProfile := ConvertMapToProfile(rawProfileData)
	baseRevision := convertedProfile["rvn"]
	commandRevision := convertedProfile["commandRevision"]

	applyProfileChanges := []interface{}{}
	if queryRVN != "" && queryRVN != fmt.Sprint(baseRevision) {
		applyProfileChanges = append(applyProfileChanges, gin.H{
			"changeType": "fullProfileUpdate",
			"profile":    convertedProfile,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"profileRevision":            baseRevision,
		"profileId":                  profileID,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     commandRevision,
		"serverTime":                 time.Now().Format(time.RFC3339),
		"responseVersion":            1,
	})
}

func ConvertMapToProfile(data interface{}) map[string]interface{} {
	profileMap, ok := data.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return profileMap
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			i, err := strconv.Atoi(v)
			if err == nil {
				return i
			}
		}
	}
	return 0
}

func checkFields(required []string, body map[string]interface{}) []string {
	var missing []string
	for _, field := range required {
		if _, ok := body[field]; !ok {
			missing = append(missing, field)
		}
	}
	return missing
}

func ensureMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key]; ok {
		if casted, ok := val.(map[string]interface{}); ok {
			return casted
		}
	}
	newMap := make(map[string]interface{})
	m[key] = newMap
	return newMap
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func toString(data interface{}) string {
	str, ok := data.(string)
	if !ok {
		return ""
	}
	return str
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromInterfaceSafe(val interface{}, fallback int) int {
	switch v := val.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if num, err := strconv.Atoi(v); err == nil {
			return num
		}
	default:
		return fallback
	}
	return fallback
}

func CheckFields(required []string, body map[string]interface{}) []string {
	missing := []string{}
	for _, field := range required {
		if val, exists := body[field]; !exists || val == nil {
			missing = append(missing, field)
		}
	}
	return missing
}

func ValidationError(c *gin.Context, field, typ string) {
	utils.CreateError(
		c,
		"errors.com.epicgames.validation.validation_failed",
		fmt.Sprintf("Validation Failed. '%s' is not %s.", field, typ),
		[]string{field},
		1040,
		"Bad Request",
		400,
	)
}

func ensureSliceLength(s []interface{}, length int) []interface{} {
	if s == nil {
		s = make([]interface{}, length)
		for i := 0; i < length; i++ {
			s[i] = ""
		}
		return s
	}
	if len(s) >= length {
		return s
	}
	for len(s) < length {
		s = append(s, "")
	}
	return s
}

func toInterfaceSlice(v interface{}) []interface{} {
	switch s := v.(type) {
	case nil:
		return []interface{}{}
	case []interface{}:
		return s
	case primitive.A:
		return []interface{}(s)
	default:
		return []interface{}{}
	}
}

func checkIfDuplicateExists(arr []string) bool {
	seen := make(map[string]bool)
	for _, v := range arr {
		if seen[v] {
			return true
		}
		seen[v] = true
	}
	return false
}
