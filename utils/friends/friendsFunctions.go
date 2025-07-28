package friends

import (
	"VentureBackend/static/models"
	"VentureBackend/utils"
	"VentureBackend/ws/xmpp"
	"time"
)

func ValidateFriendAdd(accountId, friendId string) (bool, error) {
	sender, err := utils.FindFriendByAccountID(accountId)
	if err != nil || sender == nil {
		return false, err
	}

	receiver, err := utils.FindFriendByAccountID(friendId)
	if err != nil || receiver == nil {
		return false, err
	}

	if ContainsAccountID(sender.List.Accepted, receiver.AccountID) || ContainsAccountID(receiver.List.Accepted, sender.AccountID) {
		return false, nil
	}
	if ContainsAccountID(sender.List.Blocked, receiver.AccountID) || ContainsAccountID(receiver.List.Blocked, sender.AccountID) {
		return false, nil
	}
	if sender.AccountID == receiver.AccountID {
		return false, nil
	}

	return true, nil
}

func ValidateFriendDelete(accountId, friendId string) (bool, error) {
	sender, err := utils.FindFriendByAccountID(accountId)
	if err != nil || sender == nil {
		return false, err
	}
	receiver, err := utils.FindFriendByAccountID(friendId)
	if err != nil || receiver == nil {
		return false, err
	}
	return true, nil
}

func ValidateFriendBlock(accountId, friendId string) (bool, error) {
	sender, err := utils.FindFriendByAccountID(accountId)
	if err != nil || sender == nil {
		return false, err
	}
	receiver, err := utils.FindFriendByAccountID(friendId)
	if err != nil || receiver == nil {
		return false, err
	}
	if ContainsAccountID(sender.List.Blocked, receiver.AccountID) {
		return false, nil
	}
	if sender.AccountID == receiver.AccountID {
		return false, nil
	}
	return true, nil
}

