package user

import (
	registry "VentureBackend/bot/commands"
	"VentureBackend/utils"
	"math/rand"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
)

type RegisterCommand struct{}

func (RegisterCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "register",
		Description: "Creates an account for you",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "username",
				Description: "Username for the new account",
				Required:    true,
			},
		},
	}
}

func (RegisterCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	discordId := i.Member.User.ID

	username := ""
	if opt := i.ApplicationCommandData().Options; len(opt) > 0 {
		username = opt[0].StringValue()
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	response := "Registration failed"

	user, err := utils.FindUserByDiscordId(discordId)
	if err != nil {
		response = "An error occurred while fetching user from DB"
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}
	if user != nil {
		response = "You already own an account"
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	rand.Seed(time.Now().UnixNano())
	randomEmail := strconv.Itoa(100000+rand.Intn(900000)) + "@razerhosting.xyz"
	randomPassword := generateRandomPassword(12)

	success := utils.RegisterUser(&discordId, username, randomEmail, randomPassword, false)
	if success {
		response = "Account registered!"
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: response,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

var _ registry.Command = (*RegisterCommand)(nil)
