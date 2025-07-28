package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func AddLightswitchRoutes(r *gin.Engine) {
	r.GET("/lightswitch/api/service/Fortnite/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"serviceInstanceId":  "fortnite",
			"status":             "UP",
			"message":            "Fortnite is online",
			"maintenanceUri":     nil,
			"overrideCatalogIds": []string{"a7f138b2e51945ffbfdacc1af0541053"},
			"allowedActions":     []string{},
			"banned":             false,
			"launcherInfoDTO": gin.H{
				"appName":       "Fortnite",
				"catalogItemId": "4fe75bbc5a674f4f9b356b5c90567da5",
				"namespace":     "fn",
			},
		})
	})

	r.GET("/lightswitch/api/service/bulk/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, []gin.H{
			{
				"serviceInstanceId":  "fortnite",
				"status":             "UP",
				"message":            "fortnite is up.",
				"maintenanceUri":     nil,
				"overrideCatalogIds": []string{"a7f138b2e51945ffbfdacc1af0541053"},
				"allowedActions":     []string{"PLAY", "DOWNLOAD"},
				"banned":             false,
				"launcherInfoDTO": gin.H{
					"appName":       "Fortnite",
					"catalogItemId": "4fe75bbc5a674f4f9b356b5c90567da5",
					"namespace":     "fn",
				},
			},
		})
	})
}
