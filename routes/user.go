package routes

import (
	"net/http"
	"regexp"
	"strings"

	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"VentureBackend/static/models"
	"VentureBackend/static/tokens"
	"VentureBackend/utils"
)

func RegisterAccountRoutes(r *gin.Engine, userCollection *mongo.Collection) {
	r.GET("/account/api/public/account", func(c *gin.Context) {
		query := c.Request.URL.Query()
		accountIds := query["accountId"]
		response := []gin.H{}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if len(accountIds) == 1 {
			filter := bson.M{"accountId": accountIds[0], "banned": false}
			var user models.User
			err := userCollection.FindOne(ctx, filter).Decode(&user)
			if err == nil {
				response = append(response, gin.H{
					"id":            user.AccountID,
					"displayName":   user.Username,
					"externalAuths": gin.H{},
				})
			}
		} else if len(accountIds) > 1 {
			filter := bson.M{"accountId": bson.M{"$in": accountIds}, "banned": false}
			cur, err := userCollection.Find(ctx, filter, options.Find().SetLimit(100))
			if err == nil {
				defer cur.Close(ctx)
				for cur.Next(ctx) {
					var user models.User
					if err := cur.Decode(&user); err == nil {
						response = append(response, gin.H{
							"id":            user.AccountID,
							"displayName":   user.Username,
							"externalAuths": gin.H{},
						})
					}
					if len(response) >= 100 {
						break
					}
				}
			}
		}

		c.JSON(http.StatusOK, response)
	})

	r.GET("/account/api/public/account/displayName/:displayName", func(c *gin.Context) {
		displayName := c.Param("displayName")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		filter := bson.M{
			"username": bson.M{"$regex": "^" + regexp.QuoteMeta(displayName) + "$", "$options": "i"},
			"banned":   false,
		}

		var user models.User
		err := userCollection.FindOne(ctx, filter).Decode(&user)
		if err != nil || user.IsServer {
			utils.CreateError(c,
				"errors.com.epicgames.account.account_not_found",
				"Sorry, we couldn't find an account for "+displayName,
				[]string{displayName}, 18007, "account_not_found", http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":            user.AccountID,
			"displayName":   user.Username,
			"externalAuths": gin.H{},
		})
	})

	r.GET("/persona/api/public/account/lookup", func(c *gin.Context) {
		q := c.Query("q")

		if q == "" {
			utils.CreateError(c,
				"errors.com.epicgames.bad_request",
				"Required String parameter 'q' is invalid or not present",
				nil, 1001, "parameter_invalid", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		filter := bson.M{"username_lower": strings.ToLower(q), "banned": false}
		var user models.User
		err := userCollection.FindOne(ctx, filter).Decode(&user)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.account.account_not_found",
				"Sorry, we couldn't find an account for "+q,
				[]string{q}, 18007, "account_not_found", http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":            user.AccountID,
			"displayName":   user.Username,
			"externalAuths": gin.H{},
		})
	})

	r.GET("/api/v1/search/:accountId", func(c *gin.Context) {
		//accountId := c.Param("accountId")
		prefix := c.Query("prefix")

		if prefix == "" {
			utils.CreateError(c,
				"errors.com.epicgames.bad_request",
				"Required String parameter 'prefix' is invalid or not present",
				nil, 1001, "bad_request", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		regexPattern := "^" + regexp.QuoteMeta(prefix)
		filter := bson.M{
			"username": bson.M{"$regex": regexPattern},
			"banned":   false,
		}
		cur, err := userCollection.Find(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer cur.Close(ctx)

		response := []gin.H{}
		for cur.Next(ctx) {
			var user models.User
			if err := cur.Decode(&user); err == nil {
				matchType := "prefix"
				if strings.ToLower(prefix) == strings.ToLower(user.Username) {
					matchType = "exact"
				}
				response = append(response, gin.H{
					"accountId": user.AccountID,
					"matches": []gin.H{
						{"value": user.Username, "platform": "epic"},
					},
					"matchType":    matchType,
					"epicMutuals":  0,
					"sortPosition": len(response),
				})
			}
			if len(response) >= 100 {
				break
			}
		}

		c.JSON(http.StatusOK, response)
	})

	r.GET("/account/api/public/account/:accountId", tokens.VerifyToken(), func(c *gin.Context) {
		accountId := c.Param("accountId")

		user, err := utils.FindUserByAccountID(accountId)
		if err != nil || user == nil {
			utils.CreateError(c,
				"errors.com.epicgames.account.account_not_found",
				"Account not found",
				nil, 18007, "account_not_found", http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":                         user.AccountID,
			"displayName":                user.Username,
			"name":                       "Razer Hosting",
			"email":                      "[hidden]@" + strings.SplitN(user.Email, "@", 2)[1],
			"failedLoginAttempts":        0,
			"lastLogin":                  time.Now().Format(time.RFC3339),
			"numberOfDisplayNameChanges": 0,
			"ageGroup":                   "UNKNOWN",
			"headless":                   false,
			"country":                    "US",
			"lastName":                   "Server",
			"preferredLanguage":          "en",
			"canUpdateDisplayName":       false,
			"tfaEnabled":                 false,
			"emailVerified":              true,
			"minorVerified":              false,
			"minorExpected":              false,
			"minorStatus":                "NOT_MINOR",
			"cabinedMode":                false,
			"hasHashedEmail":             false,
		})
	})

	r.GET("/sdk/v1/*filepath", func(c *gin.Context) {
		sdkJSON, err := utils.LoadJSON("./static/responses/sdkv1.json")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load SDK data"})
			return
		}
		c.JSON(http.StatusOK, sdkJSON)
	})

	r.GET("/epic/id/v2/sdk/accounts", func(c *gin.Context) {
		accountId := c.Query("accountId")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		filter := bson.M{"accountId": accountId, "banned": false}
		var user models.User
		err := userCollection.FindOne(ctx, filter).Decode(&user)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.account.account_not_found",
				"Sorry, we couldn't find an account for "+accountId,
				[]string{accountId}, 18007, "account_not_found", http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, []gin.H{{
			"accountId":         user.AccountID,
			"displayName":       user.Username,
			"preferredLanguage": "en",
			"cabinedMode":       false,
			"empty":             false,
		}})
	})

	r.Any("/fortnite/api/game/v2/profileToken/verify/:accountId", func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			err := utils.MethodNotAllowedError(c)
			c.Header(err.Header, "POST")
			c.Status(http.StatusMethodNotAllowed)
			c.Writer.WriteString(err.Message)
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.Any("/v1/epic-settings/public/users/:filepath/values", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	r.GET("/account/api/public/account/:accountId/externalAuths", func(c *gin.Context) {
		c.JSON(http.StatusOK, []string{})
	})

	r.GET("/account/api/epicdomains/ssodomains", func(c *gin.Context) {
		c.JSON(http.StatusOK, []string{
			"unrealengine.com",
			"unrealtournament.com",
			"fortnite.com",
			"epicgames.com",
		})
	})

	r.POST("/fortnite/api/game/v2/tryPlayOnPlatform/account/:accountId", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, "true")
	})
}
