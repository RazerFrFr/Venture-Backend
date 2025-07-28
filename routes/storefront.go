package routes

import (
	"VentureBackend/static/tokens"
	"VentureBackend/utils"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func RegisterStorefrontRoutes(router *gin.Engine) {
	router.GET("/fortnite/api/storefront/v2/catalog", tokens.VerifyToken(), CatalogHandler)
	router.GET("/fortnite/api/storefront/v2/keychain", KeychainHandler)
}

func CatalogHandler(c *gin.Context) {
	catalog := utils.GetItemShop()
	c.JSON(http.StatusOK, catalog)
}

func KeychainHandler(c *gin.Context) {
	data, err := ioutil.ReadFile(filepath.Join(".", "static", "responses", "keychain.json"))
	if err != nil {
		panic("Failed to load EULA JSON: " + err.Error())
	}

	var keychainJson interface{}
	if err := json.Unmarshal(data, &keychainJson); err != nil {
		panic("Failed to parse EULA JSON: " + err.Error())
	}

	c.JSON(http.StatusOK, keychainJson)
}
