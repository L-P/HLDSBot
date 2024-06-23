package bot

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"hldsbot/hlds"
	"hldsbot/twhl"
	"net/url"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

type Bot struct {
	dg               *discordgo.Session
	pool             *hlds.Pool
	steamRedirectURL string

	// There's no way to carry a context through discordgo callbacks, we need
	// to store the "global" one to be notified of cancellations.
	ctx context.Context //nolint:containedctx

	removeHandler func()
}

func New(
	token string,
	steamRedirectURL string,
	pool *hlds.Pool,
) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("unable to create discord session: %w", err)
	}

	return &Bot{
		dg:               dg,
		steamRedirectURL: steamRedirectURL,
		pool:             pool,
		ctx:              context.Background(),
	}, nil
}

func (bot *Bot) Run(ctx context.Context) error {
	bot.ctx = ctx

	log.Info().Msg("Starting Discord session.")
	if err := bot.dg.Open(); err != nil {
		return fmt.Errorf("unable to open discord session: %w", err)
	}
	defer bot.close()

	if err := bot.registerCommands(); err != nil {
		return fmt.Errorf("unable to register commands: %w", err)
	}

	<-ctx.Done()

	return nil
}

func (bot *Bot) close() {
	if bot.removeHandler != nil {
		log.Info().Msg("Removing handler.")
		bot.removeHandler()
	}

	log.Info().Msg("Closing Discord session.")
	if err := bot.dg.Close(); err != nil {
		log.Error().Err(err).Msg("unable to close discord session")
	}
}

type handler func(*discordgo.Session, *discordgo.InteractionCreate)

func (bot *Bot) registerCommands() error {
	var (
		guildID          = ""
		minID    float64 = 1
		commands         = []*discordgo.ApplicationCommand{
			{
				Name:        "hlds",
				Description: "Start a HLDM dedicated server.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "vault-id",
						Description: "Numerical ID of a TWHL Vault HLDM map.",
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    true,
						MinValue:    &minID,
					},
				},
			},
		}
	)

	handlers := map[string]handler{
		"hlds": bot.commandHandlerHLDS,
	}

	var errs = make([]error, 0, len(commands))
	for _, v := range commands {
		log.Debug().Str("name", v.Name).Msg("Registering command.")
		_, err := bot.dg.ApplicationCommandCreate(bot.dg.State.User.ID, guildID, v)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	bot.removeHandler = bot.dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if err := bot.ctx.Err(); errors.Is(err, context.Canceled) {
			log.Error().Err(err).Msg("aborting discord command handler")
			return
		}

		if h, ok := handlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	return errors.Join(errs...)
}

func getOption(i *discordgo.InteractionCreate, name string) (*discordgo.ApplicationCommandInteractionDataOption, bool) {
	if i == nil {
		return nil, false
	}

	for _, v := range i.ApplicationCommandData().Options {
		if v.Name == name {
			return v, true
		}
	}

	return nil, false
}

func (bot *Bot) commandHandlerHLDS(s *discordgo.Session, i *discordgo.InteractionCreate) {
	idOption, ok := getOption(i, "vault-id")
	if !ok {
		log.Error().Err(errors.New("missing vault-id option")).Msg("")
		return
	}
	id := int(idOption.IntValue())

	if err := hldsPleaseWaitResponse(s, i); err != nil {
		log.Error().Err(err).Msg("unable to send waiting response")
		// Other responses are follow-ups, it's no use continuing.
		return
	}

	addonsDir, mapName, err := twhl.FetchAndExtractVaultMap(bot.ctx, id)
	if err != nil {
		log.Error().Err(err).Msg("unable to fetch and extract vault item")
		errorResponse(s, i, "Could not fetch TWHL Vault item.")
		return
	}

	cfg, err := hlds.NewServerConfig(
		1*time.Hour,
		addonsDir,
		2,
		[]string{mapName},
		map[string]string{
			"sv_allow_shaders": "1",
		},
	)
	if err != nil {
		log.Error().Err(err).Msg("unable to create server config")
		errorResponse(s, i, "Could not create server.")
		return
	}

	server, err := bot.pool.AddServer(bot.ctx, cfg)
	if err != nil {
		log.Error().Err(err).Msg("unable to start server")
		errorResponse(s, i, "Could not start server.")
		return
	}

	if err := bot.hldsResponse(s, i, server); err != nil {
		log.Error().Err(err).Msg("unable to respond to command")
	}
}

func errorResponse(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: msg,
		Flags:   discordgo.MessageFlagsEphemeral,
	}); err != nil {
		log.Error().Err(err).Msg("could not send error message")
	}
}

//go:embed hlds_response.tpl
var hldsResponseTPL string

func (bot *Bot) hldsResponse(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	server hlds.Server,
) error {
	if err := s.InteractionResponseDelete(i.Interaction); err != nil {
		log.Error().Err(err).Msg("cannot remove interaction")
	}

	var (
		host     = server.Host()
		password = server.CVar("sv_password")
	)

	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf(hldsResponseTPL, password, host, server.ExpiresAt().Unix()),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					&discordgo.Button{
						Label: fmt.Sprintf("Join %s", server.CVar("hostname")),
						Style: discordgo.LinkButton,
						URL:   bot.generateConnectURL(host, password),
					},
				},
			},
		},
	})

	return err
}

func (bot *Bot) generateConnectURL(host, password string) string {
	ret, err := url.Parse(bot.steamRedirectURL)
	if err != nil {
		panic(fmt.Errorf("could not parse steam redirect URL: %w", err))
	}

	values := ret.Query()
	values.Set("host", host)
	values.Set("password", password)
	ret.RawQuery = values.Encode()

	return ret.String()
}

func hldsPleaseWaitResponse(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: "Creating server, please wait for a few seconds…",
		},
	})
}
