package discord

import (
	"encoding/json"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
)

type EndGameType int

const (
	EndAndSave EndGameType = iota
	EndAndWipe
)

type EndGameMessage struct {
	EndGameType EndGameType
}

func (bot *Bot) SubscribeToGameByConnectCode(guildID, connectCode string, endGameChannel <-chan EndGameMessage) {
	log.Println("Started Redis Subscription worker")
	connection, lobby, phase, player := bot.RedisInterface.SubscribeToGame(connectCode)

	dgsRequest := GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connectCode,
	}
	for {
		select {
		case gameMessage := <-connection.Channel():
			log.Println(gameMessage)

			//tell the producer of the connection event that we got their message
			bot.RedisInterface.PublishConnectUpdateAck(connectCode)
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			for lock == nil {
				lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			}
			if gameMessage.Payload == "true" {
				dgs.Linked = true
			} else {
				dgs.Linked = false
			}
			dgs.ConnectCode = connectCode
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			sett := bot.StorageInterface.GetGuildSettings(guildID)
			bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, dgsRequest)

			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			break
		case gameMessage := <-lobby.Channel():

			var lobby game.Lobby
			err := json.Unmarshal([]byte(gameMessage.Payload), &lobby)
			if err != nil {
				log.Println(err)
				break
			}

			bot.processLobby(bot.SessionManager.GetPrimarySession(), lobby, dgsRequest)
			break
		case gameMessage := <-phase.Channel():
			var phase game.Phase
			err := json.Unmarshal([]byte(gameMessage.Payload), &phase)
			if err != nil {
				log.Println(err)
				break
			}
			bot.processTransition(phase, dgsRequest)
			break
		case gameMessage := <-player.Channel():
			sett := bot.StorageInterface.GetGuildSettings(guildID)
			var player game.Player
			err := json.Unmarshal([]byte(gameMessage.Payload), &player)
			if err != nil {
				log.Println(err)
				break
			}

			shouldHandleTracked := bot.processPlayer(sett, player, dgsRequest)
			if shouldHandleTracked {
				bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, dgsRequest)
			}

			break
		case k := <-endGameChannel:
			log.Println("Redis subscriber received kill signal, closing all pubsubs")
			err := connection.Close()
			if err != nil {
				log.Println(err)
			}
			err = lobby.Close()
			if err != nil {
				log.Println(err)
			}
			err = phase.Close()
			if err != nil {
				log.Println(err)
			}
			err = player.Close()
			if err != nil {
				log.Println(err)
			}

			if k.EndGameType == EndAndSave {
				go bot.gracefulShutdownWorker(guildID, connectCode)
			} else if k.EndGameType == EndAndWipe {
				bot.forceEndGame(dgsRequest)
			}

			return
		}
	}
}

func (bot *Bot) processPlayer(sett *storage.GuildSettings, player game.Player, dgsRequest GameStateRequest) bool {
	if player.Name != "" {
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
		for lock == nil {
			lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
		}

		defer bot.RedisInterface.SetDiscordGameState(dgs, lock)

		if player.Disconnected || player.Action == game.LEFT {
			log.Println("I detected that " + player.Name + " disconnected or left! " +
				"I'm removing their linked game data; they will need to relink")

			dgs.ClearPlayerDataByPlayerName(player.Name)
			dgs.AmongUsData.ClearPlayerData(player.Name)
			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			return true
		} else {
			updated, isAliveUpdated, data := dgs.AmongUsData.UpdatePlayer(player)

			if player.Action == game.JOINED {
				log.Println("Detected a player joined, refreshing User data mappings")
				paired := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if !paired {
					uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
					paired = dgs.AttemptPairingByUserIDs(data, uids)
				}

				dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
				return true
			} else if updated {
				paired := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if !paired {
					uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)

					paired = dgs.AttemptPairingByUserIDs(data, uids)
				}
				//log.Println("Player update received caused an update in cached state")
				if isAliveUpdated && dgs.AmongUsData.GetPhase() == game.TASKS {
					if sett.GetUnmuteDeadDuringTasks() {
						dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
						return true
					} else {
						log.Println("NOT updating the discord status message; would leak info")
						return false
					}
				} else {
					dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
					return true
				}
			} else {
				return false
				//No changes occurred; no reason to update
			}
		}
	}
	return false
}

func (bot *Bot) processTransition(phase game.Phase, dgsRequest GameStateRequest) {
	sett := bot.StorageInterface.GetGuildSettings(dgsRequest.GuildID)
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	}

	oldPhase := dgs.AmongUsData.UpdatePhase(phase)
	if oldPhase == phase {
		lock.Release()
		return
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	switch phase {
	case game.MENU:
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		go dgs.RemoveAllReactions(bot.SessionManager.GetPrimarySession())
		break
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.SessionManager, sett, delay, NoPriority, dgsRequest)

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		go dgs.AddAllReactions(bot.SessionManager.GetPrimarySession(), bot.StatusEmojis[true])
		break
	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		//when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}

		bot.handleTrackedMembers(bot.SessionManager, sett, delay, priority, dgsRequest)
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		break
	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.SessionManager, sett, delay, DeadPriority, dgsRequest)

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		break
	}
}

func (bot *Bot) processLobby(s *discordgo.Session, lobby game.Lobby, dgsRequest GameStateRequest) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	if lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	}
	dgs.AmongUsData.SetRoomRegion(lobby.LobbyCode, lobby.Region.ToString())
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	dgs.Edit(s, bot.gameStateResponse(dgs))
}
