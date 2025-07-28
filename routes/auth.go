package routes

import (
	"net/http"
	"strings"
	"time"

	"VentureBackend/static/models"
	"VentureBackend/static/tokens"
	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
)

func AddOAuthRoutes(r *gin.Engine) {
	r.POST("/account/api/oauth/token", handleOAuthToken)
	r.GET("/account/api/oauth/verify", tokens.VerifyToken(), handleVerify)
	r.DELETE("/account/api/oauth/sessions/kill/:token", handleKillSessionByToken)
	r.DELETE("/account/api/oauth/sessions/kill", handleKillSessions)
	r.POST("/auth/v1/oauth/token", handleStaticToken)
	r.POST("/epic/oauth/v2/token", handleOAuthTokenV2)
}

func handleOAuthToken(c *gin.Context) {
	var body struct {
		GrantType    string `form:"grant_type" json:"grant_type"`
		Username     string `form:"username" json:"username"`
		Password     string `form:"password" json:"password"`
		RefreshToken string `form:"refresh_token" json:"refresh_token"`
	}

	if err := c.ShouldBind(&body); err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_request",
			"Invalid request body.",
			[]string{}, 400, "BadRequest", 400)
		return
	}

	authHeader := c.GetHeader("Authorization")
	clientDecoded, err := utils.DecodeBasicAuth(authHeader)
	if err != nil || clientDecoded.Password == "" {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_client",
			"Invalid or missing Authorization header.",
			[]string{}, 400, "BadRequest", 400)
		return
	}
	clientID := clientDecoded.Username

	switch body.GrantType {
	case "client_credentials":
		token, err := tokens.CreateClient(clientID, "client_credentials", c.ClientIP(), 4)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.common.oauth.server_error",
				"Failed to create client token.",
				[]string{}, 500, "InternalServerError", 500)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":    token,
			"expires_in":      int((4 * time.Hour).Seconds()),
			"expires_at":      time.Now().Add(4 * time.Hour).Format(time.RFC3339),
			"token_type":      "bearer",
			"client_id":       clientID,
			"internal_client": true,
			"client_service":  "fortnite",
		})
		return

	case "password":
		if body.Username == "" || body.Password == "" {
			utils.CreateError(c,
				"errors.com.epicgames.common.oauth.invalid_request",
				"Username/password is required.",
				[]string{}, 400, "BadRequest", 400)
			return
		}

		var user *models.User
		if body.Username == "hostaccount@VentureBackend.xyz" {
			user, err = utils.FindUserByEmail(body.Username)
			if err != nil || !utils.VerifyPassword(body.Password, user.Password) {
				utils.CreateError(c,
					"errors.com.epicgames.account.invalid_account_credentials",
					"Invalid credentials.",
					[]string{}, 400, "BadRequest", 400)
				return
			}
		} else {
			mobileUser, err := utils.FindMobileUserByEmail(body.Username)
			if err != nil || body.Password != mobileUser.Password {
				utils.CreateError(c,
					"errors.com.epicgames.account.invalid_account_credentials",
					"Invalid credentials.",
					[]string{}, 400, "BadRequest", 400)
				return
			}
			user, err = utils.FindUserByAccountID(mobileUser.AccountID)
			if err != nil {
				utils.CreateError(c,
					"errors.com.epicgames.account.account_not_found",
					"Account not found.",
					[]string{}, 404, "NotFound", 404)
				return
			}
		}

		if !validateUser(c, user) {
			return
		}

		accessToken, err := tokens.CreateAccess(user.AccountID, user.Username, clientID, "password", utils.GenerateDeviceID(), 8)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.common.oauth.server_error",
				"Failed to create access token.",
				[]string{}, 500, "InternalServerError", 500)
			return
		}

		refreshToken, err := tokens.CreateRefresh(user.AccountID, user.Username, clientID, "password", utils.GenerateDeviceID(), 24)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.common.oauth.server_error",
				"Failed to create refresh token.",
				[]string{}, 500, "InternalServerError", 500)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":       accessToken,
			"expires_in":         int((8 * time.Hour).Seconds()),
			"expires_at":         time.Now().Add(8 * time.Hour).Format(time.RFC3339),
			"token_type":         "bearer",
			"refresh_token":      refreshToken,
			"refresh_expires":    int((24 * time.Hour).Seconds()),
			"refresh_expires_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			"account_id":         user.AccountID,
			"client_id":          clientID,
			"displayName":        user.Username,
			"app":                "fortnite",
			"in_app_id":          user.AccountID,
			"device_id":          utils.GenerateDeviceID(),
		})
		return

	case "refresh_token":
		if body.RefreshToken == "" {
			utils.CreateError(c,
				"errors.com.epicgames.common.oauth.invalid_request",
				"Refresh token is required.",
				[]string{}, 400, "BadRequest", 400)
			return
		}

		decoded, err := tokens.DecodeJWT(body.RefreshToken)
		if err != nil || time.Now().After(decoded.ExpiresAt) {
			utils.CreateError(c,
				"errors.com.epicgames.account.auth_token.invalid_refresh_token",
				"Sorry the refresh token '"+body.RefreshToken+"' is invalid.",
				[]string{body.RefreshToken}, 18036, "invalid_grant", 400)
			return
		}

		if len(decoded.AccountID) == 0 {
			utils.CreateError(c,
				"errors.com.epicgames.account.auth_token.invalid_refresh_token",
				"Sorry the refresh token '"+body.RefreshToken+"' is invalid.",
				[]string{body.RefreshToken}, 18036, "invalid_grant", 400)
			return
		}

		accountID := decoded.AccountID

		user, err := utils.FindUserByAccountID(accountID)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.account.auth_token.invalid_refresh_token",
				"Sorry the refresh token '"+body.RefreshToken+"' is invalid.",
				[]string{body.RefreshToken}, 18036, "invalid_grant", 400)
			return
		}

		if user.Banned {
			utils.CreateError(c,
				"errors.com.epicgames.account.auth_token.invalid_refresh_token",
				"Sorry the refresh token '"+body.RefreshToken+"' is invalid.",
				[]string{body.RefreshToken}, 18036, "invalid_grant", 400)
			return
		}

		accessToken, err := tokens.CreateAccess(user.AccountID, user.Username, clientID, "refresh_token", utils.GenerateDeviceID(), 8)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.common.oauth.server_error",
				"Failed to create access token.",
				[]string{}, 500, "InternalServerError", 500)
			return
		}

		newRefreshToken, err := tokens.CreateRefresh(user.AccountID, user.Username, clientID, "refresh_token", utils.GenerateDeviceID(), 24)
		if err != nil {
			utils.CreateError(c,
				"errors.com.epicgames.common.oauth.server_error",
				"Failed to create refresh token.",
				[]string{}, 500, "InternalServerError", 500)
			return
		}

		c.Set("user", user)

		c.JSON(http.StatusOK, gin.H{
			"access_token":       accessToken,
			"expires_in":         int((8 * time.Hour).Seconds()),
			"expires_at":         time.Now().Add(8 * time.Hour).Format(time.RFC3339),
			"token_type":         "bearer",
			"refresh_token":      newRefreshToken,
			"refresh_expires":    int((24 * time.Hour).Seconds()),
			"refresh_expires_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			"account_id":         user.AccountID,
			"client_id":          clientID,
			"displayName":        user.Username,
			"app":                "fortnite",
			"in_app_id":          user.AccountID,
			"device_id":          utils.GenerateDeviceID(),
		})
		return

	default:
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.unsupported_grant_type",
			"Unsupported grant type.",
			[]string{}, 400, "BadRequest", 400)
	}
}

