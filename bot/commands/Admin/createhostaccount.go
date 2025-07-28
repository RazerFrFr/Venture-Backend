package admin

import (
	registry "VentureBackend/bot/commands"
	"VentureBackend/utils"
	"math/rand"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
)

type HostAccCommand struct{}

func (HostAccCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:              "createhostaccount",
		Description:       "Creates a host account for you (admin only so go kys fag)",
		DefaultPermission: &perms,
	}
}

func (HostAccCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	response := "Registration failed"

	rand.Seed(time.Now().UnixNano())
	randomUsername := "VentureBackendHostAccount-" + strconv.Itoa(100000+rand.Intn(900000))
	randomEmail := randomUsername + "@VentureBackend.xyz"
	randomPassword := generateRandomPassword(12)

	success := utils.RegisterUser(nil, randomUsername, randomEmail, randomPassword, true)
	if success {
		response = "Account Created!"
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x56ff00,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Message",
				Value: response,
			},
			{
				Name:  "Username",
				Value: "```" + randomUsername + "```",
			},
			{
				Name:  "Email",
				Value: "```" + randomEmail + "```",
			},
			{
				Name:  "Password",
				Value: "```" + randomPassword + "```",
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if success {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		})
	} else {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
	}
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

var _ registry.Command = (*HostAccCommand)(nil)
