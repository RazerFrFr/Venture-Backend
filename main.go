package main

import (
	discord "VentureBackend/bot"
	"VentureBackend/routes"
	"VentureBackend/utils"
	"VentureBackend/ws/matchmaker"
	"VentureBackend/ws/xmpp"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		utils.Error.Log("No .env file found, using defaults")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3551"
	}

	utils.Backend.Logf("Starting server on port %s...", port)
	utils.InitMongoDB()
	go xmpp.InitXMPP()
	go matchmaker.InitMatchmaker()
	go discord.InitBot()

	// Debug Mode
	r := gin.Default()

	// Release mode (use later)
	/*gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())*/

	r.GET("/", func(c *gin.Context) {
		c.String(200, "Venture Backend made by Razer (originally used (was planned to be) for Razer Hosting).")
	})

	r.GET("/unknown", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Venture Backend by Razer (originally used (was planned to be) for Razer Hosting).",
		})
	})

	r.NoRoute(func(c *gin.Context) {
		utils.CreateError(c,
			"errors.com.epicgames.common.not_found",
			"Sorry the resource you were trying to find could not be found",
			nil,
			1004,
			"Not Found",
			404)
	})

	RegisterRoutesEndpoints(r)

	if err := r.Run(":" + port); err != nil {
		utils.Error.Logf("Failed to start server: %v", err)
	}
}

func RegisterRoutesEndpoints(router *gin.Engine) {
	routes.RegisterAffiliateRoutes(router)
	routes.AddOAuthRoutes(router)
	routes.AddCloudStorageRoutes(router)
	routes.AddContentPagesRoutes(router)
	routes.AddEulaRoutes(router)
	routes.AddLightswitchRoutes(router)
	routes.RegisterMiscRoutes(router)
	routes.RegisterPrivacyRoutes(router)
	routes.RegisterAccountRoutes(router, utils.MobileCollection.Database().Collection("users"))
	routes.RegisterVersionRoutes(router)
	routes.RegisterMCPRoutes(router)
	routes.RegisterStorefrontRoutes(router)
	routes.RegisterTimelineRoutes(router)
	routes.AddFriendsRoutes(router)
	routes.AddMatchmakingRoutes(router)
}
