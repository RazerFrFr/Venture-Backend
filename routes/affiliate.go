package routes

import (
	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
)

func RegisterAffiliateRoutes(router *gin.Engine) {
	affiliate := router.Group("/affiliate/api/public/affiliates")
	{
		affiliate.GET("/slug/:slug", func(c *gin.Context) {
			utils.CreateError(c,
				"errors.com.epicgames.fortnite.sac_disabled",
				"SAC Codes are disabled",
				nil,
				12801,
				"Bad Request",
				400)
		})
	}
}
