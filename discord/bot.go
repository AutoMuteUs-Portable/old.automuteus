package discord

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	socketio "github.com/googollee/go-socket.io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const DefaultPort = "8123"

// AllConns mapping of socket IDs to guild IDs
var AllConns map[string]string

// AllGuilds mapping of guild IDs to GuildState references
var AllGuilds map[string]*GuildState

// MakeAndStartBot does what it sounds like
func MakeAndStartBot(token string, moveDeadPlayers bool) {

	//red := AlivenessColoredEmojis[true][0]
	//log.Println(red.DownloadAndBase64Encode())

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	port := os.Getenv("SERVER_PORT")
	num, err := strconv.Atoi(port)
	if err != nil || num < 1000 || num > 9999 {
		log.Printf("Invalid or no particular SERVER_PORT provided. Defaulting to %s\n", DefaultPort)
		port = DefaultPort
	}

	dg.AddHandler(voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(reactionCreate)
	dg.AddHandler(newGuild(moveDeadPlayers))

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions)

	//Open a websocket connection to Discord and begin listening.
	err = dg.Open()

	if err != nil {
		log.Println("Could not connect Bot to the Discord Servers with error:", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	AllGuilds = make(map[string]*GuildState)
	AllConns = make(map[string]string)

	gamePhaseUpdateChannel := make(chan game.PhaseUpdate)

	playerUpdateChannel := make(chan game.PlayerUpdate)

	go socketioServer(gamePhaseUpdateChannel, playerUpdateChannel, port)

	go discordListener(dg, gamePhaseUpdateChannel, playerUpdateChannel)

	<-sc

	dg.Close()
}

func socketioServer(gamePhaseUpdateChannel chan<- game.PhaseUpdate, playerUpdateChannel chan<- game.PlayerUpdate, port string) {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		fmt.Println("connected:", s.ID())
		return nil
	})
	server.OnEvent("/", "guildID", func(s socketio.Conn, msg string) {
		fmt.Println("set guildID:", msg)
		for gid, guild := range AllGuilds {
			if gid == msg {
				AllConns[s.ID()] = gid //associate the socket with the guild
				guild.UserDataLock.Lock()
				guild.LinkCode = ""
				guild.UserDataLock.Unlock()

				log.Printf("Associated websocket id %s with guildID %s\n", s.ID(), gid)
				s.Emit("reply", "set guildID successfully")
			}
		}
	})
	server.OnEvent("/", "state", func(s socketio.Conn, msg string) {
		fmt.Println("phase: ", msg)
		phase, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if v, ok := AllConns[s.ID()]; ok {
				log.Println("Pushing phase event to channel")
				gamePhaseUpdateChannel <- game.PhaseUpdate{
					Phase:   game.Phase(phase),
					GuildID: v,
				}
			} else {
				log.Println("This websocket is not associated with any guilds")
			}
		}

	})
	server.OnEvent("/", "player", func(s socketio.Conn, msg string) {
		fmt.Println("player: ", msg)
		player := game.Player{}
		err := json.Unmarshal([]byte(msg), &player)
		if err != nil {
			log.Println(err)
		} else {
			if v, ok := AllConns[s.ID()]; ok {
				playerUpdateChannel <- game.PlayerUpdate{
					Player:  player,
					GuildID: v,
				}
			} else {
				log.Println("This websocket is not associated with any guilds")
			}
		}
	})
	server.OnError("/", func(s socketio.Conn, e error) {
		fmt.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		fmt.Println("Client connection closed: ", reason)

		previousGid := AllConns[s.ID()]
		AllConns[s.ID()] = "" //deassociate the link

		for gid, guild := range AllGuilds {
			if gid == previousGid {
				guild.UserDataLock.Lock()
				guild.LinkCode = gid //set back to the ID; this is unlinked
				guild.UserDataLock.Unlock()

				log.Printf("Deassociated websocket id %s with guildID %s\n", s.ID(), gid)
			}
		}
	})
	go server.Serve()
	defer server.Close()

	http.Handle("/socket.io/", server)
	log.Printf("Serving at localhost:%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func discordListener(dg *discordgo.Session, phaseUpdateChannel <-chan game.PhaseUpdate, playerUpdateChannel <-chan game.PlayerUpdate) {
	for {
		select {
		case phaseUpdate := <-phaseUpdateChannel:
			log.Printf("Received PhaseUpdate message for guild %s\n", phaseUpdate.GuildID)
			if guild, ok := AllGuilds[phaseUpdate.GuildID]; ok {
				//guild.handleGameStateMessage(dg)
				switch phaseUpdate.Phase {
				case game.LOBBY:
					log.Println("Detected transition to lobby")

					guild.AmongUsDataLock.Lock()
					guild.modifyCachedAmongUsDataAlive(true)
					guild.AmongUsDataLock.Unlock()

					guild.handleTrackedMembers(dg, false, false)
					guild.AmongUsDataLock.Lock()
					guild.GamePhase = phaseUpdate.Phase
					guild.AmongUsDataLock.Unlock()

					guild.handleGameStateMessage(dg)
				case game.TASKS:
					log.Println("Detected transition to tasks")
					//delay := 0
					guild.AmongUsDataLock.RLock()
					if guild.GamePhase == game.LOBBY {
						//if we went from lobby to tasks, remove all the emojis from the game start message
						guild.handleReactionsGameStartRemoveAll(dg)

						//delay = guild.delays.GameStartDelay
					} else if guild.GamePhase == game.DISCUSS {
						//delay = guild.delays.GameResumeDelay
					}
					guild.AmongUsDataLock.RUnlock()

					//time.Sleep(time.Second * time.Duration(delay))
					guild.handleTrackedMembers(dg, true, false)

					guild.AmongUsDataLock.Lock()
					guild.GamePhase = phaseUpdate.Phase
					guild.AmongUsDataLock.Unlock()

					guild.handleGameStateMessage(dg)
				case game.DISCUSS:
					log.Println("Detected transition to discussion")
					//time.Sleep(time.Second * time.Duration(guild.delays.DiscussStartDelay))
					guild.AmongUsDataLock.Lock()
					guild.GamePhase = phaseUpdate.Phase
					guild.AmongUsDataLock.Unlock()
					guild.handleTrackedMembers(dg, false, true)
					guild.handleGameStateMessage(dg)
				default:
					log.Printf("Undetected new state: %d\n", phaseUpdate.Phase)
				}
			}

			//TODO prevent cases where 2 players are mapped to the same underlying in-game player data
		case playerUpdate := <-playerUpdateChannel:
			log.Printf("Received PlayerUpdate message for guild %s\n", playerUpdate.GuildID)
			if guild, ok := AllGuilds[playerUpdate.GuildID]; ok {

				//this updates the copies in memory
				//(player's associations to amongus data are just pointers to these structs)
				if playerUpdate.Player.Name != "" {
					updated, isAliveUpdated := guild.updateCachedAmongUsData(playerUpdate.Player)

					if updated {
						log.Println("Player update received caused an update in cached state")
						if isAliveUpdated && guild.GamePhase == game.TASKS {
							log.Println("NOT updating the discord status message; would leak info")
						} else {
							guild.handleGameStateMessage(dg)
						}
					} else {
						log.Println("Player update received did not cause an update in cached state")
					}
				}
			}
		}
	}
}

