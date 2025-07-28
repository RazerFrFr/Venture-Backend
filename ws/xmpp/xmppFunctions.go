package xmpp

import (
	"VentureBackend/utils"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/gorilla/websocket"
)

func Error(ws *websocket.Conn) {
	doc := etree.NewDocument()
	closeTag := doc.CreateElement("close")
	closeTag.CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-framing")
	xmlStr, _ := doc.WriteToString()
	ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))
	ws.Close()
}

func RemoveClient(ws *websocket.Conn, joinedMUCs []string) {
	var removedClient *Client

	ClientsMux.Lock()
	clientIndex := -1
	for i, c := range Clients {
		if c.Conn == ws {
			clientIndex = i
			break
		}
	}

	if clientIndex != -1 {
		removedClient = Clients[clientIndex]
		Clients = append(Clients[:clientIndex], Clients[clientIndex+1:]...)
	}
	ClientsMux.Unlock()

	if removedClient == nil {
		return
	}

	var clientStatus struct {
		Properties map[string]interface{} `json:"Properties"`
	}
	_ = json.Unmarshal([]byte(removedClient.LastPresence.Status), &clientStatus)

	updatePresenceForFriends(ws, "{}", false, true)

	for _, roomName := range joinedMUCs {
		if muc, ok := MUCs[roomName]; ok {
			for i, member := range muc.Members {
				if member.AccountId == removedClient.AccountId {
					muc.Members = append(muc.Members[:i], muc.Members[i+1:]...)
					break
				}
			}
		}
	}

	var partyId string
	for k, v := range clientStatus.Properties {
		if strings.HasPrefix(strings.ToLower(k), "party.joininfo") {
			if propMap, ok := v.(map[string]interface{}); ok {
				if pid, ok := propMap["partyId"].(string); ok {
					partyId = pid
				}
			}
		}
	}

	if partyId != "" {
		ClientsMux.Lock()
		snapshot := append([]*Client(nil), Clients...)
		ClientsMux.Unlock()

		for _, c := range snapshot {
			if c.AccountId == removedClient.AccountId {
				continue
			}

			doc := etree.NewDocument()
			msg := doc.CreateElement("message")
			msg.CreateAttr("id", strings.ToUpper(MakeID()))
			msg.CreateAttr("from", removedClient.Jid)
			msg.CreateAttr("xmlns", "jabber:client")
			msg.CreateAttr("to", c.Jid)

			body := msg.CreateElement("body")
			payload := map[string]interface{}{
				"type": "com.epicgames.party.memberexited",
				"payload": map[string]interface{}{
					"partyId":   partyId,
					"memberId":  removedClient.AccountId,
					"wasKicked": false,
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			body.SetText(mustJSON(payload))

			xmlStr, _ := doc.WriteToString()
			_ = c.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr))
		}
	}

	utils.XMPP.Logf("Client %s logged out", removedClient.DisplayName)
}

func updatePresenceForFriends(ws *websocket.Conn, body string, away bool, offline bool) {
	ClientsMux.Lock()
	defer ClientsMux.Unlock()

	var sender *Client
	for _, c := range Clients {
		if c.Conn == ws {
			sender = c
			break
		}
	}
	if sender == nil {
		return
	}

	sender.LastPresence.Away = away
	sender.LastPresence.Status = body

	for _, friend := range Clients {
		if friend.AccountId == sender.AccountId {
			continue
		}

		doc := etree.NewDocument()
		presence := doc.CreateElement("presence")
		presence.CreateAttr("to", friend.Jid)
		presence.CreateAttr("xmlns", "jabber:client")
		presence.CreateAttr("from", sender.Jid)
		if offline {
			presence.CreateAttr("type", "unavailable")
		}

		if away {
			presence.CreateElement("show").SetText("away")
		}
		presence.CreateElement("status").SetText(body)

		xmlStr, _ := doc.WriteToString()
		friend.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr))
	}
}

func sendXmppMessageToClient(senderJid string, msg *etree.Document, body interface{}) {
	strBody := fmt.Sprintf("%v", body)
	if b, ok := body.(string); ok {
		strBody = b
	} else {
		strBody = mustJSON(body)
	}

	to := msg.Root().SelectAttrValue("to", "")
	var receiver *Client
	for _, c := range Clients {
		if strings.Split(c.Jid, "/")[0] == to || c.Jid == to {
			receiver = c
			break
		}
	}
	if receiver == nil {
		return
	}

	doc := etree.NewDocument()
	message := doc.CreateElement("message")
	message.CreateAttr("from", senderJid)
	message.CreateAttr("id", msg.Root().SelectAttrValue("id", ""))
	message.CreateAttr("to", receiver.Jid)
	message.CreateAttr("xmlns", "jabber:client")
	message.CreateElement("body").SetText(strBody)

	xmlStr, _ := doc.WriteToString()
	receiver.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr))
}

func getPresenceFromUser(fromId, toId string, offline bool) {
	ClientsMux.Lock()
	defer ClientsMux.Unlock()

	var sender, receiver *Client
	for _, c := range Clients {
		if c.AccountId == fromId {
			sender = c
		}
		if c.AccountId == toId {
			receiver = c
		}
	}
	if sender == nil || receiver == nil {
		return
	}

	doc := etree.NewDocument()
	presence := doc.CreateElement("presence")
	presence.CreateAttr("to", receiver.Jid)
	presence.CreateAttr("xmlns", "jabber:client")
	presence.CreateAttr("from", sender.Jid)
	if offline {
		presence.CreateAttr("type", "unavailable")
	}

	if sender.LastPresence.Away {
		presence.CreateElement("show").SetText("away")
	}
	presence.CreateElement("status").SetText(sender.LastPresence.Status)

	xmlStr, _ := doc.WriteToString()
	receiver.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr))
}

func getMUCmember(roomName, displayName, accountId, resource string) string {
	return fmt.Sprintf("%s@muc.%s/%s:%s:%s", roomName, XmppDomain, displayName, accountId, resource)
}

func MakeID() string {
	return strings.ReplaceAll(strings.ToUpper(fmt.Sprintf("%x", time.Now().UnixNano())), "-", "")
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func isJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func ParseXML(raw string) (*ParsedXML, error) {
	doc := etree.NewDocument()
	err := doc.ReadFromString(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	root := doc.Root()
	if root == nil {
		return nil, errors.New("no root element found in XML")
	}
	return &ParsedXML{
		Doc:  doc,
		Root: root,
	}, nil
}

func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func findClientByAccountID(accountId string) *Client {
	for _, c := range Clients {
		if c.AccountId == accountId {
			return c
		}
	}
	return nil
}

func SendXmppMessageToId(body interface{}, toAccountId string) {
	if len(Clients) == 0 {
		return
	}

	var bodyStr string
	switch v := body.(type) {
	case string:
		bodyStr = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		bodyStr = string(b)
	}

	var receiver *Client
	ClientsMux.Lock()
	for _, c := range Clients {
		if c.AccountId == toAccountId {
			receiver = c
			break
		}
	}
	ClientsMux.Unlock()

	if receiver == nil {
		return
	}

	doc := etree.NewDocument()
	message := doc.CreateElement("message")
	message.CreateAttr("from", "xmpp-admin@"+XmppDomain)
	message.CreateAttr("to", receiver.Jid)
	message.CreateAttr("xmlns", "jabber:client")
	message.CreateElement("body").SetText(bodyStr)

	xmlStr, err := doc.WriteToString()
	if err != nil {
		return
	}

	receiver.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr))
}
