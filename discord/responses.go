package discord

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/automuteus/galactus/broker"
	"log"
	"strings"
	"time"

	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var EmojiNums = []string{":one:", ":two:", ":three:"}

const ISO8601 = "2006-01-02T15:04:05-0700"

func helpResponse(isAdmin, isPermissioned bool, CommandPrefix string, commands []Command, sett *storage.GuildSettings) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.Title",
			Other: "AutoMuteUs Bot Commands:\n",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
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
			if (!v.adminSetting || isAdmin) && (!v.permissionSetting || isPermissioned) {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   v.emoji + " " + v.command,
					Value:  sett.LocalizeMessage(v.shortDesc),
					Inline: true,
				})
			}
		}
	}
	if len(fields)%3 == 2 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200B",
			Value:  "\u200B",
			Inline: true,
		})
	}

	embed.Fields = fields
	return embed
}

func settingResponse(CommandPrefix string, settings []Setting, sett *storage.GuildSettings, prem bool) *discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.Title",
			Other: "Settings",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
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

	fields := make([]*discordgo.MessageEmbedField, 0)
	for _, v := range settings {
		if !v.premium || v.premium == prem {
			name := v.name
			if v.premium {
				name = "💎 " + name
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   name,
				Value:  sett.LocalizeMessage(v.shortDesc),
				Inline: true,
			})
		}
	}

	embed.Fields = fields
	return &embed
}

func (bot *Bot) infoResponse(sett *storage.GuildSettings) *discordgo.MessageEmbed {
	version, commit := broker.GetVersionAndCommit(bot.RedisInterface.client)
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Title",
			Other: "Bot Info",
		}),
		Description: "",
		Timestamp:   time.Now().Format(ISO8601),
		Color:       2067276, //DARK GREEN
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Footer: &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.statsResponse.BotInfo",
				Other: "v{{.Version}}-{{.Commit}} | Shard {{.ID}}/{{.Num}}",
			},
				map[string]interface{}{
					"Version": version,
					"Commit":  commit,
					"ID":      fmt.Sprintf("%d", bot.PrimarySession.ShardID),
					"Num":     fmt.Sprintf("%d", bot.PrimarySession.ShardCount),
				}),
			IconURL:      "",
			ProxyIconURL: "",
		},
	}

	totalGuilds := broker.GetGuildCounter(bot.RedisInterface.client, version)
	totalGames := broker.GetActiveGames(bot.RedisInterface.client, GameTimeoutSeconds)

	fields := make([]*discordgo.MessageEmbedField, 8)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Version",
			Other: "Version",
		}),
		Value:  version,
		Inline: true,
	}
	fields[1] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Guilds",
			Other: "Total Guilds",
		}),
		Value:  fmt.Sprintf("%d", totalGuilds),
		Inline: true,
	}
	fields[2] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Games",
			Other: "Active Games",
		}),
		Value:  fmt.Sprintf("%d", totalGames),
		Inline: true,
	}
	fields[3] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Library",
			Other: "Library",
		}),
		Value:  "discordgo",
		Inline: true,
	}
	fields[4] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Creator",
			Other: "Creator",
		}),
		Value:  "Soup#4222",
		Inline: true,
	}
	fields[5] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Website",
			Other: "Website",
		}),
		Value:  "[automute.us](https://automute.us)",
		Inline: true,
	}
	fields[6] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Invite",
			Other: "Invite",
		}),
		Value:  "[add.automute.us](https://add.automute.us)",
		Inline: true,
	}
	fields[7] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Donate",
			Other: "Donate",
		}),
		Value:  "[patreon/automuteus](https://www.patreon.com/automuteus)",
		Inline: true,
	}

	embed.Fields = fields
	return &embed
}

func (bot *Bot) gameStateResponse(dgs *DiscordGameState, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed{
		game.MENU:     menuMessage,
		game.LOBBY:    lobbyMessage,
		game.TASKS:    gamePlayMessage,
		game.DISCUSS:  gamePlayMessage,
		game.GAMEOVER: gamePlayMessage, //uses the phase to print the info differently
	}
	return messages[dgs.AmongUsData.Phase](dgs, bot.StatusEmojis, sett)
}

