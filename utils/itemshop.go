package utils

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CatalogEntry struct {
	DevName              string                   `json:"devName"`
	OfferID              string                   `json:"offerId"`
	FulfillmentIds       []string                 `json:"fulfillmentIds"`
	DailyLimit           int                      `json:"dailyLimit"`
	WeeklyLimit          int                      `json:"weeklyLimit"`
	MonthlyLimit         int                      `json:"monthlyLimit"`
	Categories           []string                 `json:"categories"`
	Prices               []map[string]interface{} `json:"prices"`
	Meta                 map[string]interface{}   `json:"meta"`
	MatchFilter          string                   `json:"matchFilter"`
	FilterWeight         int                      `json:"filterWeight"`
	AppStoreId           []string                 `json:"appStoreId"`
	Requirements         []map[string]interface{} `json:"requirements"`
	OfferType            string                   `json:"offerType"`
	GiftInfo             map[string]interface{}   `json:"giftInfo"`
	Refundable           bool                     `json:"refundable"`
	MetaInfo             []map[string]string      `json:"metaInfo"`
	DisplayAssetPath     string                   `json:"displayAssetPath"`
	ItemGrants           []map[string]interface{} `json:"itemGrants"`
	SortPriority         int                      `json:"sortPriority"`
	CatalogGroupPriority int                      `json:"catalogGroupPriority"`
}

