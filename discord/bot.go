package discord

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/capture"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const AmongUsDefaultName = "Player"
const AmongUsDefaultColor = "Cyan"

const CommandPrefix = ".au"

type DiscordUser struct {
	nick          string
	userID        string
	userName      string
	discriminator string
}

var GuildMembersCache = make(map[string]DiscordUser)

type UserData struct {
	user         DiscordUser
	voiceState   discordgo.VoiceState
	tracking     bool
	amongUsColor string
	amongUsName  string
	amongUsAlive bool
}

var VoiceStatusCache = make(map[string]UserData)

var GameStartDelay = 0
var GameResumeDelay = 0
var DiscussStartDelay = 0

var ExclusiveChannelId = ""

var TrackingVoiceId = ""
var TrackingVoiceName = ""

func MakeAndStartBot(token, guild, channel string, results chan capture.GameState, gameStartDelay, gameResumeDelay, discussStartDelay int) {
	GameStartDelay = gameStartDelay
	GameResumeDelay = gameResumeDelay
	DiscussStartDelay = discussStartDelay

	ExclusiveChannelId = channel
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	//dg.AddHandler(guildMembers)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuilds)

	//Open a websocket connection to Discord and begin listening.
	err = dg.Open()

	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	mems, err := dg.GuildMembers(guild, "", 1000)
	for _, v := range mems {
		GuildMembersCache[v.User.ID] = DiscordUser{
			nick:          v.Nick,
			userID:        v.User.ID,
			userName:      v.User.Username,
			discriminator: v.User.Discriminator,
		}
	}

	if channel != "" {
		dg.ChannelMessageSend(channel, "Bot is Online!")
	}

	go discordListener(dg, guild, results)

	<-sc

	if channel != "" {
		dg.ChannelMessageSend(channel, "Bot is going Offline!")
	}

	//kill the worker before we terminate the worker forcibly
	results <- capture.KILL

	dg.Close()
}

var GameState capture.GameState

func discordListener(dg *discordgo.Session, guild string, res <-chan capture.GameState) {
	for {
		msg := <-res
		switch msg {
		case capture.KILL:
			return
		case capture.PREGAME:
			if ExclusiveChannelId != "" {
				dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Game over! Unmuting players!"))
			}
			//Loop through and reset players (game over = everyone alive again)
			for i, v := range VoiceStatusCache {
				v.amongUsAlive = true
				VoiceStatusCache[i] = v
			}
			muteAllTrackedMembers(dg, guild, false, false)
			GameState = capture.PREGAME
		case capture.GAME:
			delay := 0
			if GameState == capture.PREGAME {
				delay = GameStartDelay
			} else if GameState == capture.DISCUSS {
				delay = GameResumeDelay
			}
			if ExclusiveChannelId != "" {
				dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Game starting; muting players in %d second(s)!", delay))
			}

			time.Sleep(time.Second * time.Duration(delay))
			muteAllTrackedMembers(dg, guild, true, false)
			GameState = capture.GAME
		case capture.DISCUSS:
			if ExclusiveChannelId != "" {
				dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Starting discussion; unmuting alive players in %d second(s)!", DiscussStartDelay))
			}
			time.Sleep(time.Second * time.Duration(DiscussStartDelay))
			muteAllTrackedMembers(dg, guild, false, true)
			GameState = capture.DISCUSS
		}
	}
}

