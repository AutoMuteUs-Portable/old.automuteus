package discord

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
)

func helpResponse() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString("Having issues or have suggestions? Join the discord at https://discord.gg/ZkqZSWF !\n")
	buf.WriteString(fmt.Sprintf("`%s help` or `%s h`: Print help info and command usage.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s start` or `%s s`: Start the game in this text channel. Accepts Room code and Region as arguments.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s list` or `%s ls`: Print the currently tracked players, and their in-game status.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` or `%s t`: Instruct bot to only use the provided voice channel for automute. Accepts true as a final argument if the channel is for dead players. Ex: `%s t <vc_name>` or `%s t <vc_name> true`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s link` or `%s l`: Link a player to their in-game name or color. Ex: `%s l @player cyan` or `%s l @player bob`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	return buf.String()
}

//TODO update original message, not post new one
//TODO delete player messages relating to this
//TODO print the tracked again
func playerListResponse(users map[string]UserData) string {
	buf := bytes.NewBuffer([]byte{})

	buf.WriteString("Player List:\n")
	for _, player := range users {
		if player.tracking {
			if player.auData != nil {
				emoji := AlivenessColoredEmojis[player.auData.IsAlive][player.auData.Color]
				buf.WriteString(fmt.Sprintf("<:%s:%s> <@!%s>: %s\n", emoji.Name, emoji.ID, player.user.userID, player.auData.Name))
			} else {
				buf.WriteString(fmt.Sprintf(":x: <@!%s>: Use `.au link @%s <Au name OR color>`\n", player.user.userID, player.user.userName))
			}

		}
	}
	return buf.String()
}

func (guild *GuildState) trackChannelResponse(channelName string, allChannels []*discordgo.Channel, forGhosts bool) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {
			//TODO check duplicates (for logging)
			guild.Tracking[c.ID] = Tracking{
				channelID:   c.ID,
				channelName: c.Name,
				forGhosts:   forGhosts,
			}
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

func (guild *GuildState) excludePlayerResponse(args []string, allUserData map[string]UserData) string {
	userID, err := extractUserIDFromMention(args[0])
	if err != nil {
		return fmt.Sprintf("Invalid mention format for \"%s\"", args[0])
	}

	for name, userData := range allUserData {
		if userData.user.userID == userID {
			userData.excluding = true
			allUserData[name] = userData
		}
	}
	return ""
}

//TODO implement a separate way to link by color (color is more volatile)
//TODO implement a way to match in-game names to discord ones WITHOUT the link command
func (guild *GuildState) linkPlayerResponse(args []string, allAuData map[string]*AmongUserData) string {
	userID, err := extractUserIDFromMention(args[0])
	if err != nil {
		return fmt.Sprintf("Invalid mention format for \"%s\"", args[0])
	}

	combinedArgs := strings.ToLower(strings.Join(args[1:], ""))

	if IsColorString(combinedArgs) {
		str, _ := guild.matchByColor(userID, combinedArgs, allAuData)
		return str
	}

	inGameName := combinedArgs
	for name, auData := range allAuData {
		name = strings.ToLower(strings.ReplaceAll(name, " ", ""))
		log.Println(name)
		if name == inGameName {
			if user, ok := guild.UserData[userID]; ok {
				user.auData = auData //point to the single copy in memory
				guild.UserData[userID] = user
				log.Printf("Linked %s to %s", args[0], user.auData.ToString())
				return fmt.Sprintf("Successfully linked player via Name!")
			}
			return fmt.Sprintf("No user found with userID %s", userID)
		}
	}
	return fmt.Sprintf(":x: No in-game name was found matching %s!\n", inGameName)
}

func (guild *GuildState) matchByColor(userID, text string, allAuData map[string]*AmongUserData) (string, bool) {
	for _, auData := range allAuData {
		if GetColorStringForInt(auData.Color) == strings.ToLower(text) {
			if user, ok := guild.UserData[userID]; ok {
				user.auData = auData //point to the single copy in memory
				guild.UserData[userID] = user
				log.Printf("Linked %s to %s", userID, user.auData.ToString())
				return fmt.Sprintf("Successfully linked player via Color!"), true
			}
			return fmt.Sprintf("No user found with userID %s", userID), false
		}
	}
	return fmt.Sprintf(":x: No in-game player data was found matching that color!\n"), false
}

// TODO:
func gameStateResponse(guild *GuildState) string {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(guild *GuildState) string{
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[guild.GamePhase](guild)
}

func lobbyMessage(g *GuildState) string {
	buf := bytes.NewBuffer([]byte{})

	buf.WriteString("Lobby is open!\n")
	buf.WriteString(fmt.Sprintf("The Lobby Code is: **%s** and the Region is: **%s**\n", g.Room, g.Region)) // maybe this is a toggle?
	buf.WriteString(playerListResponse(g.UserData))
	buf.WriteString("React to this message with your in-game color once you join the game!")

	return buf.String()
}

func gamePlayMessage(guild *GuildState) string {
	buf := bytes.NewBuffer([]byte{})

	buf.WriteString("Game is running!\n")
	// add the player list
	guild.UserDataLock.RLock()
	buf.WriteString(playerListResponse(guild.UserData))
	guild.UserDataLock.RUnlock()

	guild.GamePhaseLock.RLock()
	buf.WriteString(fmt.Sprintf("Current Phase: %s\n", guild.GamePhase.ToString()))
	guild.GamePhaseLock.RUnlock()

	return buf.String()
}

func extractUserIDFromMention(mention string) (string, error) {
	//nickname format
	if strings.HasPrefix(mention, "<@!") && strings.HasSuffix(mention, ">") {
		return mention[3 : len(mention)-1], nil
		//non-nickname format
	} else if strings.HasPrefix(mention, "<@") && strings.HasSuffix(mention, ">") {
		return mention[2 : len(mention)-1], nil
	} else {
		return "", errors.New("mention does not conform to the correct format")
	}
}