func GetItemShop() map[string]interface{} {
	baseDir := "."
	catalogPath := filepath.Join(baseDir, "static", "responses", "catalog.json")
	configPath := filepath.Join(baseDir, "static", "responses", "catalog_config.json")

	catalogBytes, _ := os.ReadFile(catalogPath)
	configBytes, _ := os.ReadFile(configPath)

	var catalog map[string]interface{}
	var config map[string]map[string]interface{}
	json.Unmarshal(catalogBytes, &catalog)
	json.Unmarshal(configBytes, &config)

	tomorrow := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	saleExpiry := tomorrow.Add(-time.Minute).Format(time.RFC3339)

	storefronts := catalog["storefronts"].([]interface{})

	var dailyKeys, featuredKeys []string
	for key := range config {
		if strings.HasPrefix(key, "daily") {
			dailyKeys = append(dailyKeys, key)
		} else if strings.HasPrefix(key, "featured") {
			featuredKeys = append(featuredKeys, key)
		}
	}

	dailyAssignments := make(map[string][]string)
	switch len(dailyKeys) {
	case 7:
		dailyAssignments["daily1"] = []string{"daily1"}
		dailyAssignments["daily2"] = []string{"daily1"}
		dailyAssignments["daily3"] = []string{}
		dailyAssignments["daily4"] = []string{}
		dailyAssignments["daily5"] = []string{}
		dailyAssignments["daily6"] = []string{}
		dailyAssignments["daily7"] = []string{}
	case 8:
		dailyAssignments["daily1"] = []string{"daily1"}
		dailyAssignments["daily2"] = []string{"daily1"}
		dailyAssignments["daily3"] = []string{}
		dailyAssignments["daily4"] = []string{}
		dailyAssignments["daily5"] = []string{}
		dailyAssignments["daily6"] = []string{}
		dailyAssignments["daily7"] = []string{"daily2"}
		dailyAssignments["daily8"] = []string{"daily2"}
	default:
		for _, key := range dailyKeys {
			dailyAssignments[key] = []string{}
		}
	}

	featuredAssignments := make(map[string][]string)
	switch len(featuredKeys) {
	case 4:
		featuredAssignments["featured1"] = []string{}
		featuredAssignments["featured2"] = []string{}
		featuredAssignments["featured3"] = []string{"Featured1"}
		featuredAssignments["featured4"] = []string{"Featured1"}
	case 5:
		featuredAssignments["featured1"] = []string{}
		featuredAssignments["featured2"] = []string{"Featured1"}
		featuredAssignments["featured3"] = []string{"Featured1"}
		featuredAssignments["featured4"] = []string{"Featured1"}
		featuredAssignments["featured5"] = []string{}
	case 6:
		featuredAssignments["featured1"] = []string{"Featured1"}
		featuredAssignments["featured2"] = []string{"Featured1"}
		featuredAssignments["featured3"] = []string{"Featured2"}
		featuredAssignments["featured4"] = []string{"Featured2"}
		featuredAssignments["featured5"] = []string{"Featured3"}
		featuredAssignments["featured6"] = []string{"Featured3"}
	default:
		for _, key := range featuredKeys {
			featuredAssignments[key] = []string{}
		}
	}

	for key, value := range config {
		itemGrants, ok := value["itemGrants"].([]interface{})
		if !ok || len(itemGrants) == 0 {
			continue
		}

		var entry CatalogEntry
		entry.DevName = ""
		entry.OfferID = ""
		entry.FulfillmentIds = []string{}
		entry.DailyLimit = -1
		entry.WeeklyLimit = -1
		entry.MonthlyLimit = -1
		entry.Categories = []string{}
		entry.Prices = []map[string]interface{}{
			{
				"currencyType":    "MtxCurrency",
				"currencySubType": "",
				"regularPrice":    value["price"],
				"finalPrice":      value["price"],
				"saleExpiration":  saleExpiry,
				"basePrice":       value["price"],
			},
		}
		entry.Meta = map[string]interface{}{
			"SectionId": "Featured",
			"TileSize":  "Small",
		}
		entry.MatchFilter = ""
		entry.FilterWeight = 0
		entry.AppStoreId = []string{}
		entry.Requirements = []map[string]interface{}{}
		entry.OfferType = "StaticPrice"
		entry.GiftInfo = map[string]interface{}{
			"bIsEnabled":              true,
			"forcedGiftBoxTemplateId": "",
			"purchaseRequirements":    []interface{}{},
			"giftRecordIds":           []interface{}{},
		}
		entry.Refundable = true
		entry.MetaInfo = []map[string]string{
			{"key": "SectionId", "value": "Featured"},
			{"key": "TileSize", "value": "Small"},
		}
		entry.DisplayAssetPath = ""
		entry.ItemGrants = []map[string]interface{}{}
		entry.SortPriority = 0
		entry.CatalogGroupPriority = 0

		storeName := "BRWeeklyStorefront"
		if strings.HasPrefix(key, "daily") {
			storeName = "BRDailyStorefront"
			entry.SortPriority = -1
			entry.Categories = dailyAssignments[key]
		} else {
			entry.Meta["TileSize"] = "Normal"
			entry.MetaInfo[1]["value"] = "Normal"
			entry.Categories = featuredAssignments[key]
		}

		for _, grant := range itemGrants {
			id, ok := grant.(string)
			if !ok || id == "" {
				continue
			}
			entry.Requirements = append(entry.Requirements, map[string]interface{}{
				"requirementType": "DenyOnItemOwnership",
				"requiredId":      id,
				"minQuantity":     1,
			})
			entry.ItemGrants = append(entry.ItemGrants, map[string]interface{}{
				"templateId": id,
				"quantity":   1,
			})
		}

		if len(entry.ItemGrants) > 0 {
			keyHash := sha1.Sum([]byte(fmt.Sprintf("%v_%v", itemGrants, value["price"])))
			uid := fmt.Sprintf("%x", keyHash)
			entry.DevName = uid
			entry.OfferID = uid

			for i, sf := range storefronts {
				sfo := sf.(map[string]interface{})
				if sfo["name"] == storeName {
					sfo["catalogEntries"] = append(sfo["catalogEntries"].([]interface{}), entry)
					storefronts[i] = sfo
					break
				}
			}
		}
	}

	return catalog
}

func GetOfferID(offerId string) (string, *CatalogEntry) {
	catalog := GetItemShop()
	storefronts, ok := catalog["storefronts"].([]interface{})
	if !ok {
		return "", nil
	}

	for _, sf := range storefronts {
		store, ok := sf.(map[string]interface{})
		if !ok {
			continue
		}

		entriesRaw, ok := store["catalogEntries"].([]interface{})
		if !ok {
			continue
		}

		for _, e := range entriesRaw {
			switch entry := e.(type) {
			case CatalogEntry:
				if entry.OfferID == offerId {
					name, _ := store["name"].(string)
					return name, &entry
				}
			case *CatalogEntry:
				if entry.OfferID == offerId {
					name, _ := store["name"].(string)
					return name, entry
				}
			case map[string]interface{}:
				entryJSON, err := json.Marshal(entry)
				if err != nil {
					continue
				}
				var ce CatalogEntry
				if err := json.Unmarshal(entryJSON, &ce); err != nil {
					continue
				}
				if ce.OfferID == offerId {
					name, _ := store["name"].(string)
					return name, &ce
				}
			default:
				continue
			}
		}
	}
	return "", nil
}
