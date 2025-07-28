package admin

import (
	registry "VentureBackend/bot/commands"
	"VentureBackend/utils"

	"github.com/bwmarrin/discordgo"
)

type DeleteUserCommand struct{}

func (DeleteUserCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "delete",
		Description: "Deletes a user account (admins only btw so go kys u fag)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "The Discord user to delete",
				Required:    true,
			},
		},
	}
}

func (DeleteUserCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userOption := i.ApplicationCommandData().Options[0].UserValue(s)
	if userOption == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Invalid user.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	discordId := userOption.ID
	user, err := utils.FindUserByDiscordId(discordId)
	if err != nil || user == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "User not found in database.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	accountId := user.AccountID
	if accountId == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "User has no accountId associated.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	success := utils.DeleteUser(accountId)
	resp := "Failed to delete user."
	if success {
		resp = "User account deleted successfully."
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: resp,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

var _ registry.Command = (*DeleteUserCommand)(nil)