func lobbyMetaEmbedFields(room, region string, playerCount int, linkedPlayers int, sett *storage.GuildSettings) []*discordgo.MessageEmbedField {
	gameInfoFields := make([]*discordgo.MessageEmbedField, 0)
	if room != "" {
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.RoomCode",
				Other: "🔒 ROOM CODE",
			}),
			Value:  fmt.Sprintf("%s", room),
			Inline: false,
		})
	}
	if region != "" {
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.Region",
				Other: "🌎 REGION",
			}),
			Value:  fmt.Sprintf("%s", region),
			Inline: false,
		})
	}

	//necessary with the latest checks for linked players
	//probably still broken, though -_-
	if linkedPlayers > playerCount {
		linkedPlayers = playerCount
	}
	gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMetaEmbedFields.PlayersLinked",
			Other: "Players Linked",
		}),
		Value:  fmt.Sprintf("%v/%v", linkedPlayers, playerCount),
		Inline: false,
	})

	return gameInfoFields
}

func menuMessage(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed {

	color := 15158332 //red
	desc := ""
	var footer *discordgo.MessageEmbedFooter
	if dgs.Linked {
		desc = dgs.makeDescription(sett)
		color = 3066993
		footer = &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.menuMessage.Linked.FooterText",
				Other: "(Enter a game lobby in Among Us to start the match)",
			}),
			IconURL:      "",
			ProxyIconURL: "",
		}
	} else {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.menuMessage.notLinked.Description",
			Other: "❌**No capture linked! Click the link in your DMs to connect!**❌",
		})
		footer = nil
	}

	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.menuMessage.Title",
			Other: "Main Menu",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
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

func lobbyMessage(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	room, region := dgs.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(room, region, dgs.AmongUsData.GetNumDetectedPlayers(), dgs.GetCountLinked(), sett)

	listResp := dgs.ToEmojiEmbedFields(emojis, sett)
	listResp = append(gameInfoFields, listResp...)

	color := 15158332 //red
	desc := ""
	if dgs.AmongUsData.GetPhase() != game.GAMEOVER {
		if dgs.Linked {
			desc = dgs.makeDescription(sett)
			color = 3066993
		} else {
			desc = sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMessage.notLinked.Description",
				Other: "❌**No capture linked! Click the link in your DMs to connect!**❌",
			})
		}
	}

	emojiLeave := "❌"
	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMessage.Title",
			Other: "Lobby",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Footer: &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
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

func gamePlayMessage(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed {

	phase := dgs.AmongUsData.GetPhase()
	//send empty fields because we don't need to display those fields during the game...
	listResp := dgs.ToEmojiEmbedFields(emojis, sett)
	desc := ""

	//if game is over, dont append the player count or the other description fields
	if phase != game.GAMEOVER {
		desc = dgs.makeDescription(sett)
		gameInfoFields := lobbyMetaEmbedFields("", "", dgs.AmongUsData.GetNumDetectedPlayers(), dgs.GetCountLinked(), sett)
		listResp = append(gameInfoFields, listResp...)
	}

	var color int
	switch phase {
	case game.TASKS:
		color = 3447003 //BLUE
	case game.DISCUSS:
		color = 10181046 //PURPLE
	case game.GAMEOVER:
		color = 12745742 //DARK GOLD
	default:
		color = 15158332 //RED
	}
	title := sett.LocalizeMessage(phase.ToLocale())
	if phase == game.GAMEOVER {
		title = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.title.GameOver",
			Other: "**Game Over**",
		})
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       title,
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
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

func (dgs *DiscordGameState) makeDescription(sett *storage.GuildSettings) string {
	buf := bytes.NewBuffer([]byte{})
	if !dgs.Running {
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.GameNotRunning",
			Other: "\n⚠ **Bot is Paused!** ⚠\n\n",
		}))
	}

	author := dgs.GameStateMsg.LeaderID
	if author != "" {
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.author",
			Other: "<@{{.author}}> is running an Among Us game!\nThe game is happening in ",
		},
			map[string]interface{}{
				"author": author,
			}))
	}

	buf.WriteString(dgs.Tracking.ToDescString(sett))

	return buf.String()
}

