package routes

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"VentureBackend/static/tokens"
	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
)

func AddCloudStorageRoutes(r *gin.Engine) {
	r.GET("/fortnite/api/cloudstorage/system", listSystemFiles)
	r.GET("/fortnite/api/cloudstorage/system/:file", serveSystemFile)
	r.GET("/fortnite/api/cloudstorage/user/:accountId/:file", tokens.VerifyToken(), serveUserFile)
	r.GET("/fortnite/api/cloudstorage/user/:accountId", tokens.VerifyToken(), listUserFiles)
	r.PUT("/fortnite/api/cloudstorage/user/:accountId/:file", tokens.VerifyToken(), limitBodySize(400000), saveUserFile)
}

func listSystemFiles(c *gin.Context) {
	dir := filepath.Join(".", "CloudStorage")
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var cloudFiles []gin.H
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".ini") {
			continue
		}
		full := filepath.Join(dir, f.Name())
		data, _ := ioutil.ReadFile(full)
		h1 := sha1.Sum(data)
		h2 := sha256.Sum256(data)
		cloudFiles = append(cloudFiles, gin.H{
			"uniqueFilename": f.Name(),
			"filename":       f.Name(),
			"hash":           hex.EncodeToString(h1[:]),
			"hash256":        hex.EncodeToString(h2[:]),
			"length":         len(data),
			"contentType":    "application/octet-stream",
			"uploaded":       f.ModTime(),
			"storageType":    "S3",
			"storageIds":     gin.H{},
			"doNotCache":     true,
		})
	}
	c.JSON(200, cloudFiles)
}

func serveSystemFile(c *gin.Context) {
	name := filepath.Base(c.Param("file"))
	if strings.Contains(name, "..") || strings.Contains(name, "~") {
		c.Status(404)
		return
	}
	path := filepath.Join(".", "CloudStorage", name)
	if b, err := ioutil.ReadFile(path); err == nil {
		c.Data(200, "application/octet-stream", b)
		return
	}
	c.Status(200)
}

func serveUserFile(c *gin.Context) {
	file := filepath.Base(c.Param("file"))
	if !strings.Contains(strings.ToLower(file), "clientsettings") {
		c.Status(200)
		return
	}

	accountId := c.Param("accountId")
	user, err := utils.FindUserByAccountID(accountId)
	if err != nil || user == nil {
		utils.CreateError(c,
			"errors.com.epicgames.account.account_not_found",
			"Account not found",
			nil, 18007, "account_not_found", http.StatusNotFound)
		return
	}
	dir := filepath.Join(".", "ClientSettings", user.AccountID)
	os.MkdirAll(dir, 0755)

	p := filepath.Join(dir, "ClientSettings.Sav")
	if b, err := ioutil.ReadFile(p); err == nil {
		c.Data(200, "application/octet-stream", b)
		return
	}
	c.Status(200)
}

func listUserFiles(c *gin.Context) {
	accountId := c.Param("accountId")
	user, err := utils.FindUserByAccountID(accountId)
	if err != nil || user == nil {
		utils.CreateError(c,
			"errors.com.epicgames.account.account_not_found",
			"Account not found",
			nil, 18007, "account_not_found", http.StatusNotFound)
		return
	}
	dir := filepath.Join(".", "ClientSettings", user.AccountID)
	os.MkdirAll(dir, 0755)

	p := filepath.Join(dir, "ClientSettings.Sav")
	if info, err := os.Stat(p); err == nil {
		b, _ := ioutil.ReadFile(p)
		h1 := sha1.Sum(b)
		h2 := sha256.Sum256(b)
		c.JSON(200, []gin.H{{
			"uniqueFilename": "ClientSettings.Sav",
			"filename":       "ClientSettings.Sav",
			"hash":           hex.EncodeToString(h1[:]),
			"hash256":        hex.EncodeToString(h2[:]),
			"length":         len(b),
			"contentType":    "application/octet-stream",
			"uploaded":       info.ModTime(),
			"storageType":    "S3",
			"storageIds":     gin.H{},
			"accountId":      user.AccountID,
			"doNotCache":     false,
		}})
	} else {
		c.JSON(200, []gin.H{})
	}
}

func saveUserFile(c *gin.Context) {
	accountId := c.Param("accountId")
	user, err := utils.FindUserByAccountID(accountId)
	if err != nil || user == nil {
		utils.CreateError(c,
			"errors.com.epicgames.account.account_not_found",
			"Account not found",
			nil, 18007, "account_not_found", http.StatusNotFound)
		return
	}
	buf, _ := ioutil.ReadAll(c.Request.Body)
	if len(buf) >= 400000 {
		c.JSON(403, gin.H{"error": "File size must be less than 400kb."})
		return
	}

	dir := filepath.Join(".", "ClientSettings", user.AccountID)
	os.MkdirAll(dir, 0755)
	p := filepath.Join(dir, "ClientSettings.Sav")
	ioutil.WriteFile(p, buf, 0644)
	c.Status(204)
}

func limitBodySize(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}
