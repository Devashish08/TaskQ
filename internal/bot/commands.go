package bot

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

var BotInstance *Bot

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

	"remind-simple": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Println("Bot: Received /remind-simple command")

		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		durationStr := optionMap["duration"].StringValue()
		message := optionMap["message"].StringValue()

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			log.Printf("Bot: Invalid duration format '%s': %v", durationStr, err)
			errorMsg := "Invalid duration format. Please use formats like `30s`, `5m`, or `1h`."
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &errorMsg,
			})
			return
		}

		payload := map[string]interface{}{
			"channel_id": i.ChannelID,
			"user_id":    i.Member.User.ID,
			"message":    message,
		}

		job, err := BotInstance.JobSvc.SubmitJob("send_discord_reminder", payload)
		if err != nil {
			log.Printf("Bot: Failed to submit reminder job: %v", err)
			errorMsg := "Sorry, there was an error scheduling your reminder. Please try again later."
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &errorMsg,
			})
			return
		}

		responseContent := fmt.Sprintf("Okay! I will remind you to '%s' in %s (Job ID: %s)", message, duration.String(), job.ID)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &responseContent,
		})
	},
}

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "ping",
		Description: "A simple command to check if the bot is responsive.",
	},
	{
		Name:        "remind-simple",
		Description: "Sets a simple one-off reminder.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "duration",
				Description: "How long from now (e.g., 30s, 5m, 1h).",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "What to remind you about.",
				Required:    true,
			},
		},
	},
}