func SendFriendReq(fromId, toId string) (bool, error) {
	ok, err := ValidateFriendAdd(fromId, toId)
	if err != nil || !ok {
		return false, err
	}

	from, err := utils.FindFriendByAccountID(fromId)
	if err != nil {
		return false, err
	}
	to, err := utils.FindFriendByAccountID(toId)
	if err != nil {
		return false, err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	from.List.Outgoing = append(from.List.Outgoing, models.FriendEntry{
		AccountID: to.AccountID,
		Created:   now,
	})

	xmpp.SendXmppMessageToId(map[string]interface{}{
		"payload": map[string]interface{}{
			"accountId": to.AccountID,
			"status":    "PENDING",
			"direction": "OUTBOUND",
			"created":   now,
			"favorite":  false,
		},
		"type":      "com.epicgames.friends.core.apiobjects.Friend",
		"timestamp": now,
	}, from.AccountID)

	to.List.Incoming = append(to.List.Incoming, models.FriendEntry{
		AccountID: from.AccountID,
		Created:   now,
	})

	xmpp.SendXmppMessageToId(map[string]interface{}{
		"payload": map[string]interface{}{
			"accountId": from.AccountID,
			"status":    "PENDING",
			"direction": "INBOUND",
			"created":   now,
			"favorite":  false,
		},
		"type":      "com.epicgames.friends.core.apiobjects.Friend",
		"timestamp": now,
	}, to.AccountID)

	if err := utils.UpdateFriendList(from); err != nil {
		return false, err
	}
	if err := utils.UpdateFriendList(to); err != nil {
		return false, err
	}

	return true, nil
}

func AcceptFriendReq(fromId, toId string) (bool, error) {
	ok, err := ValidateFriendAdd(fromId, toId)
	if err != nil || !ok {
		return false, err
	}

	from, err := utils.FindFriendByAccountID(fromId)
	if err != nil {
		return false, err
	}
	to, err := utils.FindFriendByAccountID(toId)
	if err != nil {
		return false, err
	}

	incomingIndex := indexOfAccountID(from.List.Incoming, to.AccountID)
	if incomingIndex != -1 {
		from.List.Incoming = append(from.List.Incoming[:incomingIndex], from.List.Incoming[incomingIndex+1:]...)
		now := time.Now().UTC().Format(time.RFC3339)
		from.List.Accepted = append(from.List.Accepted, models.FriendEntry{
			AccountID: to.AccountID,
			Created:   now,
		})

		xmpp.SendXmppMessageToId(map[string]interface{}{
			"payload": map[string]interface{}{
				"accountId": to.AccountID,
				"status":    "ACCEPTED",
				"direction": "OUTBOUND",
				"created":   now,
				"favorite":  false,
			},
			"type":      "com.epicgames.friends.core.apiobjects.Friend",
			"timestamp": now,
		}, from.AccountID)

		outgoingIndex := indexOfAccountID(to.List.Outgoing, from.AccountID)
		if outgoingIndex != -1 {
			to.List.Outgoing = append(to.List.Outgoing[:outgoingIndex], to.List.Outgoing[outgoingIndex+1:]...)
		}

		to.List.Accepted = append(to.List.Accepted, models.FriendEntry{
			AccountID: from.AccountID,
			Created:   now,
		})

		xmpp.SendXmppMessageToId(map[string]interface{}{
			"payload": map[string]interface{}{
				"accountId": from.AccountID,
				"status":    "ACCEPTED",
				"direction": "OUTBOUND",
				"created":   now,
				"favorite":  false,
			},
			"type":      "com.epicgames.friends.core.apiobjects.Friend",
			"timestamp": now,
		}, to.AccountID)

		if err := utils.UpdateFriendList(from); err != nil {
			return false, err
		}
		if err := utils.UpdateFriendList(to); err != nil {
			return false, err
		}
	}

	return true, nil
}

func DeleteFriend(fromId, toId string) (bool, error) {
	ok, err := ValidateFriendDelete(fromId, toId)
	if err != nil || !ok {
		return false, err
	}

	from, err := utils.FindFriendByAccountID(fromId)
	if err != nil {
		return false, err
	}
	to, err := utils.FindFriendByAccountID(toId)
	if err != nil {
		return false, err
	}

	removed := false

	for _, listType := range []string{"accepted", "incoming", "outgoing", "blocked"} {
		findFriendIdx := indexOfAccountID(*getListByName(&from.List, listType), to.AccountID)
		findToFriendIdx := indexOfAccountID(*getListByName(&to.List, listType), from.AccountID)

		if findFriendIdx != -1 {
			removeFromListByIndex(getListByName(&from.List, listType), findFriendIdx)
			removed = true
		}
		if listType == "blocked" {
			continue
		}
		if findToFriendIdx != -1 {
			removeFromListByIndex(getListByName(&to.List, listType), findToFriendIdx)
		}
	}

	if removed {
		now := time.Now().UTC().Format(time.RFC3339)
		xmpp.SendXmppMessageToId(map[string]interface{}{
			"payload": map[string]interface{}{
				"accountId": to.AccountID,
				"reason":    "DELETED",
			},
			"type":      "com.epicgames.friends.core.apiobjects.FriendRemoval",
			"timestamp": now,
		}, from.AccountID)

		xmpp.SendXmppMessageToId(map[string]interface{}{
			"payload": map[string]interface{}{
				"accountId": from.AccountID,
				"reason":    "DELETED",
			},
			"type":      "com.epicgames.friends.core.apiobjects.FriendRemoval",
			"timestamp": now,
		}, to.AccountID)

		if err := utils.UpdateFriendList(from); err != nil {
			return false, err
		}
		if err := utils.UpdateFriendList(to); err != nil {
			return false, err
		}
	}

	return true, nil
}

func BlockFriend(fromId, toId string) (bool, error) {
	ok, err := ValidateFriendDelete(fromId, toId)
	if err != nil || !ok {
		return false, err
	}
	ok, err = ValidateFriendBlock(fromId, toId)
	if err != nil || !ok {
		return false, err
	}

	_, err = DeleteFriend(fromId, toId)
	if err != nil {
		return false, err
	}

	from, err := utils.FindFriendByAccountID(fromId)
	if err != nil {
		return false, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	from.List.Blocked = append(from.List.Blocked, models.FriendEntry{
		AccountID: toId,
		Created:   now,
	})

	if err := utils.UpdateFriendList(from); err != nil {
		return false, err
	}

	return true, nil
}

func ContainsAccountID(list []models.FriendEntry, accountID string) bool {
	for _, e := range list {
		if e.AccountID == accountID {
			return true
		}
	}
	return false
}

func indexOfAccountID(list []models.FriendEntry, accountID string) int {
	for i, e := range list {
		if e.AccountID == accountID {
			return i
		}
	}
	return -1
}

func getListByName(list *models.FriendList, name string) *[]models.FriendEntry {
	switch name {
	case "accepted":
		return &list.Accepted
	case "incoming":
		return &list.Incoming
	case "outgoing":
		return &list.Outgoing
	case "blocked":
		return &list.Blocked
	default:
		return nil
	}
}

func removeFromListByIndex(list *[]models.FriendEntry, index int) {
	*list = append((*list)[:index], (*list)[index+1:]...)
}
