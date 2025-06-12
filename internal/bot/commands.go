package bot

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

var commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"ping": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Println("Bot: Recieved /ping command")
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong!",
			},
		})
		if err != nil {
			log.Printf("Bot: Error responding to /ping command: %v", err)
		}
	},
}

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "ping",
		Description: "A simple command to check if the bot is responsive.",
	},
}
