package admin

import (
	"context"

	registry "VentureBackend/bot/commands"
	"VentureBackend/static/tokens"
	"VentureBackend/utils"
	"VentureBackend/ws/xmpp"

	"github.com/bwmarrin/discordgo"
)

type BanCommand struct{}

func (BanCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "ban",
		Description: "Bans a user from the game (admin only so go kys fag).",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "username",
				Description: "Target username.",
				Required:    true,
			},
		},
		DefaultPermission: &perms,
	}
}

func (BanCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})

	username := i.ApplicationCommandData().Options[0].StringValue()
	if username == "" {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid username.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	targetUser, err := utils.FindUserByUsername(username)
	if err != nil || targetUser == nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "User not found.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	if targetUser.Banned {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "This account is already banned.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	_, err = utils.UserCollection.UpdateOne(context.Background(),
		map[string]interface{}{"accountId": targetUser.AccountID},
		map[string]interface{}{"$set": map[string]interface{}{"banned": true}},
	)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to ban the user due to a database error.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	tokenList := tokens.GetTokensByAccountID(targetUser.AccountID)
	go tokens.RemoveTokens(tokenList)

	for _, client := range xmpp.Clients {
		if client.AccountId == targetUser.AccountID {
			client.Conn.Close()
			break
		}
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Successfully banned **" + targetUser.Username + "**",
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

var _ registry.Command = (*BanCommand)(nil)
