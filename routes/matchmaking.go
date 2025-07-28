package routes

import (
	"VentureBackend/static/tokens"
	"VentureBackend/utils"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var buildUniqueId = make(map[string]string)

func AddMatchmakingRoutes(router *gin.Engine) {
	router.GET("/fortnite/api/matchmaking/session/findPlayer/:accountId", findPlayer)
	router.GET("/fortnite/api/game/v2/matchmakingservice/ticket/player/:accountId", tokens.VerifyToken(), GetMatchmakingTicket)
	router.GET("/fortnite/api/game/v2/matchmaking/account/:accountId/session/:sessionId", tokens.VerifyToken(), GetAccountSession)
	router.GET("/fortnite/api/matchmaking/session/:sessionId", tokens.VerifyToken(), GetMatchmakingSession)
	router.POST("/fortnite/api/matchmaking/session/:sessionId/join", tokens.VerifyToken(), MatchmakingJoinSession)
	router.POST("/fortnite/api/matchmaking/session/matchMakingRequest", PostMatchMakingRequest)
}

func findPlayer(c *gin.Context) {
	c.Status(http.StatusOK)
	c.Abort()
}

func GetMatchmakingTicket(c *gin.Context) {
	accountId := c.Param("accountId")

	user, err := utils.FindUserByAccountID(accountId)
	if err != nil || user == nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	if user.IsServer {
		c.Status(http.StatusForbidden)
		return
	}

	if user.MatchmakingID == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	bucketId := c.Query("bucketId")
	if bucketId == "" || len(strings.Split(bucketId, ":")) != 4 {
		c.Status(http.StatusBadRequest)
		return
	}

	parts := strings.Split(bucketId, ":")
	region := parts[2]
	playlist := parts[3]

	// Get subregions from query param: "player.subregions=IE,GB,DE,FR"
	subregions := c.Query("player.subregions")
	if subregions == "" {
		subregions = "GB" // fallback default if you want
	}

	gameServers := strings.Split(os.Getenv("GAMESERVER_IPS"), ",")
	var selectedServer string
	for _, server := range gameServers {
		serverParts := strings.Split(server, ":")
		if len(serverParts) == 3 && strings.ToLower(serverParts[2]) == playlist {
			selectedServer = server
			break
		}
	}

	if selectedServer == "" {
		utils.CreateError(c,
			"We aren't hosting this playlist right now, Check the discord server for status",
			"No server found for playlist "+playlist,
			[]string{},
			1013,
			"invalid_playlist",
			404)
		return
	}

	utils.Store.Set("playerPlaylist:"+user.AccountID, playlist)
	utils.Store.Set("playerRegion:"+user.AccountID, region)
	utils.Store.Set("playerSubregions:"+user.AccountID, subregions)

	buildUniqueId[user.AccountID] = parts[0]

	matchmakerUrl := os.Getenv("MATCHMAKER_URL")
	serviceUrl := matchmakerUrl
	if !strings.HasPrefix(matchmakerUrl, "ws") {
		serviceUrl = "ws://" + matchmakerUrl
	}

	c.JSON(http.StatusOK, gin.H{
		"serviceUrl": serviceUrl,
		"ticketType": "mms-player",
		"payload":    user.MatchmakingID,
		"signature":  "account",
	})
}

func GetAccountSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"accountId": c.Param("accountId"),
		"sessionId": c.Param("sessionId"),
		"key":       "none",
	})
}