// Gets called whenever a voice state change occurs
func voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	for id, socketGuild := range AllGuilds {
		if id == m.GuildID {
			socketGuild.voiceStateChange(s, m)
			break
		}
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	for id, socketGuild := range AllGuilds {
		if id == m.GuildID {
			socketGuild.handleMessageCreate(s, m)
			break
		}
	}
}

func reactionCreate(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	for id, socketGuild := range AllGuilds {
		if id == m.GuildID {
			socketGuild.handleReactionGameStartAdd(s, m)
			break
		}
	}
}

func newGuild(moveDeadPlayers bool) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {
		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		AllGuilds[m.ID] = &GuildState{
			ID:            m.ID,
			CommandPrefix: ".au",
			LinkCode:      m.Guild.ID,

			UserData:         make(map[string]UserData),
			Tracking:         make(map[string]Tracking),
			GameStateMessage: nil,
			delays:           GameDelays{},
			UserDataLock:     sync.RWMutex{},

			AmongUsData:     map[string]*AmongUserData{},
			GamePhase:       game.LOBBY,
			Room:            "",
			Region:          "",
			AmongUsDataLock: sync.RWMutex{},

			MoveDeadPlayers: moveDeadPlayers,
		}
		mems, err := s.GuildMembers(m.Guild.ID, "", 1000)
		if err != nil {
			log.Println(err)
		}
		AllGuilds[m.ID].UserDataLock.Lock()
		for _, v := range mems {
			AllGuilds[m.ID].UserData[v.User.ID] = UserData{
				user: User{
					nick:          v.Nick,
					userID:        v.User.ID,
					userName:      v.User.Username,
					discriminator: v.User.Discriminator,
				},
				voiceState: discordgo.VoiceState{},
				tracking:   false,
				auData:     nil,
			}
		}
		AllGuilds[m.ID].UserDataLock.Unlock()
		AllGuilds[m.ID].updateVoiceStatusCache(s)
		log.Println("Updated members for guild " + m.Guild.ID)

		allEmojis, err := s.GuildEmojis(m.Guild.ID)
		if err != nil {
			log.Println(err)
		} else {
			addAllEmojis(s, m.Guild.ID, true, allEmojis)

			addAllEmojis(s, m.Guild.ID, false, allEmojis)

			//addAllEmojis(s, m.Guild.ID, map[int]Emoji{0:AlarmEmoji}, allEmojis)
		}
	}
}

