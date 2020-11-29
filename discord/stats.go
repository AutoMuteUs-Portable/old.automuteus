package discord

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"strconv"
)

const UserLeaderboardCount = 3

func (bot *Bot) UserStatsEmbed(userID, guildID string, sett *storage.GuildSettings, premium storage.PremiumTier) *discordgo.MessageEmbed {
	gamesPlayed := bot.PostgresInterface.NumGamesPlayedByUser(userID)
	gamesPlayedServer := bot.PostgresInterface.NumGamesPlayedByUserOnServer(userID, guildID)
	winsOnServer := bot.PostgresInterface.NumWinsOnServer(userID, guildID)

	avatarUrl := ""
	mem, err := bot.PrimarySession.GuildMember(guildID, userID)
	if err != nil {
		log.Println(err)
	} else if mem.User != nil {
		avatarUrl = mem.User.AvatarURL("")
	}

	fields := make([]*discordgo.MessageEmbedField, 2)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.GamesPlayed",
			Other: "Games Played",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayedServer),
		Inline: true,
	}
	fields[1] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.TotalWins",
			Other: "Total Wins",
		}),
		Value:  fmt.Sprintf("%d", winsOnServer),
		Inline: true,
	}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "\u200b",
		Value:  "\u200b",
		Inline: true,
	})

	extraDesc := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.userStatsEmbed.NoPremium",
		Other: "Detailed stats are only available for AutoMuteUs Premium users; type `{{.CommandPrefix}} premium` to learn more",
	}, map[string]interface{}{
		"CommandPrefix": sett.CommandPrefix,
	})

	if premium != storage.FreeTier {
		extraDesc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Premium",
			Other: "Showing additional Premium Stats!",
		})
		colorRankings := bot.PostgresInterface.ColorRankingForPlayer(userID)
		if len(colorRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(colorRankings) && i < UserLeaderboardCount; i++ {
				elem := colorRankings[i]
				emoji := bot.StatusEmojis[true][elem.Mode]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", emoji.FormatForInline(), 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(colorRankings)-1 && i < UserLeaderboardCount-1 {
					buf.WriteByte('\n')
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.FavoriteColors",
					Other: "Favorite Colors",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}
		nameRankings := bot.PostgresInterface.NamesRankingForPlayer(userID)
		if len(nameRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(nameRankings) && i < UserLeaderboardCount; i++ {
				elem := nameRankings[i]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", elem.Mode, 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(nameRankings)-1 && i < UserLeaderboardCount-1 {
					buf.WriteByte('\n')
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.FavoriteNames",
					Other: "Favorite Names",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200b",
			Value:  "\u200b",
			Inline: true,
		})

		totalCrewmateGames := bot.PostgresInterface.NumGamesAsRoleOnServer(userID, guildID, int16(game.CrewmateRole))
		if totalCrewmateGames > 0 {
			crewmateWins := bot.PostgresInterface.NumWinsAsRoleOnServer(userID, guildID, int16(game.CrewmateRole))
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.CrewmateWins",
					Other: "Crewmate Wins",
				}),
				Value:  fmt.Sprintf("%d/%d games | %.0f%%", crewmateWins, totalCrewmateGames, 100.0*float64(crewmateWins)/float64(totalCrewmateGames)),
				Inline: true,
			})
		}
		totalImposterGames := bot.PostgresInterface.NumGamesAsRoleOnServer(userID, guildID, int16(game.ImposterRole))
		if totalImposterGames > 0 {
			imposterWins := bot.PostgresInterface.NumWinsAsRoleOnServer(userID, guildID, int16(game.ImposterRole))
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.ImposterWins",
					Other: "Imposter Wins",
				}),
				Value:  fmt.Sprintf("%d/%d games | %.0f%%", imposterWins, totalImposterGames, 100.0*float64(imposterWins)/float64(totalImposterGames)),
				Inline: true,
			})
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200b",
			Value:  "\u200b",
			Inline: true,
		})

	}

	var embed = discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Title",
			Other: "User Stats",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Desc",
			Other: "User stats for {{.User}} on this Server",
		}, map[string]interface{}{
			"User": "<@!" + userID + ">",
		}) + "\n\n" + extraDesc,
		Timestamp: "",
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      avatarUrl,
			ProxyURL: "",
			Width:    0,
			Height:   0,
		},
		Video:    nil,
		Provider: nil,
		Author:   nil,
		Fields:   fields,
	}
	return &embed
}

