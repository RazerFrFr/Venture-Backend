package admin

import (
	"context"

	registry "VentureBackend/bot/commands"
	"VentureBackend/utils"

	"github.com/bwmarrin/discordgo"
)

var perms = false

type UnbanCommand struct{}

func (UnbanCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "unban",
		Description: "Unban a user from the game (admin only so go kys fag).",
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

func (UnbanCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
			Content: "The account username you entered does not exist.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	if !targetUser.Banned {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "This account is already unbanned.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	_, err = utils.UserCollection.UpdateOne(context.Background(),
		map[string]interface{}{"accountId": targetUser.AccountID},
		map[string]interface{}{"$set": map[string]interface{}{"banned": false}},
	)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to unban the user due to a database error.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Successfully unbanned **" + targetUser.Username + "**",
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

var _ registry.Command = (*UnbanCommand)(nil)