func addAllEmojis(s *discordgo.Session, guildID string, alive bool, serverEmojis []*discordgo.Emoji) {
	for colorInt, emoji := range AlivenessColoredEmojis[alive] {
		alreadyExists := false
		for _, v := range serverEmojis {
			if v.Name == emoji.Name {
				emoji.ID = v.ID
				AlivenessColoredEmojis[alive][colorInt] = emoji
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			b64 := emoji.DownloadAndBase64Encode()
			em, err := s.GuildEmojiCreate(guildID, emoji.Name, b64, nil)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("Added emoji %s successfully!\n", emoji.Name)
				emoji.ID = em.ID
				AlivenessColoredEmojis[alive][colorInt] = emoji
			}
		}
	}
}

func (guild *GuildState) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	guild.updateVoiceStatusCache(s)

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	//TODO This should check VOICE channels, not TEXT channels
	contents := m.Content
	if strings.HasPrefix(contents, guild.CommandPrefix) {
		args := strings.Split(contents, " ")[1:]
		for i, v := range args {
			args[i] = strings.ToLower(v)
		}
		if len(args) == 0 {
			s.ChannelMessageSend(m.ChannelID, helpResponse(guild.CommandPrefix))
		} else {
			switch args[0] {
			case "help":
				fallthrough
			case "h":
				s.ChannelMessageSend(m.ChannelID, helpResponse(guild.CommandPrefix))
				break
			case "track":
				fallthrough
			case "t":
				if len(args[1:]) == 0 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {
					// have to explicitly check for true. Otherwise, processing the 2-word VC names gets really ugly...
					forGhosts := false
					endIdx := len(args)
					if args[len(args)-1] == "true" || args[len(args)-1] == "t" {
						forGhosts = true
						endIdx -= 1
					}

					channelName := strings.Join(args[1:endIdx], " ")

					channels, err := s.GuildChannels(m.GuildID)
					if err != nil {
						log.Println(err)
					}

					guild.UserDataLock.Lock()
					guild.trackChannelResponse(channelName, channels, forGhosts)
					guild.UserDataLock.Unlock()

					guild.handleGameStateMessage(s)
					//_, err = s.ChannelMessageSend(m.ChannelID, resp)
					//if err != nil {
					//	log.Println(err)
					//}
				}
				break

			case "link":
				fallthrough
			case "l":
				if len(args[1:]) < 2 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {
					guild.AmongUsDataLock.Lock()
					guild.UserDataLock.Lock()
					resp := guild.linkPlayerResponse(args[1:], guild.AmongUsData)
					guild.UserDataLock.Unlock()
					guild.AmongUsDataLock.Unlock()
					_, err := s.ChannelMessageSend(m.ChannelID, resp)
					if err != nil {
						log.Println(err)
					}
				}
				break
			case "start":
				fallthrough
			case "s":
				room, region := GetRoomAndRegionFromArgs(args[1:])
				//TODO lock, or don't access directly...
				guild.Room = room
				guild.Region = region

				//TODO if the game is already started, we should re-issue the message and COMPLETELY reset game state?
				//How do we do a fresh purge of all data haha
				//Don't need to purge the AmongUs data, but all the associations/discord links to the data need to be reset,
				//and we need to un-track all the users

				guild.handleGameStartMessage(s, m)
				break
			default:
				s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand that command! Please see `.au help` for commands")

			}
			//TODO in alpha/beta, don't delete the player's message
			deleteMessage(s, m.ChannelID, m.Message.ID)
		}
	}
}

func GetRoomAndRegionFromArgs(args []string) (string, string) {
	if len(args) == 0 {
		return "Unprovided", "Unprovided"
	}
	room := strings.ToUpper(args[0])
	if len(args) == 1 {
		return room, "Unprovided"
	}
	region := strings.ToLower(args[1])
	switch region {
	case "na":
		fallthrough
	case "us":
		fallthrough
	case "usa":
		fallthrough
	case "north":
		region = "North America"
	case "eu":
		fallthrough
	case "europe":
		region = "Europe"
	case "as":
		fallthrough
	case "asia":
		region = "Asia"
	}
	return room, region
}
