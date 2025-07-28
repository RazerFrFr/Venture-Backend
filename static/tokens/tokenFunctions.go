package tokens

import (
	log "VentureBackend/utils"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var (
	JWTSecret  []byte
	tokensPath = "./static/tokens/tokens.json"
	mu         sync.Mutex
	tokenStore = TokenStore{}
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Backend.Log("Warning: .env file not found or failed to load")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Backend.Log("JWT_SECRET not set in environment variables")
	}
	JWTSecret = []byte(secret)
	err = loadTokens()
	if err != nil {
		log.Backend.Log("Failed to load tokens from file: ", err)
	}
}

type ClientToken struct {
	IP        string    `json:"ip"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type AccessToken struct {
	AccountID string    `json:"accountId"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type RefreshToken struct {
	AccountID string    `json:"accountId"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type TokenStore struct {
	AccessTokens  []AccessToken  `json:"accessTokens"`
	ClientTokens  []ClientToken  `json:"clientTokens"`
	RefreshTokens []RefreshToken `json:"refreshTokens"`
}

type DecodedToken struct {
	ID          string
	Subject     string
	Issuer      string
	Audience    string
	ExpiresAt   time.Time
	GrantType   string
	TokenType   string
	ClientID    string
	DeviceID    string
	DisplayName string
	AccountID   string
	RawClaims   jwt.MapClaims
}

func DecodeJWT(tokenWithPrefix string) (*DecodedToken, error) {
	if !strings.HasPrefix(tokenWithPrefix, "eg1~") {
		return nil, errors.New("invalid token prefix")
	}
	tokenStr := strings.TrimPrefix(tokenWithPrefix, "eg1~")

	token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid JWT claims")
	}

	sub, _ := claims["sub"].(string)
	iss, _ := claims["iss"].(string)
	aud, _ := claims["aud"].(string)
	jti, _ := claims["jti"].(string)
	am, _ := claims["am"].(string)
	t, _ := claims["t"].(string)
	clid, _ := claims["clid"].(string)
	dvid, _ := claims["dvid"].(string)
	dn, _ := claims["dn"].(string)

	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return nil, errors.New("missing or invalid exp claim")
	}
	exp := time.Unix(int64(expFloat), 0)

	return &DecodedToken{
		ID:          jti,
		Subject:     sub,
		Issuer:      iss,
		Audience:    aud,
		ExpiresAt:   exp,
		GrantType:   am,
		TokenType:   t,
		ClientID:    clid,
		DeviceID:    dvid,
		AccountID:   aud,
		DisplayName: dn,
		RawClaims:   claims,
	}, nil
}

func loadTokens() error {
	mu.Lock()
	defer mu.Unlock()

	err := os.MkdirAll("./static/tokens", os.ModePerm)
	if err != nil {
		log.Error.Log("Failed to create tokens directory: ", err)
		return errors.New("failed to create tokens directory")
	}

	if _, err := os.Stat(tokensPath); os.IsNotExist(err) {
		empty := TokenStore{
			AccessTokens:  []AccessToken{},
			ClientTokens:  []ClientToken{},
			RefreshTokens: []RefreshToken{},
		}
		data, _ := json.MarshalIndent(empty, "", "  ")
		if err := ioutil.WriteFile(tokensPath, data, 0644); err != nil {
			log.Error.Log("Failed to create empty tokens file: ", err)
			return errors.New("failed to create empty tokens file")
		}
	}

	data, err := ioutil.ReadFile(tokensPath)
	if err != nil {
		log.Error.Log("Failed to read tokens file: ", err)
		return errors.New("failed to read tokens file")
	}

	err = json.Unmarshal(data, &tokenStore)
	if err != nil {
		log.Error.Log("Failed to unmarshal tokens: ", err)
		return errors.New("failed to unmarshal tokens")
	}

	return nil
}

func saveTokens() error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(tokenStore, "", "  ")
	if err != nil {
		log.Error.Log("Marshal error while saving tokens: ", err)
		return err
	}
	err = ioutil.WriteFile(tokensPath, data, 0644)
	if err != nil {
		log.Error.Log("WriteFile error while saving tokens: ", err)
		return err
	}
	return nil
}

func MakeID() string {
	return uuid.New().String()
}

func encodeBase64(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}

func createToken(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

func CreateClient(clientId, grantType, ip string, expiresIn int) (string, error) {
	now := time.Now()
	expiry := now.Add(time.Duration(expiresIn) * time.Hour)
	claims := jwt.MapClaims{
		"p":             encodeBase64(MakeID()),
		"clsvc":         "fortnite",
		"t":             "s",
		"mver":          false,
		"clid":          clientId,
		"ic":            true,
		"am":            grantType,
		"jti":           strings.ReplaceAll(MakeID(), "-", ""),
		"creation_date": now.Unix(),
		"hours_expire":  expiresIn,
		"exp":           expiry.Unix(),
	}

	tokenStr, err := createToken(claims)
	if err != nil {
		return "", err
	}
	tokenWithPrefix := "eg1~" + tokenStr

	mu.Lock()
	newList := []ClientToken{}
	for _, t := range tokenStore.ClientTokens {
		if t.IP != ip {
			newList = append(newList, t)
		}
	}
	tokenStore.ClientTokens = newList
	tokenStore.ClientTokens = append(tokenStore.ClientTokens, ClientToken{
		IP:        ip,
		Token:     tokenWithPrefix,
		CreatedAt: now,
		ExpiresAt: expiry,
	})
	mu.Unlock()

	if err := saveTokens(); err != nil {
		return "", err
	}
	return tokenWithPrefix, nil
}

func CreateAccess(accountId, username, clientId, grantType, deviceId string, expiresIn int) (string, error) {
	now := time.Now()
	expiry := now.Add(time.Duration(expiresIn) * time.Hour)
	claims := jwt.MapClaims{
		"app":           "fortnite",
		"sub":           accountId,
		"dvid":          deviceId,
		"mver":          false,
		"clid":          clientId,
		"dn":            username,
		"am":            grantType,
		"p":             encodeBase64(MakeID()),
		"iai":           accountId,
		"sec":           1,
		"clsvc":         "fortnite",
		"t":             "s",
		"ic":            true,
		"jti":           strings.ReplaceAll(MakeID(), "-", ""),
		"creation_date": now.Unix(),
		"hours_expire":  expiresIn,
		"exp":           expiry.Unix(),
	}

	tokenStr, err := createToken(claims)
	if err != nil {
		return "", err
	}
	tokenWithPrefix := "eg1~" + tokenStr

	mu.Lock()
	newList := []AccessToken{}
	for _, t := range tokenStore.AccessTokens {
		if t.AccountID != accountId {
			newList = append(newList, t)
		}
	}
	tokenStore.AccessTokens = newList
	tokenStore.AccessTokens = append(tokenStore.AccessTokens, AccessToken{
		AccountID: accountId,
		Token:     tokenWithPrefix,
		CreatedAt: now,
		ExpiresAt: expiry,
	})
	mu.Unlock()

	if err := saveTokens(); err != nil {
		return "", err
	}
	return tokenWithPrefix, nil
}

func CreateRefresh(accountId, username, clientId, grantType, deviceId string, expiresIn int) (string, error) {
	now := time.Now()
	expiry := now.Add(time.Duration(expiresIn) * time.Hour)
	claims := jwt.MapClaims{
		"sub":           accountId,
		"dvid":          deviceId,
		"t":             "r",
		"dn":            username,
		"clid":          clientId,
		"iai":           accountId,
		"am":            grantType,
		"aud":           accountId,
		"jti":           strings.ReplaceAll(MakeID(), "-", ""),
		"creation_date": now.Unix(),
		"hours_expire":  expiresIn,
		"exp":           expiry.Unix(),
	}

	tokenStr, err := createToken(claims)
	if err != nil {
		return "", err
	}
	tokenWithPrefix := "eg1~" + tokenStr

	mu.Lock()
	newList := []RefreshToken{}
	for _, t := range tokenStore.RefreshTokens {
		if t.AccountID != accountId {
			newList = append(newList, t)
		}
	}
	tokenStore.RefreshTokens = newList
	tokenStore.RefreshTokens = append(tokenStore.RefreshTokens, RefreshToken{
		AccountID: accountId,
		Token:     tokenWithPrefix,
		CreatedAt: now,
		ExpiresAt: expiry,
	})
	mu.Unlock()

	if err := saveTokens(); err != nil {
		return "", err
	}
	return tokenWithPrefix, nil
}

func tokenExistsInAccessTokens(token string) bool {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	for _, t := range tokenStore.AccessTokens {
		if t.Token == token && now.Before(t.ExpiresAt) {
			return true
		}
	}
	return false
}

func FindAccessToken(token string) *AccessToken {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	for i, t := range tokenStore.AccessTokens {
		if t.Token == token && now.Before(t.ExpiresAt) {
			return &tokenStore.AccessTokens[i]
		}
	}
	return nil
}

func findClientToken(token string) *ClientToken {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	for i, t := range tokenStore.ClientTokens {
		if t.Token == token && now.Before(t.ExpiresAt) {
			return &tokenStore.ClientTokens[i]
		}
	}
	return nil
}

func VerifyToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		authErr := func() {
			c.Header("X-Epic-Error-Name", "errors.com.epicgames.common.authorization.authorization_failed")
			c.Header("X-Epic-Error-Code", "1032")
			c.JSON(http.StatusUnauthorized, gin.H{
				"errorCode":        "errors.com.epicgames.common.authorization.authorization_failed",
				"errorMessage":     fmt.Sprintf("Authorization failed for %s", c.Request.RequestURI),
				"messageVars":      []string{c.Request.RequestURI},
				"numericErrorCode": 1032,
			})
			c.Abort()
		}
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) < 7 || !strings.EqualFold(authHeader[:7], "bearer ") {
			authErr()
			return
		}
		tokenPart := authHeader[7:]
		if !strings.HasPrefix(tokenPart, "eg1~") {
			authErr()
			return
		}
		tokenStr := strings.TrimPrefix(tokenPart, "eg1~")
		token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
		if err != nil {
			authErr()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			authErr()
			return
		}
		tokenWithPrefix := "eg1~" + tokenStr
		if !tokenExistsInAccessTokens(tokenWithPrefix) {
			authErr()
			return
		}
		creationDateUnix, ok := claims["creation_date"].(float64)
		if !ok {
			authErr()
			return
		}
		hoursExpire, ok := claims["hours_expire"].(float64)
		if !ok {
			authErr()
			return
		}
		expiry := time.Unix(int64(creationDateUnix), 0).Add(time.Duration(hoursExpire) * time.Hour)
		if time.Now().After(expiry) {
			authErr()
			return
		}
		c.Next()
	}
}

func FindAccessTokenIndex(token string) int {
	for i, t := range tokenStore.AccessTokens {
		if t.Token == token {
			return i
		}
	}
	return -1
}

func RemoveAccessToken(token string) {
	mu.Lock()
	defer mu.Unlock()
	index := FindAccessTokenIndex(token)
	if index != -1 {
		tokenStore.AccessTokens = append(tokenStore.AccessTokens[:index], tokenStore.AccessTokens[index+1:]...)
		if err := saveTokens(); err != nil {
			log.Backend.Log("Failed to save tokens after removing access token: ", err)
		}
	}
}

func FindClientTokenIndex(token string) int {
	for i, t := range tokenStore.ClientTokens {
		if t.Token == token {
			return i
		}
	}
	return -1
}

func RemoveClientToken(token string) {
	mu.Lock()
	defer mu.Unlock()
	index := FindClientTokenIndex(token)
	if index != -1 {
		tokenStore.ClientTokens = append(tokenStore.ClientTokens[:index], tokenStore.ClientTokens[index+1:]...)
		if err := saveTokens(); err != nil {
			log.Backend.Log("Failed to save tokens after removing client token: ", err)
		}
	}
}

func FindRefreshTokenIndex(token string) int {
	for i, t := range tokenStore.RefreshTokens {
		if t.Token == token {
			return i
		}
	}
	return -1
}

func RemoveRefreshToken(token string) {
	mu.Lock()
	defer mu.Unlock()
	index := FindRefreshTokenIndex(token)
	if index != -1 {
		tokenStore.RefreshTokens = append(tokenStore.RefreshTokens[:index], tokenStore.RefreshTokens[index+1:]...)
		if err := saveTokens(); err != nil {
			log.Backend.Log("Failed to save tokens after removing refresh token: ", err)
		}
	}
}

func GetRefreshToken(index int) (RefreshToken, bool) {
	mu.Lock()
	defer mu.Unlock()
	if index >= 0 && index < len(tokenStore.RefreshTokens) {
		return tokenStore.RefreshTokens[index], true
	}
	return RefreshToken{}, false
}

func RemoveTokens(tokensToRemove []string) {
	mu.Lock()
	now := time.Now()

	removeToken := func(token string) {
		if idx := FindAccessTokenIndex(token); idx != -1 {
			tokenStore.AccessTokens[idx].ExpiresAt = now
			tokenStore.AccessTokens = append(tokenStore.AccessTokens[:idx], tokenStore.AccessTokens[idx+1:]...)
		}
		if idx := FindClientTokenIndex(token); idx != -1 {
			tokenStore.ClientTokens[idx].ExpiresAt = now
			tokenStore.ClientTokens = append(tokenStore.ClientTokens[:idx], tokenStore.ClientTokens[idx+1:]...)
		}
		if idx := FindRefreshTokenIndex(token); idx != -1 {
			tokenStore.RefreshTokens[idx].ExpiresAt = now
			tokenStore.RefreshTokens = append(tokenStore.RefreshTokens[:idx], tokenStore.RefreshTokens[idx+1:]...)
		}
	}

	for _, token := range tokensToRemove {
		removeToken(token)
	}
	mu.Unlock()

	if err := saveTokens(); err != nil {
		log.Error.Log("Failed to save tokens after removing tokens: ", err)
	}
}

func GetTokensByAccountID(accountID string) []string {
	mu.Lock()
	defer mu.Unlock()

	var tokens []string

	for _, t := range tokenStore.AccessTokens {
		if t.AccountID == accountID {
			tokens = append(tokens, t.Token)
		}
	}
	for _, t := range tokenStore.RefreshTokens {
		if t.AccountID == accountID {
			tokens = append(tokens, t.Token)
		}
	}

	return tokens
}
