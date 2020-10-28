package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bsm/redislock"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

var ctx = context.Background()

const LockTimeoutSecs = 5

//for TTL of Discord Game State (shouldn't need to last more than 12 hours)
const SecsIn12Hrs = 43200

type RedisInterface struct {
	client *redis.Client
}

func (redisInterface *RedisInterface) Init(params interface{}) error {
	redisParams := params.(storage.RedisParameters)
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisParams.Addr,
		Username: redisParams.Username,
		Password: redisParams.Password,
		DB:       0, // use default DB
	})
	redisInterface.client = rdb
	return nil
}

func lobbyUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:lobby"
}

func phaseUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:phase"
}

func playerUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:player"
}

func connectUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:connect"
}

func gameKey(connCode string) string {
	return "automuteus:game:" + connCode
}

func discordKeyTextChannelPtr(guildID, channelID string) string {
	return "automuteus:discord:" + guildID + ":pointer:text:" + channelID
}

func discordKeyVoiceChannelPtr(guildID, channelID string) string {
	return "automuteus:discord:" + guildID + ":pointer:voice:" + channelID
}

func discordKeyConnectCodePtr(guildID, code string) string {
	return "automuteus:discord:" + guildID + ":pointer:code:" + code
}

func discordKey(guildID, id string) string {
	return "automuteus:discord:" + guildID + ":" + id
}

func cacheHash(guildID string) string {
	return "automuteus:discord:" + guildID + ":cache"
}

func (redisInterface *RedisInterface) PublishLobbyUpdate(connectCode, lobbyJson string) {
	redisInterface.publish(lobbyUpdateKey(connectCode), lobbyJson)
}

func (redisInterface *RedisInterface) PublishPhaseUpdate(connectCode, phase string) {
	redisInterface.publish(phaseUpdateKey(connectCode), phase)
}

func (redisInterface *RedisInterface) PublishPlayerUpdate(connectCode, playerJson string) {
	redisInterface.publish(playerUpdateKey(connectCode), playerJson)
}

func (redisInterface *RedisInterface) PublishConnectUpdate(connectCode, connect string) {
	redisInterface.publish(connectUpdateKey(connectCode), connect)
}

func (redisInterface *RedisInterface) publish(topic, message string) {
	log.Printf("Publishing %s to %s\n", message, topic)
	err := redisInterface.client.Publish(ctx, topic, message).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) SubscribeToGame(connectCode string) (connection, lobby, phase, player *redis.PubSub) {
	connect := redisInterface.client.Subscribe(ctx, connectUpdateKey(connectCode))
	lob := redisInterface.client.Subscribe(ctx, lobbyUpdateKey(connectCode))
	phas := redisInterface.client.Subscribe(ctx, phaseUpdateKey(connectCode))
	play := redisInterface.client.Subscribe(ctx, playerUpdateKey(connectCode))

	return connect, lob, phas, play
}

//todo this can technically be a race condition? what happens if one of these is updated while we're fetching...
func (redisInterface *RedisInterface) getDiscordGameStateKey(guildID, textChannel, voiceChannel, connectCode string) string {
	key := redisInterface.CheckPointer(discordKeyConnectCodePtr(guildID, connectCode))
	if key == "" {
		key = redisInterface.CheckPointer(discordKeyTextChannelPtr(guildID, textChannel))
		if key == "" {
			key = redisInterface.CheckPointer(discordKeyVoiceChannelPtr(guildID, voiceChannel))
		}
	}
	return key
}

//need at least one of these fields to fetch
func (redisInterface *RedisInterface) GetReadOnlyDiscordGameState(guildID, textChannel, voiceChannel, connectCode string) *DiscordGameState {
	return redisInterface.getDiscordGameState(guildID, textChannel, voiceChannel, connectCode)
}

func (redisInterface *RedisInterface) GetDiscordGameStateAndLock(guildID, textChannel, voiceChannel, connectCode string) (*redislock.Lock, *DiscordGameState) {
	key := redisInterface.getDiscordGameStateKey(guildID, textChannel, voiceChannel, connectCode)
	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(key+":lock", LockTimeoutSecs*time.Second, nil)
	if err == redislock.ErrNotObtained {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Fatalln(err)
	}

	return lock, redisInterface.getDiscordGameState(guildID, textChannel, voiceChannel, connectCode)
}

func (redisInterface *RedisInterface) getDiscordGameState(guildID, textChannel, voiceChannel, connectCode string) *DiscordGameState {
	key := redisInterface.getDiscordGameStateKey(guildID, textChannel, voiceChannel, connectCode)

	jsonStr, err := redisInterface.client.Get(ctx, key).Result()
	if err == redis.Nil {
		dgs := NewDiscordGameState(guildID)
		//this is a little silly, but it works...
		dgs.ConnectCode = connectCode
		dgs.GameStateMsg.MessageChannelID = textChannel
		dgs.Tracking.ChannelID = voiceChannel
		redisInterface.SetDiscordGameState(dgs, nil)
		return dgs
	} else if err != nil {
		log.Println(err)
		return nil
	} else {
		dgs := DiscordGameState{}
		err := json.Unmarshal([]byte(jsonStr), &dgs)
		if err != nil {
			log.Println(err)
			return nil
		} else {
			return &dgs
		}
	}
}

