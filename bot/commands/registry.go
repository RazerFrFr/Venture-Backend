package registry

import "github.com/bwmarrin/discordgo"

type Command interface {
	Data() *discordgo.ApplicationCommand
	Execute(s *discordgo.Session, i *discordgo.InteractionCreate)
}