func premiumEmbedResponse(tier storage.PremiumTier, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	desc := ""
	fields := []*discordgo.MessageEmbedField{}

	if tier != storage.FreeTier {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumResponse.PremiumDescription",
			Other: "Looks like you have AutoMuteUs **{{.Tier}}**! Thanks for the support!\n\nBelow are some of the benefits you can customize with your Premium status!",
		},
			map[string]interface{}{
				"Tier": storage.PremiumTierStrings[tier],
			})

		fields = []*discordgo.MessageEmbedField{
			{
				Name: "Bot Invites",
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.Invites",
					Other: "View a list of Premium bots you can invite with `{{.CommandPrefix}} premium invites`!",
				}, map[string]interface{}{
					"CommandPrefix": sett.GetCommandPrefix(),
				}),
				Inline: false,
			},
			{
				Name: "Premium Settings",
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.Settings",
					Other: "Look for the settings marked with 💎 under `{{.CommandPrefix}} settings!`",
				}, map[string]interface{}{
					"CommandPrefix": sett.GetCommandPrefix(),
				}),
				Inline: false,
			},
		}
	} else {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumResponse.FreeDescription",
			Other: "Check out the cool things that Premium AutoMuteUs has to offer!\n\n[Get AutoMuteUs Premium](https://patreon.com/automuteus)",
		})
		//TODO localize
		fields = []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.PriorityGameAccess",
					Other: "👑 Priority Game Access",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.PriorityGameAccessDesc",
					Other: "If the Bot is under heavy load, Premium users will always be able to make new games!",
				}),
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.FastMute",
					Other: "🙊 Fast Mute/Deafen",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.FastMuteDesc",
					Other: "Premium users get access to \"helper\" bots that make sure muting is fast!",
				}),
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.Stats",
					Other: "📊 Game Stats and Leaderboards",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.StatsDesc",
					Other: "Premium users have access to a full suite of player stats and leaderboards!",
				}),
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.Settings",
					Other: "🛠 Special Settings",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.SettingsDesc",
					Other: "Premium users can specify additional settings, like displaying an end-game status message, or auto-refreshing the status message!",
				}),
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.Support",
					Other: "👂 Premium Support",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.SupportDesc",
					Other: "Premium users get access to private channels on our official Discord channel!",
				}),
				Inline: false,
			},
		}
	}

	msg := discordgo.MessageEmbed{
		URL:  "https://patreon.com/automuteus",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumResponse.Title",
			Other: "💎 AutoMuteUs Premium 💎",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Color:       10181046, //PURPLE
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      fields,
	}
	return &msg
}

func nonPremiumSettingResponse(sett *storage.GuildSettings) string {
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.nonPremiumSetting.Desc",
		Other: "Sorry, but that setting is reserved for AutoMuteUs Premium users! See `{{.CommandPrefix}} premium` for details",
	}, map[string]interface{}{
		"CommandPrefix": sett.GetCommandPrefix(),
	})
}

//if you're reading this, adding these bots won't help you.
//Galactus+AutoMuteUs verify the premium status internally before using these bots ;)
var BotInvites = []string{
	"https://discord.com/api/oauth2/authorize?client_id=780323275624546304&permissions=12582912&scope=bot",
	"https://discord.com/api/oauth2/authorize?client_id=769022114229125181&permissions=12582912&scope=bot",
	"https://discord.com/api/oauth2/authorize?client_id=780323801173983262&permissions=25165824&scope=bot"}

