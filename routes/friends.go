package routes

import (
	"VentureBackend/static/tokens"
	"VentureBackend/utils"
	"VentureBackend/utils/friends"
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AddFriendsRoutes(r *gin.Engine) {
	r.GET("/friends/api/v1/:accountId/settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	r.GET("/friends/api/v1/:accountId/blocklist", func(c *gin.Context) {
		c.JSON(http.StatusOK, []string{})
	})

	r.GET("/friends/api/public/list/fortnite/:accountId/recentPlayers", func(c *gin.Context) {
		c.JSON(http.StatusOK, []string{})
	})

	r.Any("/friends/api/v1/:accountId/friends/:friendId/alias", tokens.VerifyToken(), GetRawBody(), func(c *gin.Context) {
		accountID := c.Param("accountId")
		friendID := c.Param("friendId")

		friendDoc, err := utils.FindFriendByAccountID(accountID)
		if err != nil || friendDoc == nil {
			c.JSON(http.StatusNotFound, gin.H{})
			return
		}

		idx := -1
		for i, f := range friendDoc.List.Accepted {
			if f.AccountID == friendID {
				idx = i
				break
			}
		}

		if idx == -1 {
			c.JSON(http.StatusNotFound, gin.H{
				"errorCode":    "errors.com.epicgames.friends.friendship_not_found",
				"errorMessage": "Friendship not found",
			})
			return
		}

		switch c.Request.Method {
		case "PUT":
			raw, _ := c.GetRawData()
			alias := string(raw)

			if len(alias) < 3 || len(alias) > 16 {
				c.JSON(http.StatusBadRequest, gin.H{
					"errorCode":    "errors.com.epicgames.validation.validation_failed",
					"errorMessage": "Validation Failed. Invalid fields were [alias]",
				})
				return
			}

			friendDoc.List.Accepted[idx].Alias = alias

		case "DELETE":
			friendDoc.List.Accepted[idx].Alias = ""
		}

		if err := utils.UpdateFriendList(friendDoc); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		c.Status(http.StatusNoContent)
	})

	r.GET("/friends/api/public/friends/:accountId", tokens.VerifyToken(), func(c *gin.Context) {
		accountID := c.Param("accountId")
		friendDoc, err := utils.FindFriendByAccountID(accountID)
		if err != nil || friendDoc == nil {
			c.JSON(http.StatusNotFound, []interface{}{})
			return
		}

		response := []gin.H{}

		for _, acc := range friendDoc.List.Accepted {
			response = append(response, gin.H{
				"accountId": acc.AccountID,
				"status":    "ACCEPTED",
				"direction": "OUTBOUND",
				"created":   acc.Created,
				"favorite":  false,
			})
		}
		for _, inc := range friendDoc.List.Incoming {
			response = append(response, gin.H{
				"accountId": inc.AccountID,
				"status":    "PENDING",
				"direction": "INBOUND",
				"created":   inc.Created,
				"favorite":  false,
			})
		}
		for _, out := range friendDoc.List.Outgoing {
			response = append(response, gin.H{
				"accountId": out.AccountID,
				"status":    "PENDING",
				"direction": "OUTBOUND",
				"created":   out.Created,
				"favorite":  false,
			})
		}

		c.JSON(http.StatusOK, response)
	})

	r.POST("/friends/api/:version/friends/:accountId/:receiverId", tokens.VerifyToken(), func(c *gin.Context) {
		senderId := c.Param("accountId")
		receiverId := c.Param("receiverId")

		sender, err := utils.FindFriendByAccountID(senderId)
		if err != nil || sender == nil {
			c.Status(http.StatusForbidden)
			return
		}

		receiver, err := utils.FindFriendByAccountID(receiverId)
		if err != nil || receiver == nil {
			c.Status(http.StatusForbidden)
			return
		}

		if friends.ContainsAccountID(sender.List.Incoming, receiver.AccountID) {
			ok, err := friends.AcceptFriendReq(senderId, receiverId)
			if err != nil || !ok {
				c.Status(http.StatusForbidden)
				return
			}
			c.Status(http.StatusNoContent)
			return
		}

		if !friends.ContainsAccountID(sender.List.Outgoing, receiver.AccountID) {
			ok, err := friends.SendFriendReq(senderId, receiverId)
			if err != nil || !ok {
				c.Status(http.StatusForbidden)
				return
			}
			c.Status(http.StatusNoContent)
			return
		}

		c.Status(http.StatusNoContent)
	})

	r.DELETE("/friends/api/:version/friends/:accountId/:receiverId", tokens.VerifyToken(), func(c *gin.Context) {
		senderId := c.Param("accountId")
		receiverId := c.Param("receiverId")

		ok, err := friends.DeleteFriend(senderId, receiverId)
		if err != nil || !ok {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.POST("/friends/api/:version/blocklist/:accountId/:receiverId", tokens.VerifyToken(), func(c *gin.Context) {
		senderId := c.Param("accountId")
		receiverId := c.Param("receiverId")

		ok, err := friends.BlockFriend(senderId, receiverId)
		if err != nil || !ok {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.DELETE("/friends/api/:version/blocklist/:accountId/:receiverId", tokens.VerifyToken(), func(c *gin.Context) {
		senderId := c.Param("accountId")
		receiverId := c.Param("receiverId")

		ok, err := friends.DeleteFriend(senderId, receiverId)
		if err != nil || !ok {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.GET("/friends/api/v1/:accountId/summary", tokens.VerifyToken(), func(c *gin.Context) {
		accountID := c.Param("accountId")
		friendDoc, err := utils.FindFriendByAccountID(accountID)
		if err != nil || friendDoc == nil {
			c.JSON(http.StatusOK, gin.H{
				"friends":   []interface{}{},
				"incoming":  []interface{}{},
				"outgoing":  []interface{}{},
				"suggested": []interface{}{},
				"blocklist": []interface{}{},
				"settings": gin.H{
					"acceptInvites": "public",
				},
			})
			return
		}

		response := gin.H{
			"friends":   []interface{}{},
			"incoming":  []interface{}{},
			"outgoing":  []interface{}{},
			"suggested": []interface{}{},
			"blocklist": []interface{}{},
			"settings": gin.H{
				"acceptInvites": "public",
			},
		}

		for _, acc := range friendDoc.List.Accepted {
			response["friends"] = append(response["friends"].([]interface{}), gin.H{
				"accountId": acc.AccountID,
				"groups":    []interface{}{},
				"mutual":    0,
				"alias":     acc.Alias,
				"note":      "",
				"favorite":  false,
				"created":   acc.Created,
			})
		}

		for _, inc := range friendDoc.List.Incoming {
			response["incoming"] = append(response["incoming"].([]interface{}), gin.H{
				"accountId": inc.AccountID,
				"mutual":    0,
				"favorite":  false,
				"created":   inc.Created,
			})
		}

		for _, out := range friendDoc.List.Outgoing {
			response["outgoing"] = append(response["outgoing"].([]interface{}), gin.H{
				"accountId": out.AccountID,
				"favorite":  false,
			})
		}

		for _, blk := range friendDoc.List.Blocked {
			response["blocklist"] = append(response["blocklist"].([]interface{}), gin.H{
				"accountId": blk.AccountID,
			})
		}

		c.JSON(http.StatusOK, response)
	})

	r.GET("/friends/api/public/blocklist/:accountId", tokens.VerifyToken(), func(c *gin.Context) {
		accountID := c.Param("accountId")
		friendDoc, err := utils.FindFriendByAccountID(accountID)
		if err != nil || friendDoc == nil {
			c.JSON(http.StatusOK, gin.H{"blockedUsers": []string{}})
			return
		}

		blocked := []string{}
		for _, b := range friendDoc.List.Blocked {
			blocked = append(blocked, b.AccountID)
		}

		c.JSON(http.StatusOK, gin.H{
			"blockedUsers": blocked,
		})
	})
}

func GetRawBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		const maxSize = 16
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusForbidden, gin.H{"error": "File size must be 16 bytes or less."})
			c.Abort()
			return
		}

		bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSize))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Something went wrong while trying to access the request body."})
			c.Abort()
			return
		}
		c.Request.Body.Close()

		c.Set("rawBody", string(bodyBytes))

		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		c.Next()
	}
}
