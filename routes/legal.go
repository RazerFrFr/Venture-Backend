package routes

import (
	"VentureBackend/utils"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

func AddEulaRoutes(r *gin.Engine) {
	data, err := ioutil.ReadFile(filepath.Join(".", "static", "responses", "EULA", "eula.json"))
	if err != nil {
		panic("Failed to load EULA JSON: " + err.Error())
	}

	var eulaJson interface{}
	if err := json.Unmarshal(data, &eulaJson); err != nil {
		panic("Failed to parse EULA JSON: " + err.Error())
	}

	r.GET("/eulatracking/api/shared/agreements/fn", func(c *gin.Context) {
		c.JSON(http.StatusOK, eulaJson)
	})

	r.GET("/eulatracking/api/public/agreements/:platform/account/:accountId", func(c *gin.Context) {
		accountId := c.Param("accountId")

		user, err := utils.FindUserByAccountID(accountId)
		if err != nil || user == nil {
			utils.CreateError(c,
				"errors.com.epicgames.account.account_not_found",
				"Account not found",
				nil, 18007, "account_not_found", http.StatusNotFound)
			return
		}

		if !user.AcceptedEULA {
			c.JSON(http.StatusOK, eulaJson)
		} else {
			c.Status(http.StatusNoContent)
		}
	})

	r.POST("/eulatracking/api/public/agreements/:platform/version/:version/account/:accountId/accept", func(c *gin.Context) {
		accountId := c.Param("accountId")
		user, err := utils.FindUserByAccountID(accountId)
		if err != nil || user == nil {
			utils.CreateError(c,
				"errors.com.epicgames.account.account_not_found",
				"Account not found",
				nil, 18007, "account_not_found", http.StatusNotFound)
			return
		}

		if !user.AcceptedEULA {
			db := os.Getenv("DB_NAME")
			collection := utils.MongoClient.Database(db).Collection("users")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			filter := bson.M{"accountId": accountId}
			update := bson.M{"$set": bson.M{"acceptedEULA": true}}

			_, err := collection.UpdateOne(ctx, filter, update)
			if err != nil {
				utils.CreateError(c,
					"errors.com.epicgames.account.update_failed",
					"Failed to update EULA status.",
					nil, 50000, "update_failed", http.StatusInternalServerError)
				return
			}
		}

		c.Status(http.StatusNoContent)
	})
}
