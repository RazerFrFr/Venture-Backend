package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	registry "VentureBackend/bot/commands"
	"VentureBackend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/bwmarrin/discordgo"
)

type FullLockerCommand struct{}

func (FullLockerCommand) Data() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "fulllocker",
		Description: "Gives you full locker (admin only so kys fag)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "The user you want to grant full locker to",
				Required:    true,
			},
		},
	}
}

func (FullLockerCommand) Execute(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	targetUserID := targetUser.ID

	user, err := utils.FindUserByDiscordId(targetUserID)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "An error occurred while fetching user from database.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}
	if user == nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "That user does not own an account.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	profile, err := utils.FindProfileByAccountID(user.AccountID)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "An error occurred while fetching user profile.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}
	if profile == nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "That user does not have a profile.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	filePath := filepath.Join(".", "static", "profiles", "allathena.json")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to read allathena.json.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	var allItems struct {
		Items map[string]interface{} `json:"items"`
	}

	if err := json.Unmarshal(data, &allItems); err != nil {
		fmt.Println("JSON unmarshal error:", err)
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to parse allathena.json.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	filter := bson.M{"accountId": user.AccountID}
	update := bson.M{"$set": bson.M{"profiles.athena.items": allItems.Items}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	err = utils.ProfileCollection.FindOneAndUpdate(context.Background(), filter, update, opts).Err()
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "There was an error updating the profile.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Successfully gave full locker to the user",
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

var _ registry.Command = (*FullLockerCommand)(nil)
