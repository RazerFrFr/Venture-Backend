package user

import (
	registry "VentureBackend/bot/commands"
	"VentureBackend/utils"
	"time"

	"github.com/bwmarrin/discordgo"
)

type MobileLoginCommand struct{}

func (MobileLoginCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "mobilelogin",
		Description: "Retrieves your login details to use on Venture Backend mobile",
	}
}

func (MobileLoginCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})

	discordId := i.Member.User.ID

	user, err := utils.FindUserByDiscordId(discordId)
	if err != nil || user == nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "You don't have a registered account.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	details, err := utils.FindMobileByAccountID(user.AccountID)
	if err != nil || details == nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No mobile login details found for your account.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	avatarURL := i.Member.User.AvatarURL("png")

	embed := &discordgo.MessageEmbed{
		Color:     0x56ff00,
		Title:     "Your Mobile Login Details:",
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: avatarURL},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Email",
				Value: "```" + details.Email + "```",
			},
			{
				Name:  "Password",
				Value: "```" + details.Password + "```",
			},
			{
				Name:  "Notice",
				Value: "**DO NOT SHARE THESE DETAILS** or else people will login to your account.",
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Flags:  discordgo.MessageFlagsEphemeral,
	})
}

var _ registry.Command = (*MobileLoginCommand)(nil)