const LeaderboardSize = 5

func (bot *Bot) GuildStatsEmbed(guildID string, sett *storage.GuildSettings, premium storage.PremiumTier) *discordgo.MessageEmbed {
	gname := ""
	avatarUrl := ""
	g, err := bot.PrimarySession.Guild(guildID)

	if err != nil {
		log.Println(err)
		gname = guildID
	} else {
		gname = g.Name
		avatarUrl = g.IconURL()
	}

	gamesPlayed := bot.PostgresInterface.NumGamesPlayedOnGuild(guildID)

	fields := make([]*discordgo.MessageEmbedField, 1)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.GamesPlayed",
			Other: "Games Played",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayed),
		Inline: true,
	}

	extraDesc := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.guildStatsEmbed.NoPremium",
		Other: "Detailed stats are only available for AutoMuteUs Premium users; type `{{.CommandPrefix}} premium` to learn more",
	}, map[string]interface{}{
		"CommandPrefix": sett.CommandPrefix,
	})

	if premium != storage.FreeTier {
		extraDesc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.Premium",
			Other: "Showing additional Premium Stats!",
		})
		gid, err := strconv.ParseUint(guildID, 10, 64)
		if err == nil {
			totalGameRankings := bot.PostgresInterface.TotalGamesRankingForServer(gid)

			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(totalGameRankings) && i < LeaderboardSize; i++ {
				elem := totalGameRankings[i]
				buf.WriteString(fmt.Sprintf("<@%d> | %d Games", elem.Mode, elem.Count))
				if i < len(totalGameRankings)-1 && i < LeaderboardSize-1 {
					buf.WriteByte('\n')
				}
			}
			if len(totalGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.MostGames",
						Other: "Games Played",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			crewmateGameRankings := bot.PostgresInterface.TotalWinRankingForServerByRole(gid, 0)
			buf = bytes.NewBuffer([]byte{})
			for i := 0; i < len(crewmateGameRankings) && i < LeaderboardSize; i++ {
				elem := crewmateGameRankings[i]
				buf.WriteString(fmt.Sprintf("<@%d> | %d Wins", elem.Mode, elem.Count))
				if i < len(crewmateGameRankings)-1 && i < LeaderboardSize-1 {
					buf.WriteByte('\n')
				}
			}
			if len(crewmateGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   "\u200b",
					Value:  "\u200b",
					Inline: true,
				})
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.CrewmateWins",
						Other: "Crewmate Wins",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			imposterGameRankings := bot.PostgresInterface.TotalWinRankingForServerByRole(gid, 1)
			buf = bytes.NewBuffer([]byte{})
			for i := 0; i < len(imposterGameRankings) && i < LeaderboardSize; i++ {
				elem := imposterGameRankings[i]
				buf.WriteString(fmt.Sprintf("<@%d> | %d Wins", elem.Mode, elem.Count))
				if i < len(imposterGameRankings)-1 && i < LeaderboardSize-1 {
					buf.WriteByte('\n')
				}
			}
			if len(imposterGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.ImposterWins",
						Other: "Imposter Wins",
					}),
					Value:  buf.String(),
					Inline: true,
				})
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   "\u200b",
					Value:  "\u200b",
					Inline: true,
				})
			}
		}
	}

	var embed = discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.Title",
			Other: "Guild Stats",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.Desc",
			Other: "Guild stats for {{.GuildName}}",
		}, map[string]interface{}{
			"GuildName": gname,
		}) + "\n\n" + extraDesc,
		Timestamp: "",
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      avatarUrl,
			ProxyURL: "",
			Width:    0,
			Height:   0,
		},
		Video:    nil,
		Provider: nil,
		Author:   nil,
		Fields:   fields,
	}
	return &embed
}