func handleVerify(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_request",
			"Authorization token missing.",
			[]string{},
			400,
			"BadRequest",
			400)
		return
	}
	token = strings.Replace(token, "bearer ", "", 1)
	decoded, err := tokens.DecodeJWT(token)
	if err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_token",
			"Token invalid or expired.",
			[]string{},
			401,
			"Unauthorized",
			401)
		return
	}

	accountID := decoded.Subject

	user, err := utils.FindUserByAccountID(accountID)
	if err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_token",
			"Token invalid or expired.",
			[]string{},
			401,
			"Unauthorized",
			401)
		return
	}

	if !validateUser(c, user) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":           token,
		"session_id":      decoded.ID,
		"token_type":      "bearer",
		"client_id":       decoded.ClientID,
		"internal_client": true,
		"client_service":  "fortnite",
		"account_id":      accountID,
		"expires_in":      int(time.Until(time.Unix(decoded.ExpiresAt.Unix(), 0)).Seconds()),
		"expires_at":      time.Unix(decoded.ExpiresAt.Unix(), 0).Format(time.RFC3339),
		"display_name":    user.Username,
		"app":             "fortnite",
		"in_app_id":       decoded.Subject,
		"device_id":       decoded.DeviceID,
	})
}

func handleKillSessions(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func handleKillSessionByToken(c *gin.Context) {
	token := c.Param("token")
	tokens.RemoveTokens([]string{token})
	c.Status(http.StatusNoContent)
}

func handleStaticToken(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"access_token":    "razertoken",
		"token_type":      "bearer",
		"expires_at":      "9999-12-31T23:59:59.999Z",
		"features":        []string{"AntiCheat", "Connect", "Ecom"},
		"organization_id": "razertoken",
		"product_id":      "prod-fn",
		"sandbox_id":      "fn",
		"deployment_id":   "razerdeployment",
		"expires_in":      3599,
	})
}

