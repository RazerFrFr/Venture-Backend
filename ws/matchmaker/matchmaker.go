package matchmaker

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Player struct {
	AccountID string
	Conn      *websocket.Conn
	TicketID  string
	MatchID   string
	SessionID string
}

var (
	queues         = make(map[string][]*Player)
	queueLock      sync.Mutex
	joinableQueues = make(map[string]bool)
	joinableLock   sync.Mutex
)

func md5Hash(input string) string {
	hash := md5.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	var matchmakingID string
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) >= 3 {
			matchmakingID = parts[2]
		}
	}

	var accountId string
	if matchmakingID != "" {
		user, err := utils.FindUserByMatchmakingID(matchmakingID)
		if err == nil && user != nil {
			accountId = user.AccountID
		}
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		utils.Matchmaker.Logf("WebSocket upgrade failed: %v", err)
		return
	}
	defer func() {
		ws.Close()
	}()

	now := strconv.FormatInt(time.Now().UnixNano(), 10)
	ticketId := md5Hash("1" + now)
	matchId := md5Hash("2" + now)
	sessionId := md5Hash("3" + now)

	if accountId != "" {
		utils.Store.Set("session:"+sessionId, accountId)
	}

	regionIfc, _ := utils.Store.Get("playerRegion:" + accountId)
	playlistIfc, _ := utils.Store.Get("playerPlaylist:" + accountId)

	region, _ := regionIfc.(string)
	playlist, _ := playlistIfc.(string)

	if region == "" {
		region = "default"
	}
	if playlist == "" {
		playlist = "default"
	}

	queueKey := region + ":" + playlist

	player := &Player{
		AccountID: accountId,
		Conn:      ws,
		TicketID:  ticketId,
		MatchID:   matchId,
		SessionID: sessionId,
	}

	queueLock.Lock()
	queues[queueKey] = append(queues[queueKey], player)
	queuePlayers := len(queues[queueKey])
	queueLock.Unlock()

	sendJSON := func(msg interface{}) {
		if err := ws.WriteJSON(msg); err != nil {
			utils.Matchmaker.Logf("Error sending JSON to %s: %v", accountId, err)
		}
	}

	time.AfterFunc(200*time.Millisecond, func() {
		sendJSON(map[string]interface{}{"name": "StatusUpdate", "payload": map[string]interface{}{"state": "Connecting"}})
	})

	time.AfterFunc(1200*time.Millisecond, func() {
		sendJSON(map[string]interface{}{"name": "StatusUpdate", "payload": map[string]interface{}{
			"totalPlayers":     queuePlayers,
			"connectedPlayers": queuePlayers,
			"state":            "Waiting",
		}})
	})

	time.AfterFunc(2200*time.Millisecond, func() {
		broadcastQueuedStatus(queueKey)
	})

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			assignSessionsIfReady(queueKey)
		}
	}()

	ws.SetCloseHandler(func(code int, text string) error {
		queueLock.Lock()
		players := queues[queueKey]
		for i, p := range players {
			if p.AccountID == player.AccountID && p.TicketID == player.TicketID {
				queues[queueKey] = append(players[:i], players[i+1:]...)
				break
			}
		}
		queueLock.Unlock()
		broadcastQueuedStatus(queueKey)
		return nil
	})

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			utils.Matchmaker.Logf("ReadMessage error for player %s: %v", accountId, err)
			break
		}
	}
}

func broadcastQueuedStatus(queueKey string) {
	queueLock.Lock()
	defer queueLock.Unlock()

	players := queues[queueKey]

	for _, p := range players {
		if p.Conn != nil {
			if err := p.Conn.WriteJSON(map[string]interface{}{
				"name": "StatusUpdate",
				"payload": map[string]interface{}{
					"ticketId":         p.TicketID,
					"queuedPlayers":    len(players),
					"estimatedWaitSec": 0,
					"status":           map[string]interface{}{},
					"state":            "Queued",
				},
			}); err != nil {
				utils.Matchmaker.Logf("Error broadcasting to player %s: %v", p.AccountID, err)
			}
		}
	}
}

func assignSessionsIfReady(queueKey string) {
	queueLock.Lock()
	defer queueLock.Unlock()

	joinableLock.Lock()
	canAssign := joinableQueues[queueKey]
	joinableLock.Unlock()

	if !canAssign {
		return
	}

	players := queues[queueKey]
	if len(players) == 0 {
		return
	}

	assignPlayers := players
	queues[queueKey] = []*Player{}

	for _, p := range assignPlayers {
		if err := p.Conn.WriteJSON(map[string]interface{}{
			"name": "StatusUpdate",
			"payload": map[string]interface{}{
				"matchId": p.MatchID,
				"state":   "SessionAssignment",
			},
		}); err != nil {
			utils.Matchmaker.Logf("Error sending session assignment to %s: %v", p.AccountID, err)
		}

		go func(player *Player) {
			time.Sleep(2 * time.Second)
			if err := player.Conn.WriteJSON(map[string]interface{}{
				"name": "Play",
				"payload": map[string]interface{}{
					"matchId":      player.MatchID,
					"sessionId":    player.SessionID,
					"joinDelaySec": 1,
				},
			}); err != nil {
				utils.Matchmaker.Logf("Error sending play message to %s: %v", player.AccountID, err)
			}
		}(p)
	}

	broadcastQueuedStatus(queueKey)
}

func InitMatchmaker() {
	port := os.Getenv("MATCHMAKER_PORT")
	if port == "" {
		port = "80"
	}

	// Debug Mode
	//router := gin.Default()

	// Release mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/api/VentureBackend/matchmaker/:region/:playlist", func(c *gin.Context) {
		apiKey := c.Query("apikey")
		joinableStr := c.Query("joinable")

		expectedApiKey := os.Getenv("bApiKey")
		if apiKey != expectedApiKey {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing API key"})
			return
		}

		if joinableStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'joinable' query parameter"})
			return
		}

		joinable := false
		if joinableStr == "true" {
			joinable = true
		} else if joinableStr != "false" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value for 'joinable' (expected 'true' or 'false')"})
			return
		}

		region := c.Param("region")
		playlist := c.Param("playlist")
		queueKey := region + ":" + playlist

		joinableLock.Lock()
		joinableQueues[queueKey] = joinable
		joinableLock.Unlock()

		msg := "Successfully set server status to off"
		if joinable {
			msg = "Successfully set server status to on"
		}

		c.JSON(http.StatusOK, gin.H{"message": msg})
	})

	server := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if websocket.IsWebSocketUpgrade(r) {
				websocketHandler(w, r)
				return
			}
			router.ServeHTTP(w, r)
		}),
	}

	utils.Matchmaker.Logf("Matchmaker started listening on port :%s", port)
	if err := server.ListenAndServe(); err != nil {
		utils.Matchmaker.Logf("Matchmaker server failed: %v", err)
	}
}