func muteAllTrackedMembers(dg *discordgo.Session, guildId string, mute bool, checkAlive bool) {
	skipExec := false
	for user, v := range VoiceStatusCache {
		if v.tracking {
			buf := bytes.NewBuffer([]byte{})
			if mute {
				buf.WriteString("Muting ")
			} else {
				if checkAlive {
					if v.amongUsAlive {
						buf.WriteString("Unmuting (alive) ")
					} else {
						buf.WriteString("Not Unmuting (dead) ")
						skipExec = true
					}
				} else {
					buf.WriteString("Unmuting ")
				}
			}
			buf.WriteString(fmt.Sprintf("Username: %s, Nickname: %s, ID: %s", v.user.userName, v.user.nick, user))
			//buf.WriteString(v.User.Username)
			//if v.Nick != "" {
			//buf.WriteString(fmt.Sprintf(" (%s)", v.Nick))
			//}
			log.Println(buf.String())
			if !skipExec {
				err := guildMemberMute(dg, guildId, user, mute)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func guildMemberMute(session *discordgo.Session, guildID string, userID string, mute bool) (err error) {
	data := struct {
		Mute bool `json:"mute"`
	}{mute}

	_, err = session.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
}

// Gets called whenever a voice state change occurs
func voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {

	updateVoiceStatusCache(s, m.GuildID)

	//if the user is already in the voice status cache, only update if we don't know the voice channel to track,
	//or the user has ENTERED this voice channel
	if v, ok := VoiceStatusCache[m.UserID]; ok {
		v.voiceState = *m.VoiceState

		//only track if we have no tracked channel so far, or the user is in the tracked channel
		v.tracking = TrackingVoiceId == "" || m.ChannelID == TrackingVoiceId

		VoiceStatusCache[m.UserID] = v
		log.Printf("Saw a cached \"%s\" user's voice status change, tracking: %v\n", v.user.userName, v.tracking)
		//unmute the member if they left the chat while muted
		if !v.tracking && m.Mute {
			guildMemberMute(s, m.GuildID, m.UserID, false)
			//if the user rejoins, only mute if the game is going, or if it's discussion and they're dead
		} else if v.tracking && (GameState == capture.GAME || (GameState == capture.DISCUSS && !v.amongUsAlive)) {
			guildMemberMute(s, m.GuildID, m.UserID, true)
		}
	} else {
		user := DiscordUser{}
		//if we know of the user from the more general cache (we should)
		if v, ok := GuildMembersCache[m.UserID]; ok {
			user = v
		} else {
			//otherwise, construct a small record just of the userid
			user = DiscordUser{
				nick:          "",
				userID:        m.UserID,
				userName:      "",
				discriminator: "",
			}
		}
		//only track if we have no tracked channel so far, or the user is in the tracked channel. Otherwise, don't track
		tracking := TrackingVoiceId == "" || m.ChannelID == TrackingVoiceId
		log.Printf("Saw \"%s\" user's voice status change, tracking: %v\n", user.userName, tracking)
		VoiceStatusCache[m.UserID] = UserData{
			user:         user,
			voiceState:   *m.VoiceState,
			tracking:     tracking,
			amongUsColor: AmongUsDefaultColor,
			amongUsName:  AmongUsDefaultName,
			amongUsAlive: true,
		}
	}
}

func updateVoiceStatusCache(s *discordgo.Session, guildID string) {
	g, err := s.State.Guild(guildID)
	if err != nil {
		log.Println(err)
	}

	//make sure all the people in the voice status cache are still in voice
	for id, v := range VoiceStatusCache {
		foundUser := false
		for _, state := range g.VoiceStates {
			if state.UserID == id {
				foundUser = true
				break
			}
		}
		//TODO can you server unmute someone not in voice? Prob not...
		if !foundUser {
			v.tracking = false
			VoiceStatusCache[id] = v
		}
	}

	for _, state := range g.VoiceStates {
		//add the user we haven't seen in our cache before
		if _, ok := VoiceStatusCache[state.UserID]; !ok {
			user := DiscordUser{
				userID: state.UserID,
			}
			if v, okok := GuildMembersCache[state.UserID]; okok {
				user = v
			}

			VoiceStatusCache[state.UserID] = UserData{
				user:         user,
				voiceState:   *state,
				tracking:     TrackingVoiceId == "" || state.ChannelID == TrackingVoiceId,
				amongUsColor: "Cyan",
				amongUsName:  "Player",
				amongUsAlive: true,
			}
		}
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	updateVoiceStatusCache(s, m.GuildID)

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	if ExclusiveChannelId == "" || (ExclusiveChannelId == m.ChannelID) {
		contents := m.Content
		if strings.HasPrefix(contents, CommandPrefix) {
			args := strings.Split(contents, " ")[1:]
			for i, v := range args {
				args[i] = strings.ToLower(v)
			}
			if len(args) == 0 {
				s.ChannelMessageSend(m.ChannelID, helpResponse())
			} else {
				switch args[0] {
				case "help":
				case "h":
					s.ChannelMessageSend(m.ChannelID, helpResponse())
					break
				case "add":
				case "a":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
					} else {
						responses := processAddUsersArgs(args[1:])
						buf := bytes.NewBuffer([]byte("Results:\n"))
						for name, msg := range responses {
							buf.WriteString(fmt.Sprintf("`%s`: %s\n", name, msg))
						}
						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
					break
				case "track":
				case "t":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
					} else {
						channelName := strings.Join(args[1:], " ")

						channels, err := s.GuildChannels(m.GuildID)
						if err != nil {
							log.Println(err)
						}

						resp := processTrackChannelArg(channelName, channels)
						s.ChannelMessageSend(m.ChannelID, resp)
					}
					break
				case "list":
				case "l":
					resp := playerListResponse()
					s.ChannelMessageSend(m.ChannelID, resp)
					break
				case "reset":
				case "r":
					for i, v := range VoiceStatusCache {
						v.tracking = false
						v.amongUsAlive = true
						VoiceStatusCache[i] = v
					}
					s.ChannelMessageSend(m.ChannelID, "Reset Player List!")
					break
				case "dead":
				case "d":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
					} else {
						responses := processMarkAliveUsers(s, m.GuildID, args[1:], false)
						buf := bytes.NewBuffer([]byte("Results:\n"))
						for name, msg := range responses {
							buf.WriteString(fmt.Sprintf("`%s`: %s\n", name, msg))
						}
						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
					break
				case "alive":
				case "al":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
					} else {
						responses := processMarkAliveUsers(s, m.GuildID, args[1:], true)
						buf := bytes.NewBuffer([]byte("Results:\n"))
						for name, msg := range responses {
							buf.WriteString(fmt.Sprintf("`%s`: %s\n", name, msg))
						}
						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
					break
				case "unmuteall":
				case "ua":
					s.ChannelMessageSend(m.ChannelID, "Forcibly unmuting ALL players!")
					for id, _ := range VoiceStatusCache {
						err := guildMemberMute(s, m.GuildID, id, false)
						if err != nil {
							log.Println(err)
						}
					}
					break
				case "muteall":
				case "ma":
					s.ChannelMessageSend(m.ChannelID, "Forcibly muting ALL players!")
					for id, _ := range VoiceStatusCache {
						err := guildMemberMute(s, m.GuildID, id, true)
						if err != nil {
							log.Println(err)
						}

					}
					break
				case "broadcast":
				case "bcast":
				case "b":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
					} else {
						str, err := processBroadcastArgs(args[1:])
						if err != nil {
							log.Println(err)
						}
						s.ChannelMessageSend(m.ChannelID, str)
					}
				}
			}
		}
	}
}

func processBroadcastArgs(args []string) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	code, region := "", ""
	//just the room code
	code = strings.ToUpper(args[0])

	if len(args) > 1 {
		region = strings.ToLower(args[1])
		switch region {
		case "na":
		case "north":
			region = "North America"
		case "eu":
		case "europe":
			region = "Europe"
		case "as":
		case "asia":
			region = "Asia"
		}
	}
	for _, player := range VoiceStatusCache {
		if player.tracking {
			buf.WriteString(fmt.Sprintf("<@!%s> ", player.user.userID))
		}
	}
	buf.WriteString(fmt.Sprintf("\nThe Room Code is **%s**\n", code))

	if region == "" {
		buf.WriteString("I wasn't told the Region, though :cry:")
	} else {
		buf.WriteString(fmt.Sprintf("The Region is **%s**\n", region))
	}
	return buf.String(), nil
}

func processAddUsersArgs(args []string) map[string]string {
	responses := make(map[string]string)
	for _, v := range args {
		if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
			//strip the special characters off front and end
			idLookup := v[3 : len(v)-1]
			for id, user := range GuildMembersCache {
				if id == idLookup {
					VoiceStatusCache[id] = UserData{
						user:         user,
						voiceState:   discordgo.VoiceState{},
						tracking:     true, //always assume true if we're adding users manually
						amongUsColor: AmongUsDefaultColor,
						amongUsName:  AmongUsDefaultName,
						amongUsAlive: true,
					}
					nameIdx := user.userName
					if user.nick != "" {
						nameIdx = user.userName + " (" + user.nick + ")"
					}
					responses[nameIdx] = "Added successfully!"
				}
			}
		} else {
			responses[v] = "Not currently supporting non-`@` direct mentions, sorry!"
		}
	}
	return responses
}

func processMarkAliveUsers(dg *discordgo.Session, guildID string, args []string, markAlive bool) map[string]string {
	responses := make(map[string]string)
	for _, v := range args {
		if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
			//strip the special characters off front and end
			idLookup := v[3 : len(v)-1]
			for id, user := range GuildMembersCache {
				if id == idLookup {
					temp := VoiceStatusCache[id]
					temp.amongUsAlive = markAlive
					VoiceStatusCache[id] = temp

					nameIdx := user.userName
					if user.nick != "" {
						nameIdx = user.userName + " (" + user.nick + ")"
					}
					if markAlive {
						responses[nameIdx] = "Marked Alive"
					} else {
						responses[nameIdx] = "Marked Dead"
					}

					if GameState == capture.DISCUSS {
						err := guildMemberMute(dg, guildID, id, !markAlive)
						if err != nil {
							log.Printf("Error muting/unmuting %s: %s\n", user.userName, err)
						}
						if markAlive {
							responses[nameIdx] = "Marked Alive and Unmuted"
						} else {
							responses[nameIdx] = "Marked Dead and Muted"
						}

					}
				}
			}
		} else {
			responses[v] = "Not currently supporting non-`@` direct mentions, sorry!"
		}
	}
	return responses
}

func processTrackChannelArg(channelName string, allChannels []*discordgo.Channel) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {
			TrackingVoiceId = c.ID
			TrackingVoiceName = c.Name
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute!", c.Name)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

func playerListResponse() string {
	buf := bytes.NewBuffer([]byte{})
	if TrackingVoiceId != "" {
		buf.WriteString(fmt.Sprintf("Currently tracking \"%s\" Voice Channel:\n", TrackingVoiceName))
	} else {
		buf.WriteString("Not tracking a Voice Channel; all players will be Automuted (use `.au t` to track)\n")
	}

	buf.WriteString("Player List:\n")
	for _, player := range VoiceStatusCache {
		if player.tracking {
			emoji := ":heart:"
			if !player.amongUsAlive {
				emoji = ":skull:"
			}
			buf.WriteString(fmt.Sprintf("<@!%s>: %s (%s) %s\n", player.user.userID, player.amongUsName, player.amongUsColor, emoji))
		}
	}
	return buf.String()
}
