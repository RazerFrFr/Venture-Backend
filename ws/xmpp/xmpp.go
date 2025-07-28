package xmpp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"VentureBackend/static/tokens"
	"VentureBackend/utils"

	"github.com/beevik/etree"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Client struct {
	Conn         *websocket.Conn
	AccountId    string
	DisplayName  string
	Token        string
	Jid          string
	Resource     string
	LastPresence struct {
		Away   bool
		Status string
	}
}

type MUC struct {
	Members []MUCMember
}

type MUCMember struct {
	AccountId string
}

var (
	Clients    []*Client
	ClientsMux sync.Mutex
	MUCs       = make(map[string]*MUC)
	XmppDomain = "prod.ol.epicgames.com"
	upgrader   = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type ParsedXML struct {
	Doc  *etree.Document
	Root *etree.Element
}

func InitXMPP() {
	port := os.Getenv("XMPP_PORT")
	if port == "" {
		port = "5000"
	}

	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		ClientsMux.Lock()
		clientNames := []string{}
		for _, cl := range Clients {
			clientNames = append(clientNames, cl.DisplayName)
		}
		ClientsMux.Unlock()

		data := map[string]interface{}{
			"Clients": map[string]interface{}{
				"amount":  len(clientNames),
				"clients": clientNames,
			},
		}
		c.JSON(200, data)
	})

	r.GET("/clients", func(c *gin.Context) {
		ClientsMux.Lock()
		clientNames := []string{}
		for _, cl := range Clients {
			clientNames = append(clientNames, cl.DisplayName)
		}
		ClientsMux.Unlock()

		data := map[string]interface{}{
			"amount":  len(clientNames),
			"clients": clientNames,
		}
		c.JSON(200, data)
	})

	server := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if websocket.IsWebSocketUpgrade(req) {
				xmppHandler(w, req)
				return
			}
			r.ServeHTTP(w, req)
		}),
	}

	utils.XMPP.Log("Listening on port :" + port)
	server.ListenAndServe()
}

func xmppHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	var joinedMUCs []string
	var accountId, displayName, token, jid, resource, ID string
	var Authenticated, clientExists, connectionClosed bool

	ws.SetCloseHandler(func(code int, text string) error {
		connectionClosed = true
		clientExists = false
		RemoveClient(ws, joinedMUCs)
		return nil
	})

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}

		msg := strings.TrimSpace(string(message))
		if msg == "" {
			Error(ws)
			continue
		}

		parsed, err := ParseXML(msg)
		if err != nil || parsed.Root == nil {
			Error(ws)
			continue
		}

		switch parsed.Root.Tag {
		case "open":
			handleOpen(ws, &ID, &Authenticated)
		case "auth":
			handleAuth(ws, parsed.Doc, &accountId, &displayName, &token, &Authenticated)
		case "iq":
			handleIQ(ws, parsed.Doc, &accountId, &resource, &jid, clientExists)
		case "message":
			handleMessage(ws, parsed.Doc, accountId, jid, displayName, resource, clientExists)
		case "presence":
			handlePresence(ws, parsed.Doc, accountId, displayName, jid, resource, &joinedMUCs, clientExists)
		}

		if !clientExists && !connectionClosed {
			if accountId != "" && displayName != "" && token != "" && jid != "" && ID != "" && resource != "" && Authenticated {
				ClientsMux.Lock()
				Clients = append(Clients, &Client{
					Conn:        ws,
					AccountId:   accountId,
					DisplayName: displayName,
					Token:       token,
					Jid:         jid,
					Resource:    resource,
					LastPresence: struct {
						Away   bool
						Status string
					}{Away: false, Status: "{}"},
				})
				ClientsMux.Unlock()

				utils.XMPP.Logf("New client %s connected", displayName)
				clientExists = true
			}
		}
	}
}

