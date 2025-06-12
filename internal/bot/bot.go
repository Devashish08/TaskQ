package bot

import (
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/service"
	"github.com/bwmarrin/discordgo"
)

// Bot represents the discord bot client and its dependencies
type Bot struct {
	Session *discordgo.Session
	JobSvc  *service.JobService //to enqueue jobs from commands
}

func NewBot(token string, jobService *service.JobService) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("bot token cannnot be empty")
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("error creating discord session: %w", err)
	}

	return &Bot{
		Session: dg,
		JobSvc:  jobService,
	}, nil
}

func (b *Bot) Start() error {
	log.Println("Bot: Starting connection...")

	b.Session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}
		}
	})

	b.Session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Bot: Logged in as %v#%v", s.State.User.Username, s.State.User.Discriminator)
		log.Println("Bot: Registering slash commands...")
		registeredCommands, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, "", commands)
		if err != nil {
			log.Fatalf("Failed to register commands: %v", err)
		}
		log.Printf("Bot: successfully registered %d commands.", len(registeredCommands))
	})

	err := b.Session.Open()
	if err != nil {
		return fmt.Errorf("error opening discord session: %w", err)
	}

	return nil
}

func (b *Bot) Stop() {
	log.Println("Bot: Closing Discord session.")
	log.Println("Bot: Unregistering commands...")

	b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, "", []*discordgo.ApplicationCommand{})

	b.Session.Close()

}
