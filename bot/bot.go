package discord

import (
	registry "VentureBackend/bot/commands"
	admin "VentureBackend/bot/commands/Admin"
	user "VentureBackend/bot/commands/User"
	"VentureBackend/utils"
	"os"

	"github.com/bwmarrin/discordgo"
)

var commandList = []registry.Command{
	user.RegisterCommand{},
	user.DetailsCommand{},
	user.MobileLoginCommand{},
	admin.DeleteUserCommand{},
	admin.FullLockerCommand{},
	admin.BanCommand{},
	admin.UnbanCommand{},
	admin.HostAccCommand{},
}

func InitBot() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		utils.Discord.Log("BOT_TOKEN env missing")
		return
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		utils.Discord.Log("Failed to create Discord session:", err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		for _, cmd := range commandList {
			if i.ApplicationCommandData().Name == cmd.Data().Name {
				go cmd.Execute(s, i)
				return
			}
		}
	})

	err = dg.Open()
	if err != nil {
		utils.Discord.Log("Failed to open Discord connection:", err)
		return
	}

	appID := dg.State.User.ID

	/*existing, err := dg.ApplicationCommands(appID, "")
	if err == nil {
		for _, cmd := range existing {
			_ = dg.ApplicationCommandDelete(appID, "", cmd.ID)
		}
	} else {
		utils.Discord.Log("Failed to fetch existing commands:", err)
	}*/

	for _, cmd := range commandList {
		_, err := dg.ApplicationCommandCreate(appID, "", cmd.Data())
		if err != nil {
			utils.Discord.Log("Failed to register command:", cmd.Data().Name, err)
		}
	}

	utils.Discord.Log("Slash commands synced.")
	utils.Discord.Log("Bot is running.")
}