func handleOpen(ws *websocket.Conn, ID *string, Authenticated *bool) {
	if *ID == "" {
		*ID = MakeID()
	}

	doc := etree.NewDocument()
	openElem := doc.CreateElement("open")
	openElem.CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-framing")
	openElem.CreateAttr("from", XmppDomain)
	openElem.CreateAttr("id", *ID)
	openElem.CreateAttr("version", "1.0")
	openElem.CreateAttr("xml:lang", "en")

	xmlStr, _ := doc.WriteToString()
	ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))

	featuresDoc := etree.NewDocument()
	features := featuresDoc.CreateElement("stream:features")
	features.CreateAttr("xmlns:stream", "http://etherx.jabber.org/streams")

	if *Authenticated {
		features.CreateElement("ver").CreateAttr("xmlns", "urn:xmpp:features:rosterver")

		features.CreateElement("starttls").CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-tls")

		features.CreateElement("bind").CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-bind")

		compression := features.CreateElement("compression")
		compression.CreateAttr("xmlns", "http://jabber.org/features/compress")
		compression.CreateElement("method").SetText("zlib")

		features.CreateElement("session").CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-session")
	} else {
		mechanisms := features.CreateElement("mechanisms")
		mechanisms.CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-sasl")
		mechanisms.CreateElement("mechanism").SetText("PLAIN")

		features.CreateElement("ver").CreateAttr("xmlns", "urn:xmpp:features:rosterver")

		features.CreateElement("starttls").CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-tls")

		compression := features.CreateElement("compression")
		compression.CreateAttr("xmlns", "http://jabber.org/features/compress")
		compression.CreateElement("method").SetText("zlib")

		features.CreateElement("auth").CreateAttr("xmlns", "http://jabber.org/features/iq-auth")
	}

	featuresXML, _ := featuresDoc.WriteToString()
	ws.WriteMessage(websocket.TextMessage, []byte(featuresXML))
}

func handleAuth(ws *websocket.Conn, parsed *etree.Document, accountId, displayName, token *string, Authenticated *bool) {
	if *accountId != "" {
		return
	}

	root := parsed.Root()
	if root == nil {
		Error(ws)
		return
	}

	content := root.Text()
	if content == "" {
		Error(ws)
		return
	}

	decodedBytes, err := DecodeBase64(content)
	if err != nil || !strings.Contains(string(decodedBytes), "\x00") {
		Error(ws)
		return
	}

	parts := strings.Split(string(decodedBytes), "\x00")
	if len(parts) != 3 {
		Error(ws)
		return
	}

	tokenStr := parts[2]

	object := tokens.FindAccessToken(tokenStr)
	if object == nil {
		Error(ws)
		return
	}

	ClientsMux.Lock()
	for _, c := range Clients {
		if c.AccountId == object.AccountID {
			ClientsMux.Unlock()
			Error(ws)
			return
		}
	}
	ClientsMux.Unlock()

	user, err := utils.FindUserByAccountID(object.AccountID)
	if err != nil || user == nil || user.Banned {
		Error(ws)
		return
	}

	*accountId = user.AccountID
	*displayName = user.Username
	*token = object.Token
	*Authenticated = true

	utils.XMPP.Logf("A new client with the username %s has authentificated.", *displayName)

	doc := etree.NewDocument()
	success := doc.CreateElement("success")
	success.CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-sasl")

	xmlStr, _ := doc.WriteToString()
	ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))
}