func GetMatchmakingSession(c *gin.Context) {
	sessionId := c.Param("sessionId")

	accountIdIfc, exists := utils.Store.Get("session:" + sessionId)
	if !exists {
		utils.CreateError(c,
			"errors.com.epicgames.common.matchmaking.session.not_found",
			"No session found for "+sessionId,
			[]string{},
			1015,
			"invalid_session",
			http.StatusNotFound)
		return
	}

	accountId, ok := accountIdIfc.(string)
	if !ok || accountId == "" {
		utils.CreateError(c,
			"errors.com.epicgames.common.matchmaking.session.invalid_account",
			"Invalid account for session "+sessionId,
			[]string{},
			1016,
			"invalid_account",
			http.StatusInternalServerError)
		return
	}

	playlistIfc, _ := utils.Store.Get("playerPlaylist:" + accountId)
	playlist, _ := playlistIfc.(string)

	customKeyIfc, _ := utils.Store.Get("playerCustomKey:" + accountId)
	customKeyJSON, _ := customKeyIfc.(string)

	gameServers := strings.Split(os.Getenv("GAMESERVER_IPS"), ",")

	if customKeyJSON == "" {
		var selectedServer string
		for _, server := range gameServers {
			parts := strings.Split(server, ":")
			if len(parts) == 3 && parts[2] == playlist {
				selectedServer = server
				break
			}
		}

		if selectedServer == "" {
			utils.CreateError(c,
				"errors.com.epicgames.common.matchmaking.playlist.not_found",
				"No server found for playlist "+playlist,
				[]string{},
				1013,
				"invalid_playlist",
				http.StatusNotFound)
			return
		}

		serverInfo := map[string]string{
			"ip":       strings.Split(selectedServer, ":")[0],
			"port":     strings.Split(selectedServer, ":")[1],
			"playlist": strings.Split(selectedServer, ":")[2],
		}
		bytes, _ := json.Marshal(serverInfo)
		customKeyJSON = string(bytes)
	}

	var codeKV struct {
		IP       string `json:"ip"`
		Port     string `json:"port"`
		Playlist string `json:"playlist"`
	}
	err := json.Unmarshal([]byte(customKeyJSON), &codeKV)
	if err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.common.matchmaking.invalid_server_info",
			"Invalid server info stored",
			[]string{},
			1014,
			"invalid_server_info",
			http.StatusInternalServerError)
		return
	}

	ownerId := strings.ToUpper(strings.ReplaceAll(uuid.New().String(), "-", ""))
	sessionKey := strings.ToUpper(strings.ReplaceAll(uuid.New().String(), "-", ""))
	subregion := "GB"

	c.JSON(http.StatusOK, gin.H{
		"id":                 sessionId,
		"ownerId":            ownerId,
		"ownerName":          "[DS]fortnite-liveeugcec1c2e30ubrcore0a-z8hj-1968",
		"serverName":         "[DS]fortnite-liveeugcec1c2e30ubrcore0a-z8hj-1968",
		"serverAddress":      codeKV.IP,
		"serverPort":         codeKV.Port,
		"maxPublicPlayers":   220,
		"openPublicPlayers":  175,
		"maxPrivatePlayers":  0,
		"openPrivatePlayers": 0,
		"attributes": map[string]interface{}{
			"REGION_s":                 "EU",
			"GAMEMODE_s":               "FORTATHENA",
			"ALLOWBROADCASTING_b":      true,
			"SUBREGION_s":              subregion,
			"DCID_s":                   "FORTNITE-LIVEEUGCEC1C2E30UBRCORE0A-14840880",
			"tenant_s":                 "Fortnite",
			"MATCHMAKINGPOOL_s":        "Any",
			"STORMSHIELDDEFENSETYPE_i": 0,
			"HOTFIXVERSION_i":          0,
			"PLAYLISTNAME_s":           codeKV.Playlist,
			"SESSIONKEY_s":             sessionKey,
			"TENANT_s":                 "Fortnite",
			"BEACONPORT_i":             15009,
		},
		"publicPlayers":                   []interface{}{},
		"privatePlayers":                  []interface{}{},
		"totalPlayers":                    45,
		"allowJoinInProgress":             false,
		"shouldAdvertise":                 false,
		"isDedicated":                     false,
		"usesStats":                       false,
		"allowInvites":                    false,
		"usesPresence":                    false,
		"allowJoinViaPresence":            true,
		"allowJoinViaPresenceFriendsOnly": false,
		"buildUniqueId":                   buildUniqueId[accountId],
		"lastUpdated":                     time.Now().UTC().Format(time.RFC3339),
		"started":                         false,
	})
}

func MatchmakingJoinSession(c *gin.Context) {
	c.Status(http.StatusNoContent)
	c.Abort()
}

func PostMatchMakingRequest(c *gin.Context) {
	c.JSON(http.StatusOK, []interface{}{})
}
