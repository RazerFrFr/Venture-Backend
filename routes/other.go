package routes

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var EnableGlobalChat = os.Getenv("ENABLE_GLOBAL_CHAT") == "true"

func RegisterMiscRoutes(r *gin.Engine) {
	r.POST("/fortnite/api/game/v2/chat/*path", chatRouterHandler)

	r.GET("/launcher/api/public/distributionpoints/", distributionPointsHandler)

	r.GET("/launcher/api/public/assets/*asset", launcherAssetsHandler)

	r.GET("/Builds/Fortnite/Content/CloudDir/*filename", func(c *gin.Context) {
		filename := filepath.Base(c.Param("filename"))

		var filePath string
		switch {
		case strings.HasSuffix(filename, ".manifest"):
			filePath = filepath.Join(".", "static", "responses", "CloudDir", "VentureBackend.manifest")
		case strings.HasSuffix(filename, ".chunk"):
			filePath = filepath.Join(".", "static", "responses", "CloudDir", "VentureBackend.chunk")
		case strings.HasSuffix(filename, ".ini"):
			filePath = filepath.Join(".", "static", "responses", "CloudDir", "Full.ini")
		default:
			c.Status(404)
			return
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			c.Status(404)
			return
		}

		c.Data(200, "application/octet-stream", data)
	})

	r.POST("/fortnite/api/game/v2/grant_access/:any", grantAccessHandler)
	r.POST("/api/v1/user/setting", userSettingHandler)
	r.GET("/waitingroom/api/waitingroom", waitingRoomHandler)
	r.GET("/socialban/api/public/v1/*any", socialBanHandler)
	r.GET("/fortnite/api/game/v2/events/tournamentandhistory/:any/EU/WindowsClient", tournamentHandler)
	r.GET("/fortnite/api/statsv2/account/:accountId", statsHandler)
	r.GET("/statsproxy/api/statsv2/account/:accountId", statsHandler)
	r.GET("/fortnite/api/stats/accountId/:accountId/bulk/window/alltime", statsHandler)
	r.POST("/fortnite/api/feedback/:any", feedbackHandler)
	r.POST("/fortnite/api/statsv2/query", statsQueryHandler)
	r.POST("/statsproxy/api/statsv2/query", statsQueryHandler)
	r.POST("/fortnite/api/game/v2/events/v2/setSubgroup/:any", setSubgroupHandler)
	r.GET("/fortnite/api/game/v2/enabled_features", enabledFeaturesHandler)
	r.GET("/api/v1/events/Fortnite/download/:accountId", eventsDownloadHandler)
	r.GET("/fortnite/api/game/v2/twitch/:any", twitchHandler)
	r.GET("/fortnite/api/game/v2/world/info", worldInfoHandler)
	r.GET("/presence/api/v1/_/:any/last-online", lastOnlineHandler)
	r.GET("/fortnite/api/receipts/v1/account/:accountId/receipts", receiptsHandler)
	r.GET("/fortnite/api/game/v2/leaderboards/cohort/:any", leaderboardsHandler)
	r.POST("/api/v1/assets/Fortnite/:param1/:param2", assetsFortniteHandler)
	r.GET("/region", regionHandler)
	r.GET("/fortnite/api/game/v2/br-inventory/account/:accountId", brInventoryHandler)
	r.POST("/datarouter/api/v1/public/data", datarouterDataHandler)
}

func chatRouterHandler(c *gin.Context) {
	path := c.Param("path")
	switch {
	case strings.HasSuffix(path, "/recommendGeneralChatRooms/pc"):
		c.JSON(http.StatusOK, gin.H{})
	case strings.HasSuffix(path, "/pc") || strings.HasSuffix(path, "/mobile"):
		resp := gin.H{}
		if EnableGlobalChat {
			resp = gin.H{"GlobalChatRooms": []gin.H{{"roomName": "rhglobal"}}}
		}
		c.JSON(http.StatusOK, resp)
	default:
		c.Status(http.StatusNotFound)
	}
}

func distributionPointsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"distributions": []string{
			"https://download.epicgames.com/",
			"https://download2.epicgames.com/",
			"https://download3.epicgames.com/",
			"https://download4.epicgames.com/",
			"https://epicgames-download1.akamaized.net/",
		},
	})
}

func launcherAssetsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"appName":       "FortniteContentBuilds",
		"labelName":     "VentureBackend",
		"buildVersion":  "++Fortnite+Release-9.10-6573057-Android",
		"catalogItemId": "5cb97847cee34581afdbc445400e2f77",
		"expires":       "9999-12-31T23:59:59.999Z",
		"items": gin.H{
			"MANIFEST": gin.H{
				"signature":               "VentureBackend",
				"distribution":            "https://VentureBackend.ol.epicgames.com/",
				"path":                    "Builds/Fortnite/Content/CloudDir/VentureBackend.manifest",
				"hash":                    "55bb954f5596cadbe03693e1c06ca73368d427f3",
				"additionalDistributions": []string{},
			},
			"CHUNKS": gin.H{
				"signature":               "VentureBackend",
				"distribution":            "https://VentureBackend.ol.epicgames.com/",
				"path":                    "Builds/Fortnite/Content/CloudDir/VentureBackend.manifest",
				"additionalDistributions": []string{},
			},
		},
		"assetId": "FortniteContentBuilds",
	})
}

func grantAccessHandler(c *gin.Context) {
	c.JSON(http.StatusNoContent, gin.H{})
}

func userSettingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []interface{}{})
}

func waitingRoomHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func socialBanHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"bans":     []interface{}{},
		"warnings": []interface{}{},
	})
}

func tournamentHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func statsHandler(c *gin.Context) {
	accountId := c.Param("accountId")
	c.JSON(http.StatusOK, gin.H{
		"startTime": 0,
		"endTime":   0,
		"stats":     gin.H{},
		"accountId": accountId,
	})
}

func feedbackHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}

func statsQueryHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []interface{}{})
}

func setSubgroupHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func enabledFeaturesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []interface{}{})
}

func eventsDownloadHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func twitchHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}

func worldInfoHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func lastOnlineHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func receiptsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []interface{}{})
}

func leaderboardsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []interface{}{})
}

func assetsFortniteHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"FortCreativeDiscoverySurface": gin.H{
			"meta":   gin.H{"promotion": 0},
			"assets": gin.H{},
		},
	})
}

func regionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"continent": gin.H{
			"code":       "EU",
			"geoname_id": 6255148,
			"names": gin.H{
				"de": "Europa", "en": "Europe", "es": "Europa",
				"it": "Europa", "fr": "Europe", "ja": "ヨーロッパ",
				"pt-BR": "Europa", "ru": "Европа", "zh-CN": "欧洲",
			},
		},
		"country": gin.H{
			"geoname_id":           2635167,
			"is_in_european_union": false,
			"iso_code":             "GB",
			"names": gin.H{
				"de": "UK", "en": "United Kingdom", "es": "RU",
				"it": "Stati Uniti", "fr": "Royaume Uni", "ja": "英国",
				"pt-BR": "Reino Unido", "ru": "Британия", "zh-CN": "英国",
			},
		},
		"subdivisions": []gin.H{
			{
				"geoname_id": 6269131,
				"iso_code":   "ENG",
				"names": gin.H{
					"de": "England", "en": "England", "es": "Inglaterra",
					"it": "Inghilterra", "fr": "Angleterre", "ja": "イングランド",
					"pt-BR": "Inglaterra", "ru": "Англия", "zh-CN": "英格兰",
				},
			},
			{
				"geoname_id": 3333157,
				"iso_code":   "KEC",
				"names":      gin.H{"en": "Royal Kensington and Chelsea"},
			},
		},
	})
}

func brInventoryHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"stash": gin.H{
			"globalcash": 0,
		},
	})
}

func datarouterDataHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