func premiumInvitesEmbed(tier storage.PremiumTier, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	desc := ""
	fields := []*discordgo.MessageEmbedField{}

	if tier == storage.FreeTier || tier == storage.BronzeTier {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponseNoAccess.desc",
			Other: "{{.Tier}} users don't have access to Priority mute bots!\nPlease type `{{.CommandPrefix}} premium` to see more details about AutoMuteUs Premium",
		}, map[string]interface{}{
			"Tier":          storage.PremiumTierStrings[tier],
			"CommandPrefix": sett.GetCommandPrefix(),
		})
	} else {
		count := 0
		if tier == storage.SilverTier {
			count = 1
		} else if tier == storage.GoldTier || tier == storage.PlatTier {
			count = 3
		}
		//TODO account for Platinum
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponse.desc",
			Other: "{{.Tier}} users have access to {{.Count}} Priority mute bots: invites provided below!",
		}, map[string]interface{}{
			"Tier":          storage.PremiumTierStrings[tier],
			"Count":         count,
			"CommandPrefix": sett.GetCommandPrefix(),
		})

		for i := 0; i < count; i++ {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("Bot %s", EmojiNums[i]),
				Value:  fmt.Sprintf("[Invite](%s)", BotInvites[i]),
				Inline: false,
			})
		}
	}

	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponse.Title",
			Other: "Premium Bot Invites",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Color:       10181046, //PURPLE
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      fields,
	}
	return &msg
}

func (bot *Bot) privacyResponse(guildID, authorID, arg string, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	desc := ""

	switch arg {
	case "showme":
		cached := bot.RedisInterface.GetUsernameOrUserIDMappings(guildID, authorID)
		if len(cached) == 0 {
			desc = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.ShowMe.emptyCachedNames",
				Other: "❌ {{.User}} I don't have any cached player names stored for you!",
			}, map[string]interface{}{
				"User": "<@!" + authorID + ">",
			})
		} else {
			buf := bytes.NewBuffer([]byte(sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.ShowMe.cachedNames",
				Other: "❗ {{.User}} Here's your cached in-game names:",
			}, map[string]interface{}{
				"User": "<@!" + authorID + ">",
			})))
			buf.WriteString("\n```\n")
			for n := range cached {
				buf.WriteString(fmt.Sprintf("%s\n", n))
			}
			buf.WriteString("```")
			desc = buf.String()
		}
		desc += "\n"
		user, _ := bot.PostgresInterface.GetUserByString(authorID)

		if user != nil && user.Opt && user.UserID != 0 {
			desc += sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.ShowMe.linkedID",
				Other: "❗ {{.User}} You are opted **in** to data collection for game statistics",
			}, map[string]interface{}{
				"User": "<@!" + authorID + ">",
			})
		} else {
			desc += sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.ShowMe.unlinkedID",
				Other: "❌ {{.User}} You are opted **out** of data collection for game statistics, or you haven't played a game yet",
			}, map[string]interface{}{
				"User": "<@!" + authorID + ">",
			})
		}
	case "optout":
		err := bot.RedisInterface.DeleteLinksByUserID(guildID, authorID)
		if err != nil {
			log.Println(err)
		} else {
			desc = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.ForgetMe.Success",
				Other: "✅ {{.User}} I successfully deleted your cached player names",
			},
				map[string]interface{}{
					"User": "<@!" + authorID + ">",
				})
			desc += "\n"
			didOpt, err := bot.PostgresInterface.OptUserByString(authorID, false)
			if err != nil {
				log.Println(err)
			} else {
				if didOpt {
					desc += sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.optout.SuccessDB",
						Other: "✅ {{.User}} I successfully opted you out of data collection",
					},
						map[string]interface{}{
							"User": "<@!" + authorID + ">",
						})
				} else {
					desc += sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.optout.FailDB",
						Other: "❌ {{.User}} You are already opted out of data collection",
					},
						map[string]interface{}{
							"User": "<@!" + authorID + ">",
						})
				}

			}
		}
	case "optin":
		didOpt, err := bot.PostgresInterface.OptUserByString(authorID, true)
		if err != nil {
			log.Println(err)
		} else {
			if didOpt {
				desc += sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.HandleCommand.optin.SuccessDB",
					Other: "✅ {{.User}} I successfully opted you into data collection",
				},
					map[string]interface{}{
						"User": "<@!" + authorID + ">",
					})
			} else {
				desc += sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.HandleCommand.optin.FailDB",
					Other: "❌ {{.User}} You are already opted into data collection",
				},
					map[string]interface{}{
						"User": "<@!" + authorID + ">",
					})
			}

		}
	}

	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.privacyResponse.Title",
			Other: "AutoMuteUs Privacy",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Color:       10181046, //PURPLE
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      nil,
	}
	return &msg
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