func handleOAuthTokenV2(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_client",
			"It appears that your Authorization header may be invalid or not present, please verify that you are sending the correct headers.",
			[]string{}, 1011, "invalid_client", 400)
		return
	}

	clientDecoded, err := utils.DecodeBasicAuth(authHeader)
	if err != nil || clientDecoded.Password == "" {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_client",
			"It appears that your Authorization header may be invalid or not present, please verify that you are sending the correct headers.",
			[]string{}, 1011, "invalid_client", 400)
		return
	}
	clientID := clientDecoded.Username

	var body struct {
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
		utils.CreateError(c,
			"errors.com.epicgames.common.oauth.invalid_request",
			"Refresh token is required.",
			[]string{}, 1013, "invalid_request", 400)
		return
	}

	refreshToken := body.RefreshToken
	index := tokens.FindRefreshTokenIndex(refreshToken)
	if index == -1 {
		utils.CreateError(c,
			"errors.com.epicgames.account.auth_token.invalid_refresh_token",
			"Sorry the refresh token '"+refreshToken+"' is invalid.",
			[]string{refreshToken}, 18036, "invalid_grant", 400)
		return
	}

	tokenData, ok := tokens.GetRefreshToken(index)
	if !ok {
		utils.CreateError(c,
			"errors.com.epicgames.account.auth_token.invalid_refresh_token",
			"Sorry the refresh token '"+refreshToken+"' is invalid.",
			[]string{refreshToken}, 18036, "invalid_grant", 400)
		return
	}
	decoded, err := tokens.DecodeJWT(strings.TrimPrefix(refreshToken, "eg1~"))
	if err != nil || time.Now().After(decoded.ExpiresAt) {
		tokens.RemoveRefreshToken(refreshToken)
		utils.CreateError(c,
			"errors.com.epicgames.account.auth_token.invalid_refresh_token",
			"Sorry the refresh token '"+refreshToken+"' is invalid.",
			[]string{refreshToken}, 18036, "invalid_grant", 400)
		return
	}

	user, err := utils.FindUserByAccountID(tokenData.AccountID)
	if err != nil {
		utils.CreateError(c,
			"errors.com.epicgames.account.account_not_found",
			"Account not found.",
			[]string{}, 18036, "invalid_grant", 404)
		return
	}

	c.Set("user", user)

	c.JSON(http.StatusOK, gin.H{
		"scope":               body.Scope,
		"token_type":          "bearer",
		"access_token":        "razertoken",
		"refresh_token":       "razerrefreshtoken",
		"id_token":            "razeridtoken",
		"expires_in":          7200,
		"expires_at":          "9999-12-31T23:59:59.999Z",
		"refresh_expires_in":  28800,
		"refresh_expires_at":  "9999-12-31T23:59:59.999Z",
		"account_id":          user.AccountID,
		"client_id":           clientID,
		"application_id":      "razersacpplicationid",
		"selected_account_id": user.AccountID,
		"merged_accounts":     []string{},
	})
}

func validateUser(c *gin.Context, user *models.User) bool {
	if user == nil {
		utils.CreateError(c,
			"errors.com.epicgames.account.account_not_found",
			"Account not found.",
			[]string{}, 404, "NotFound", 404)
		return false
	}
	if user.Banned {
		utils.CreateError(c,
			"errors.com.epicgames.account.account_not_active",
			"You have been permanently banned from Fortnite.",
			[]string{}, -1, "BadRequest", 400,
		)
		return false
	}
	return true
}