func handleIQ(ws *websocket.Conn, parsed *etree.Document, accountId, resource, jid *string, clientExists bool) {
	root := parsed.Root()
	if root == nil {
		Error(ws)
		return
	}

	id := root.SelectAttrValue("id", "")
	if id == "" {
		return
	}

	switch id {
	case "_xmpp_bind1":
		if *resource != "" || *accountId == "" {
			return
		}

		bindElem := root.FindElement("bind")
		if bindElem == nil {
			return
		}

		ClientsMux.Lock()
		for _, c := range Clients {
			if c.AccountId == *accountId {
				ClientsMux.Unlock()
				Error(ws)
				return
			}
		}
		ClientsMux.Unlock()

		resourceElem := bindElem.FindElement("resource")
		if resourceElem == nil || resourceElem.Text() == "" {
			return
		}

		*resource = resourceElem.Text()
		*jid = fmt.Sprintf("%s@%s/%s", *accountId, XmppDomain, *resource)

		doc := etree.NewDocument()
		iq := doc.CreateElement("iq")
		iq.CreateAttr("to", *jid)
		iq.CreateAttr("id", "_xmpp_bind1")
		iq.CreateAttr("xmlns", "jabber:client")
		iq.CreateAttr("type", "result")

		bind := iq.CreateElement("bind")
		bind.CreateAttr("xmlns", "urn:ietf:params:xml:ns:xmpp-bind")
		jidElem := bind.CreateElement("jid")
		jidElem.SetText(*jid)

		xmlStr, _ := doc.WriteToString()
		ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))

	case "_xmpp_session1":
		if !clientExists {
			Error(ws)
			return
		}

		doc := etree.NewDocument()
		iq := doc.CreateElement("iq")
		iq.CreateAttr("to", *jid)
		iq.CreateAttr("from", XmppDomain)
		iq.CreateAttr("id", "_xmpp_session1")
		iq.CreateAttr("xmlns", "jabber:client")
		iq.CreateAttr("type", "result")

		xmlStr, _ := doc.WriteToString()
		ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))

		getPresenceFromUser(*accountId, *jid, false)

	default:
		if !clientExists {
			Error(ws)
			return
		}

		doc := etree.NewDocument()
		iq := doc.CreateElement("iq")
		iq.CreateAttr("to", *jid)
		iq.CreateAttr("from", XmppDomain)
		iq.CreateAttr("id", id)
		iq.CreateAttr("xmlns", "jabber:client")
		iq.CreateAttr("type", "result")

		xmlStr, _ := doc.WriteToString()
		ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))
	}
}

func handleMessage(ws *websocket.Conn, msg *etree.Document, accountId, jid, displayName, resource string, clientExists bool) {
	if !clientExists {
		Error(ws)
		return
	}

	root := msg.Root()
	if root == nil {
		Error(ws)
		return
	}

	bodyElem := root.FindElement("body")
	if bodyElem == nil || bodyElem.Text() == "" {
		return
	}

	body := bodyElem.Text()
	msgType := root.SelectAttrValue("type", "")

	switch msgType {
	case "chat":
		toJid := root.SelectAttrValue("to", "")
		if toJid == "" || len(body) >= 300 {
			return
		}

		ClientsMux.Lock()
		var receiver *Client
		for _, c := range Clients {
			if strings.Split(c.Jid, "/")[0] == toJid {
				receiver = c
				break
			}
		}
		ClientsMux.Unlock()

		if receiver == nil || receiver.AccountId == accountId {
			return
		}

		doc := etree.NewDocument()
		message := doc.CreateElement("message")
		message.CreateAttr("to", receiver.Jid)
		message.CreateAttr("from", jid)
		message.CreateAttr("xmlns", "jabber:client")
		message.CreateAttr("type", "chat")
		message.CreateElement("body").SetText(body)

		xmlStr, _ := doc.WriteToString()
		receiver.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr))

	case "groupchat":
		toJid := root.SelectAttrValue("to", "")
		if toJid == "" || len(body) >= 300 {
			return
		}

		roomName := strings.Split(toJid, "@")[0]

		muc, ok := MUCs[roomName]
		if !ok {
			return
		}

		memberFound := false
		for _, m := range muc.Members {
			if m.AccountId == accountId {
				memberFound = true
				break
			}
		}
		if !memberFound {
			return
		}

		for _, member := range muc.Members {
			ClientsMux.Lock()
			clientData := findClientByAccountID(member.AccountId)
			ClientsMux.Unlock()
			if clientData == nil {
				continue
			}

			doc := etree.NewDocument()
			message := doc.CreateElement("message")
			message.CreateAttr("to", clientData.Jid)
			message.CreateAttr("from", getMUCmember(roomName, displayName, accountId, resource))
			message.CreateAttr("xmlns", "jabber:client")
			message.CreateAttr("type", "groupchat")
			message.CreateElement("body").SetText(body)

			xmlStr, _ := doc.WriteToString()
			clientData.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr))
		}

	default:
		if isJSON(body) {
			var bodyJSON map[string]interface{}
			err := json.Unmarshal([]byte(body), &bodyJSON)
			if err != nil {
				return
			}

			var temp interface{}
			json.Unmarshal([]byte(body), &temp)
			if _, isArray := temp.([]interface{}); isArray {
				return
			}

			typeVal, ok := bodyJSON["type"].(string)
			if !ok || typeVal == "" {
				return
			}

			to := root.SelectAttrValue("to", "")
			id := root.SelectAttrValue("id", "")
			if to == "" || id == "" {
				return
			}

			sendXmppMessageToClient(jid, msg, body)
		}
	}
}