func (redisInterface *RedisInterface) CheckPointer(pointer string) string {
	key, err := redisInterface.client.Get(ctx, pointer).Result()
	if err != nil {
		return ""
	} else {
		return key
	}
}

func (redisInterface *RedisInterface) SetDiscordGameState(data *DiscordGameState, lock *redislock.Lock) {
	if lock != nil {
		defer lock.Release()
	}
	if data == nil {
		return
	}

	key := redisInterface.getDiscordGameStateKey(data.GuildID, data.GameStateMsg.MessageChannelID, data.Tracking.ChannelID, data.ConnectCode)
	//connectCode is the 1 sole key we should ever rely on for tracking games. Because we generate it ourselves
	//randomly, it's unique to every single game, and the capture and bot BOTH agree on the linkage
	if key == "" && data.ConnectCode == "" {
		log.Println("SET: No key found in Redis for any of the text, voice, or connect codes!")
		return
	} else {
		key = discordKey(data.GuildID, data.ConnectCode)
	}

	jBytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return
	}

	//log.Printf("Setting %s to JSON", key)
	err = redisInterface.client.Set(ctx, key, jBytes, SecsIn12Hrs*time.Second).Err()
	if err != nil {
		log.Println(err)
	}

	if data.ConnectCode != "" {
		//log.Printf("Setting %s to %s", discordKeyConnectCodePtr(guildID, data.ConnectCode), key)
		err = redisInterface.client.Set(ctx, discordKeyConnectCodePtr(data.GuildID, data.ConnectCode), key, SecsIn12Hrs*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.Tracking.ChannelID != "" {
		//log.Printf("Setting %s to %s", discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyVoiceChannelPtr(data.GuildID, data.Tracking.ChannelID), key, SecsIn12Hrs*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.GameStateMsg.MessageChannelID != "" {
		//log.Printf("Setting %s to %s", discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyTextChannelPtr(data.GuildID, data.GameStateMsg.MessageChannelID), key, SecsIn12Hrs*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func (redisInterface *RedisInterface) DeleteDiscordGameState(dgs *DiscordGameState) {
	guildID := dgs.GuildID
	connCode := dgs.ConnectCode
	if guildID == "" || connCode == "" {
		log.Println("Can't delete DGS with null guildID or null ConnCode")
	}
	data := redisInterface.getDiscordGameState(guildID, "", "", connCode)
	key := discordKey(guildID, connCode)

	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(key+":lock", LockTimeoutSecs*time.Second, nil)
	if err == redislock.ErrNotObtained {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Fatalln(err)
	} else {
		defer lock.Release()
	}

	//delete all the pointers to the underlying -actual- discord data
	err = redisInterface.client.Del(ctx, discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisInterface.client.Del(ctx, discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisInterface.client.Del(ctx, discordKeyConnectCodePtr(guildID, data.ConnectCode)).Err()
	if err != nil {
		log.Println(err)
	}

	err = redisInterface.client.Del(ctx, key).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) GetUsernameOrUserIDMappings(guildID, key string) map[string]interface{} {
	cacheHash := cacheHash(guildID)

	value, err := redisInterface.client.HGet(ctx, cacheHash, key).Result()
	if err != nil {
		log.Println(err)
		return map[string]interface{}{}
	}
	var ret map[string]interface{}
	err = json.Unmarshal([]byte(value), &ret)
	if err != nil {
		log.Println(err)
		return map[string]interface{}{}
	}

	log.Println(ret)
	return ret
}

func (redisInterface *RedisInterface) AddUsernameLink(guildID, userID, userName string) error {
	err := redisInterface.appendToHashedEntry(guildID, userID, userName)
	if err != nil {
		return err
	}
	return redisInterface.appendToHashedEntry(guildID, userName, userID)
}

func (redisInterface *RedisInterface) DeleteLinksByUserID(guildID, userID string) error {

	//over all the usernames associated with just this userID, delete the underlying mapping of username->userID
	usernames := redisInterface.GetUsernameOrUserIDMappings(guildID, userID)
	for username := range usernames {
		err := redisInterface.deleteHashSubEntry(guildID, username, userID)
		if err != nil {
			log.Println(err)
		}
	}

	//now delete the userID->username list entirely
	cacheHash := cacheHash(guildID)
	return redisInterface.client.HDel(ctx, cacheHash, userID).Err()
}

func (redisInterface *RedisInterface) appendToHashedEntry(guildID, key, value string) error {
	resp := redisInterface.GetUsernameOrUserIDMappings(guildID, key)

	resp[value] = struct{}{}

	return redisInterface.setUsernameOrUserIDMappings(guildID, key, resp)
}

func (redisInterface *RedisInterface) deleteHashSubEntry(guildID, key, entry string) error {
	entries := redisInterface.GetUsernameOrUserIDMappings(guildID, key)

	delete(entries, entry)
	return redisInterface.setUsernameOrUserIDMappings(guildID, key, entries)
}

func (redisInterface *RedisInterface) setUsernameOrUserIDMappings(guildID, key string, values map[string]interface{}) error {
	cacheHash := cacheHash(guildID)

	jBytes, err := json.Marshal(values)
	if err != nil {
		return err
	}

	return redisInterface.client.HSet(ctx, cacheHash, key, jBytes).Err()
}

func (redisInterface *RedisInterface) Close() error {
	return redisInterface.client.Close()
}
