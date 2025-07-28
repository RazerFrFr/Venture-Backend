package user

import (
	registry "VentureBackend/bot/commands"
	"VentureBackend/utils"
	"VentureBackend/ws/xmpp"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
)

type DetailsCommand struct{}

func (DetailsCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "details",
		Description: "Retrieves your account info.",
	}
}

func (DetailsCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return
	}

	user, err := utils.FindUserByDiscordId(i.Member.User.ID)
	if err != nil || user == nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "You do not have a registered account!",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	profile, err := utils.FindProfileByAccountID(user.AccountID)
	if err != nil || profile == nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to fetch your profile information.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	currency := "0"

	profilesMap := profile.Profiles
	commonCore, ok := profilesMap["common_core"].(map[string]interface{})
	if ok {
		items, ok := commonCore["items"].(map[string]interface{})
		if ok {
			currencyItem, ok := items["Currency:MtxPurchased"].(map[string]interface{})
			if ok {
				quantityVal, ok := currencyItem["quantity"]
				if ok {
					switch v := quantityVal.(type) {
					case int32:
						currency = strconv.Itoa(int(v))
					case int64:
						currency = strconv.Itoa(int(v))
					case float64:
						currency = strconv.Itoa(int(v))
					case int:
						currency = strconv.Itoa(v)
					}
				}
			}
		}
	}

	onlineStatus := false
	for _, client := range xmpp.Clients {
		if client.AccountId == user.AccountID {
			onlineStatus = true
			break
		}
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x00FF00,
		Title: "Account details",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: i.Member.User.AvatarURL("png"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Username:", Value: user.Username, Inline: true},
			{Name: "Email:", Value: user.Email, Inline: true},
			{Name: "Online:", Value: boolToYesNo(onlineStatus), Inline: true},
			{Name: "Banned:", Value: boolToYesNo(user.Banned), Inline: true},
			{Name: "V-Bucks:", Value: currency + " V-Bucks", Inline: true},
			{Name: "Account ID:", Value: user.AccountID, Inline: true},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Flags:  discordgo.MessageFlagsEphemeral,
	})
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	default:
		return "0"
	}
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

var _ registry.Command = (*DetailsCommand)(nil)