func handlePresence(ws *websocket.Conn, msg *etree.Document, accountId, displayName, jid, resource string, joinedMUCs *[]string, clientExists bool) {
	if !clientExists {
		Error(ws)
		return
	}

	root := msg.Root()
	if root == nil {
		Error(ws)
		return
	}

	pType := root.SelectAttrValue("type", "")

	switch pType {
	case "unavailable":
		to := root.SelectAttrValue("to", "")
		if to == "" {
			return
		}

		domainSuffix := "@muc." + XmppDomain
		if strings.HasSuffix(to, domainSuffix) || strings.HasSuffix(strings.Split(to, "/")[0], domainSuffix) {
			if !strings.HasPrefix(strings.ToLower(to), "party-") {
				return
			}

			roomName := strings.Split(to, "@")[0]

			muc, ok := MUCs[roomName]
			if !ok {
				return
			}

			for i, member := range muc.Members {
				if member.AccountId == accountId {
					muc.Members = append(muc.Members[:i], muc.Members[i+1:]...)
					break
				}
			}

			for i, r := range *joinedMUCs {
				if r == roomName {
					*joinedMUCs = append((*joinedMUCs)[:i], (*joinedMUCs)[i+1:]...)
					break
				}
			}

			doc := etree.NewDocument()
			presence := doc.CreateElement("presence")
			presence.CreateAttr("to", jid)
			presence.CreateAttr("from", getMUCmember(roomName, displayName, accountId, resource))
			presence.CreateAttr("xmlns", "jabber:client")
			presence.CreateAttr("type", "unavailable")

			x := presence.CreateElement("x")
			x.CreateAttr("xmlns", "http://jabber.org/protocol/muc#user")

			item := x.CreateElement("item")
			nick := strings.Replace(getMUCmember(roomName, displayName, accountId, resource), roomName+"@muc."+XmppDomain+"/", "", 1)
			item.CreateAttr("nick", nick)
			item.CreateAttr("jid", jid)
			item.CreateAttr("role", "none")

			x.CreateElement("status").CreateAttr("code", "110")
			x.CreateElement("status").CreateAttr("code", "100")
			x.CreateElement("status").CreateAttr("code", "170")

			xmlStr, _ := doc.WriteToString()
			ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))
			return
		}

	default:
		var hasMUCElem, hasXElem bool
		for _, child := range root.ChildElements() {
			if child.Tag == "muc:x" {
				hasMUCElem = true
				break
			}
			if child.Tag == "x" {
				hasXElem = true
			}
		}

		if hasMUCElem || hasXElem {
			to := root.SelectAttrValue("to", "")
			if to == "" {
				return
			}

			roomName := strings.Split(to, "@")[0]

			muc, ok := MUCs[roomName]
			if !ok {
				muc = &MUC{Members: []MUCMember{}}
				MUCs[roomName] = muc
			}

			for _, member := range muc.Members {
				if member.AccountId == accountId {
					return
				}
			}

			muc.Members = append(muc.Members, MUCMember{AccountId: accountId})
			*joinedMUCs = append(*joinedMUCs, roomName)

			doc := etree.NewDocument()
			presence := doc.CreateElement("presence")
			presence.CreateAttr("to", jid)
			presence.CreateAttr("from", getMUCmember(roomName, displayName, accountId, resource))
			presence.CreateAttr("xmlns", "jabber:client")

			x := presence.CreateElement("x")
			x.CreateAttr("xmlns", "http://jabber.org/protocol/muc#user")

			item := x.CreateElement("item")
			nick := strings.Replace(getMUCmember(roomName, displayName, accountId, resource), roomName+"@muc."+XmppDomain+"/", "", 1)
			item.CreateAttr("nick", nick)
			item.CreateAttr("jid", jid)
			item.CreateAttr("role", "participant")
			item.CreateAttr("affiliation", "none")

			x.CreateElement("status").CreateAttr("code", "110")
			x.CreateElement("status").CreateAttr("code", "100")
			x.CreateElement("status").CreateAttr("code", "170")
			x.CreateElement("status").CreateAttr("code", "201")

			xmlStr, _ := doc.WriteToString()
			ws.WriteMessage(websocket.TextMessage, []byte(xmlStr))

			for _, member := range muc.Members {
				clientData := findClientByAccountID(member.AccountId)
				if clientData == nil {
					continue
				}

				doc2 := etree.NewDocument()
				presence2 := doc2.CreateElement("presence")
				presence2.CreateAttr("from", getMUCmember(roomName, clientData.DisplayName, clientData.AccountId, clientData.Resource))
				presence2.CreateAttr("to", jid)
				presence2.CreateAttr("xmlns", "jabber:client")

				x2 := presence2.CreateElement("x")
				x2.CreateAttr("xmlns", "http://jabber.org/protocol/muc#user")

				item2 := x2.CreateElement("item")
				nick2 := strings.Replace(getMUCmember(roomName, clientData.DisplayName, clientData.AccountId, clientData.Resource), roomName+"@muc."+XmppDomain+"/", "", 1)
				item2.CreateAttr("nick", nick2)
				item2.CreateAttr("jid", clientData.Jid)
				item2.CreateAttr("role", "participant")
				item2.CreateAttr("affiliation", "none")

				xmlStr2, _ := doc2.WriteToString()
				ws.WriteMessage(websocket.TextMessage, []byte(xmlStr2))

				if accountId == clientData.AccountId {
					continue
				}

				doc3 := etree.NewDocument()
				presence3 := doc3.CreateElement("presence")
				presence3.CreateAttr("from", getMUCmember(roomName, displayName, accountId, resource))
				presence3.CreateAttr("to", clientData.Jid)
				presence3.CreateAttr("xmlns", "jabber:client")

				x3 := presence3.CreateElement("x")
				x3.CreateAttr("xmlns", "http://jabber.org/protocol/muc#user")

				item3 := x3.CreateElement("item")
				nick3 := strings.Replace(getMUCmember(roomName, displayName, accountId, resource), roomName+"@muc."+XmppDomain+"/", "", 1)
				item3.CreateAttr("nick", nick3)
				item3.CreateAttr("jid", jid)
				item3.CreateAttr("role", "participant")
				item3.CreateAttr("affiliation", "none")

				xmlStr3, _ := doc3.WriteToString()
				clientData.Conn.WriteMessage(websocket.TextMessage, []byte(xmlStr3))
			}

			return
		}
	}

	var findStatus *etree.Element
	var findShow *etree.Element

	for _, child := range root.ChildElements() {
		if child.Tag == "status" {
			findStatus = child
		}
		if child.Tag == "show" {
			findShow = child
		}
	}

	if findStatus == nil || findStatus.Text() == "" {
		return
	}

	statusContent := findStatus.Text()

	if !isJSON(statusContent) {
		return
	}

	var statusCheck interface{}
	json.Unmarshal([]byte(statusContent), &statusCheck)
	if _, isArray := statusCheck.([]interface{}); isArray {
		return
	}

	away := findShow != nil

	updatePresenceForFriends(ws, statusContent, away, false)
	getPresenceFromUser(accountId, accountId, false)
}
