package routes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
)

func AddContentPagesRoutes(r *gin.Engine) {
	r.GET("/content/api/pages/:any", handleContentPages)
	r.POST("/api/v1/fortnite-br/surfaces/motd/target", handleMotdTarget)
}

func handleContentPages(c *gin.Context) {
	contentPages := getContentPages(c.Request)
	c.JSON(http.StatusOK, contentPages)
}

func handleMotdTarget(c *gin.Context) {
	data, err := ioutil.ReadFile(filepath.Join(".", "static", "responses", "motdTarget.json"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read motdTarget.json"})
		return
	}

	var motd map[string]interface{}
	if err := json.Unmarshal(data, &motd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse motdTarget.json"})
		return
	}

	var body struct {
		Language string `json:"language"`
	}
	if err := c.BindJSON(&body); err != nil {
		body.Language = "en"
	}

	if items, ok := motd["contentItems"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				fields, ok := m["contentFields"].(map[string]interface{})
				if ok {
					if titleMap, ok := fields["title"].(map[string]interface{}); ok {
						fields["title"] = titleMap[body.Language]
					}
					if bodyMap, ok := fields["body"].(map[string]interface{}); ok {
						fields["body"] = bodyMap[body.Language]
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, motd)
}

func getContentPages(req *http.Request) map[string]interface{} {
	memory := utils.GetVersionInfo(req)

	data, _ := ioutil.ReadFile(filepath.Join(".", "static", "responses", "contentpages.json"))
	var content map[string]interface{}
	json.Unmarshal(data, &content)

	lang := "en"
	langHeader := req.Header.Get("Accept-Language")
	if langHeader != "" {
		if strings.Contains(langHeader, "-") && langHeader != "es-419" {
			lang = strings.Split(langHeader, "-")[0]
		} else {
			lang = langHeader
		}
	}

	modes := []string{"saveTheWorldUnowned", "battleRoyale", "creative", "saveTheWorld"}
	for _, mode := range modes {
		if d, ok := content["subgameselectdata"].(map[string]interface{})[mode].(map[string]interface{}); ok {
			if msg, ok := d["message"].(map[string]interface{}); ok {
				if title, ok := msg["title"].(map[string]interface{}); ok {
					msg["title"] = title[lang]
				}
				if body, ok := msg["body"].(map[string]interface{}); ok {
					msg["body"] = body[lang]
				}
			}
		}
	}

	if memory.Build < 5.30 {
		news := []string{"savetheworldnews", "battleroyalenews"}
		for _, mode := range news {
			if section, ok := content[mode].(map[string]interface{}); ok {
				if newsData, ok := section["news"].(map[string]interface{}); ok {
					if messages, ok := newsData["messages"].([]interface{}); ok && len(messages) >= 2 {
						if m1, ok := messages[0].(map[string]interface{}); ok {
							m1["image"] = "https://cdn.discordapp.com/attachments/927739901540188200/930879507496308736/discord.png"
						}
						if m2, ok := messages[1].(map[string]interface{}); ok {
							m2["image"] = "https://i.imgur.com/ImIwpRm.png"
						}
					}
				}
			}
		}
	}

	backgrounds := content["dynamicbackgrounds"].(map[string]interface{})["backgrounds"].(map[string]interface{})["backgrounds"].([]interface{})
	if len(backgrounds) >= 2 {
		stage := "season" + strconv.Itoa(memory.Season)
		backgrounds[0].(map[string]interface{})["stage"] = stage
		backgrounds[1].(map[string]interface{})["stage"] = stage

		switch memory.Season {
		case 10:
			backgrounds[0].(map[string]interface{})["stage"] = "seasonx"
			backgrounds[1].(map[string]interface{})["stage"] = "seasonx"
		case 11:
			if memory.Build == 11.31 || memory.Build == 11.40 {
				backgrounds[0].(map[string]interface{})["stage"] = "Winter19"
				backgrounds[1].(map[string]interface{})["stage"] = "Winter19"
			}
		case 19:
			if memory.Build == 19.01 {
				backgrounds[0].(map[string]interface{})["stage"] = "winter2021"
				backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn.discordapp.com/attachments/927739901540188200/930880158167085116/t-bp19-lobby-xmas-2048x1024-f85d2684b4af.png"
				content["subgameinfo"].(map[string]interface{})["battleroyale"].(map[string]interface{})["image"] = "https://cdn.discordapp.com/attachments/927739901540188200/930880421514846268/19br-wf-subgame-select-512x1024-16d8bb0f218f.jpg"
				content["specialoffervideo"].(map[string]interface{})["bSpecialOfferEnabled"] = "true"
			}
		case 20:
			if memory.Build == 20.40 {
				backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-bp20-40-armadillo-glowup-lobby-2048x2048-2048x2048-3b83b887cc7f.jpg"
			} else {
				backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-bp20-lobby-2048x1024-d89eb522746c.png"
			}
		case 21:
			backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/s21-lobby-background-2048x1024-2e7112b25dc3.jpg"
			if memory.Build == 21.10 {
				backgrounds[0].(map[string]interface{})["stage"] = "season2100"
			} else if memory.Build == 21.30 {
				backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/nss-lobbybackground-2048x1024-f74a14565061.jpg"
				backgrounds[0].(map[string]interface{})["stage"] = "season2130"
			}
		case 22:
			backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-bp22-lobby-square-2048x2048-2048x2048-e4e90c6e8018.jpg"
		case 23:
			if memory.Build == 23.10 {
				backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-bp23-winterfest-lobby-square-2048x2048-2048x2048-277a476e5ca6.png"
				content["specialoffervideo"].(map[string]interface{})["bSpecialOfferEnabled"] = "true"
			} else {
				backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-bp20-lobby-2048x1024-d89eb522746c.png"
			}
		case 24:
			backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-ch4s2-bp-lobby-4096x2048-edde08d15f7e.jpg"
		case 25:
			backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/fn-shop-ch4s3-04-1920x1080-785ce1d90213.png"
			if memory.Build == 25.11 {
				backgrounds[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-s25-14dos-lobby-4096x2048-2be24969eee3.jpg"
			}
		case 27:
			backgrounds[0].(map[string]interface{})["stage"] = "rufus"
		}
	}

	return content
}
