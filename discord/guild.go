package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"sync"
)

// GuildState struct
type GuildState struct {
	Linked bool

	UserData UserDataSet
	Tracking Tracking

	GameStateMsg GameStateMessage

	StatusEmojis  AlivenessEmojis
	SpecialEmojis map[string]Emoji

	AmongUsData game.AmongUsData
	GameRunning bool

	persistentGuildData *storage.PersistentGuildData
}

type EmojiCollection struct {
	statusEmojis  AlivenessEmojis
	specialEmojis map[string]Emoji
	lock          sync.RWMutex
}

// TrackedMemberAction struct
type TrackedMemberAction struct {
	mute          bool
	move          bool
	message       string
	targetChannel Tracking
}

func (guild *GuildState) checkCacheAndAddUser(g *discordgo.Guild, s *discordgo.Session, userID string) (game.UserData, bool) {
	if g == nil {
		return game.UserData{}, false
	}
	//check and see if they're cached first
	for _, v := range g.Members {
		if v.User.ID == userID {
			user := game.MakeUserDataFromDiscordUser(v.User, v.Nick)
			guild.UserData.AddFullUser(user)
			return user, true
		}
	}
	mem, err := s.GuildMember(guild.persistentGuildData.GuildID, userID)
	if err != nil {
		log.Println(err)
		return game.UserData{}, false
	}
	user := game.MakeUserDataFromDiscordUser(mem.User, mem.Nick)
	guild.UserData.AddFullUser(user)
	return user, true
}

func (bot *Bot) handleReactionGameStartAdd(guild *GuildState, s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	g, err := s.State.Guild(guild.persistentGuildData.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	if guild.GameStateMsg.Exists() {

		//verify that the user is reacting to the state/status message
		if guild.GameStateMsg.IsReactionTo(m) {
			idMatched := false
			for color, e := range guild.StatusEmojis[true] {
				if e.ID == m.Emoji.ID {
					idMatched = true
					log.Printf("Player %s reacted with color %s", m.UserID, game.GetColorStringForInt(color))
					//the user doesn't exist in our userdata cache; add them

					_, added := guild.checkCacheAndAddUser(g, s, m.UserID)
					if !added {
						log.Println("No users found in Discord for userID " + m.UserID)
					}

					playerData := guild.AmongUsData.GetByColor(game.GetColorStringForInt(color))
					if playerData != nil {
						guild.UserData.UpdatePlayerData(m.UserID, playerData)
					} else {
						log.Println("I couldn't find any player data for that color; is your capture linked?")
					}

					//then remove the player's reaction if we matched, or if we didn't
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, e.FormatForReaction(), m.UserID)
					if err != nil {
						log.Println(err)
					}
					break
				}
			}
			if !idMatched {
				//log.Println(m.Emoji.Name)
				if m.Emoji.Name == "❌" {
					log.Printf("Removing player %s", m.UserID)
					guild.UserData.ClearPlayerData(m.UserID)
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, "❌", m.UserID)
					if err != nil {
						log.Println(err)
					}
					idMatched = true
				}
			}
			//make sure to update any voice changes if they occurred
			if idMatched {
				guild.handleTrackedMembers(&bot.SessionManager, 0, NoPriority)
				guild.GameStateMsg.Edit(s, gameStateResponse(guild))
			}
		}
	}
}

// ToString returns a simple string representation of the current state of the guild
func (guild *GuildState) ToString() string {
	return fmt.Sprintf("%v", guild)
}

func (guild *GuildState) clearGameTracking(s *discordgo.Session) {
	//clear the discord user links to underlying player data
	guild.UserData.ClearAllPlayerData()

	//clears the base-level player data in memory
	guild.AmongUsData.ClearAllPlayerData()

	//reset all the tracking channels
	guild.Tracking.Reset()

	guild.GameStateMsg.Delete(s)
}

func (guild *GuildState) CommandPrefix() string {
	return guild.persistentGuildData.GuildSettings.GetCommandPrefix()
}

func (guild *GuildState) EmptyAdminAndRolePerms() bool {
	return guild.persistentGuildData.GuildSettings.EmptyAdminAndRolePerms()
}

func (guild *GuildState) HasAdminPerms(mem *discordgo.Member) bool {
	return guild.persistentGuildData.GuildSettings.HasAdminPerms(mem)
}

func (guild *GuildState) HasRolePerms(mem *discordgo.Member) bool {
	return guild.persistentGuildData.GuildSettings.HasRolePerms(mem)
}

func (guild *GuildState) GetDelay(oldPhase, newPhase game.Phase) int {
	return guild.persistentGuildData.GuildSettings.Delays.GetDelay(oldPhase, newPhase)
}

func (guild *GuildState) UnmuteDeadImmediately() bool {
	return guild.persistentGuildData.GuildSettings.UnmuteDeadDuringTasks
}

func (guild *GuildState) DefaultTrackedChannel() string {
	return guild.persistentGuildData.GuildSettings.DefaultTrackedChannel
}
