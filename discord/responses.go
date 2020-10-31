package discord

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/locale"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func helpResponse(version, CommandPrefix string, commands []Command) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.Title",
			Other: "AutoMuteUs Bot Commands (v{{.version}}):\n",
		},
			map[string]interface{}{
				"version": version,
			}),
		Description: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.SubTitle",
			Other: "[View the Github Project](https://github.com/denverquane/automuteus) or [Join our Discord](https://discord.gg/ZkqZSWF)\n\nType `{{.CommandPrefix}} help <command>` to see more details on a command!",
		},
			map[string]interface{}{
				"CommandPrefix": CommandPrefix,
			}),
		Timestamp: "",
		Color:     15844367, //GOLD
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      "https://github.com/denverquane/automuteus/blob/master/assets/BotProfilePicture.png?raw=true",
			ProxyURL: "",
			Width:    0,
			Height:   0,
		},
		Video:    nil,
		Provider: nil,
		Author:   nil,
		Footer:   nil,
	}

	fields := make([]*discordgo.MessageEmbedField, 0)
	for _, v := range commands {
		if !v.secret && v.cmdType != Help && v.cmdType != Null {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   v.emoji + " " + v.command,
				Value:  locale.LocalizeMessage(v.shortDesc),
				Inline: true,
			})
		}
	}

	embed.Fields = fields
	return embed
}

func settingResponse(CommandPrefix string, settings []Setting) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.Title",
			Other: "Settings",
		}),
		Description: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.Description",
			Other: "Type `{{.CommandPrefix}} settings <setting>` to change a setting from those listed below",
		},
			map[string]interface{}{
				"CommandPrefix": CommandPrefix,
			}),
		Timestamp: "",
		Color:     15844367, //GOLD
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
	}

	fields := make([]*discordgo.MessageEmbedField, len(settings))
	for i, v := range settings {
		fields[i] = &discordgo.MessageEmbedField{
			Name:   v.name,
			Value:  locale.LocalizeMessage(v.shortDesc),
			Inline: true,
		}
	}

	embed.Fields = fields
	return embed
}

// TODO:
func (bot *Bot) gameStateResponse(dgs *DiscordGameState) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed{
		game.MENU:    menuMessage,
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[dgs.AmongUsData.Phase](dgs, bot.StatusEmojis)
}

func lobbyMetaEmbedFields(tracking TrackingChannel, room, region string, playerCount int, linkedPlayers int) []*discordgo.MessageEmbedField {
	gameInfoFields := make([]*discordgo.MessageEmbedField, 4)
	gameInfoFields[0] = &discordgo.MessageEmbedField{
		Name: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMetaEmbedFields.RoomCode",
			Other: "Room Code",
		}),
		Value:  fmt.Sprintf("%s", room),
		Inline: true,
	}
	gameInfoFields[1] = &discordgo.MessageEmbedField{
		Name: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMetaEmbedFields.Region",
			Other: "Region",
		}),
		Value:  fmt.Sprintf("%s", region),
		Inline: true,
	}
	gameInfoFields[2] = &discordgo.MessageEmbedField{
		Name: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMetaEmbedFields.Tracking",
			Other: "Tracking",
		}),
		Value:  tracking.ToStatusString(),
		Inline: true,
	}
	gameInfoFields[3] = &discordgo.MessageEmbedField{
		Name: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMetaEmbedFields.PlayersLinked",
			Other: "Players Linked",
		}),
		Value:  fmt.Sprintf("%v/%v", linkedPlayers, playerCount),
		Inline: false,
	}

	return gameInfoFields
}

func menuMessage(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed {

	color := 15158332 //red
	desc := ""
	var footer *discordgo.MessageEmbedFooter
	if dgs.Linked {
		desc = dgs.makeDescription()
		color = 3066993
		footer = &discordgo.MessageEmbedFooter{
			Text: locale.LocalizeMessage(&i18n.Message{
				ID:    "responses.menuMessage.Linked.FooterText",
				Other: "(Enter a game lobby in Among Us to start the match)",
			}),
			IconURL:      "",
			ProxyIconURL: "",
		}
	} else {
		desc = locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.menuMessage.notLinked.Description",
			Other: "❌**No capture linked! Click the link in your DMs to connect!**❌",
		})
		footer = nil
	}

	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.menuMessage.Title",
			Other: "Main Menu",
		}),
		Description: desc,
		Timestamp:   "",
		Footer:      footer,
		Color:       color,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      nil,
	}
	return &msg
}

func lobbyMessage(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed {
	//gameInfoFields[2] = &discordgo.MessageEmbedField{
	//	Name:   "\u200B",
	//	Value:  "\u200B",
	//	Inline: false,
	//}
	room, region := dgs.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(dgs.Tracking, room, region, dgs.AmongUsData.GetNumDetectedPlayers(), dgs.GetCountLinked())

	listResp := dgs.ToEmojiEmbedFields(emojis)
	listResp = append(gameInfoFields, listResp...)

	color := 15158332 //red
	desc := ""
	if dgs.Linked {
		desc = dgs.makeDescription()
		color = 3066993
	} else {
		desc = locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMessage.notLinked.Description",
			Other: "❌**No capture linked! Click the link in your DMs to connect!**❌",
		})
	}

	emojiLeave := "❌"
	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMessage.Title",
			Other: "Lobby",
		}),
		Description: desc,
		Timestamp:   "",
		Footer: &discordgo.MessageEmbedFooter{
			Text: locale.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMessage.Footer.Text",
				Other: "React to this message with your in-game color! (or {{.emojiLeave}} to leave)",
			},
				map[string]interface{}{
					"emojiLeave": emojiLeave,
				}),
			IconURL:      "",
			ProxyIconURL: "",
		},
		Color:     color,
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields:    listResp,
	}
	return &msg
}

func gamePlayMessage(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed {
	// add the player list
	//guild.UserDataLock.Lock()
	room, region := dgs.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(dgs.Tracking, room, region, dgs.AmongUsData.GetNumDetectedPlayers(), dgs.GetCountLinked())
	listResp := dgs.ToEmojiEmbedFields(emojis)
	listResp = append(gameInfoFields, listResp...)
	//guild.UserDataLock.Unlock()
	var color int

	phase := dgs.AmongUsData.GetPhase()

	switch phase {
	case game.TASKS:
		color = 3447003 //BLUE
	case game.DISCUSS:
		color = 10181046 //PURPLE
	default:
		color = 15158332 //RED
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       locale.LocalizeMessage(phase.ToLocale()),
		Description: dgs.makeDescription(),
		Timestamp:   "",
		Color:       color,
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      listResp,
	}

	return &msg
}

func (dgs *DiscordGameState) makeDescription() string {
	buf := bytes.NewBuffer([]byte{})
	if !dgs.Running {
		buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.GameNotRunning",
			Other: "\n**Bot is Paused!**\n\n",
		}))
	}

	author := dgs.GameStateMsg.LeaderID
	if author != "" {
		buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.author",
			Other: "<@{{.author}}> is running an Among Us game!\nThe game is happening in ",
		},
			map[string]interface{}{
				"author": author,
			}))
	}

	buf.WriteString(dgs.Tracking.ToDescString())

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

func extractRoleIDFromMention(mention string) (string, error) {
	//role is formatted <&123456>
	if strings.HasPrefix(mention, "<@&") && strings.HasSuffix(mention, ">") {
		return mention[3 : len(mention)-1], nil
	} else {
		return "", errors.New("mention does not conform to the correct format")
	}
}
