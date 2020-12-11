package storage

import (
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/locale"
)

type GuildSettings struct {
	CommandPrefix string `json:"commandPrefix"`
	Language      string `json:"language"`

	AdminUserIDs             []string        `json:"adminIDs"`
	PermissionRoleIDs        []string        `json:"permissionRoleIDs"`
	Delays                   game.GameDelays `json:"delays"`
	VoiceRules               game.VoiceRules `json:"voiceRules"`
	UnmuteDeadDuringTasks    bool            `json:"unmuteDeadDuringTasks"`
	DeleteGameSummaryMinutes int             `json:"deleteGameSummary"`
	AutoRefresh              bool            `json:"autoRefresh"`
	MapVersion               string          `json:"mapVersion"`

	lock sync.RWMutex
}

func MakeGuildSettings() *GuildSettings {
	return &GuildSettings{
		CommandPrefix:            ".au",
		Language:                 locale.DefaultLang,
		AdminUserIDs:             []string{},
		PermissionRoleIDs:        []string{},
		Delays:                   game.MakeDefaultDelays(),
		VoiceRules:               game.MakeMuteAndDeafenRules(),
		UnmuteDeadDuringTasks:    false,
		DeleteGameSummaryMinutes: 0, //-1 for never delete the match summary
		AutoRefresh:              false,
		MapVersion:               "simple",
		lock:                     sync.RWMutex{},
	}
}

func (gs *GuildSettings) LocalizeMessage(args ...interface{}) string {
	args = append(args, gs.GetLanguage())
	return locale.LocalizeMessage(args...)
}

func (gs *GuildSettings) HasAdminPerms(user *discordgo.User) bool {
	if user == nil {
		return false
	}

	for _, v := range gs.AdminUserIDs {
		if v == user.ID {
			return true
		}
	}
	return false
}

func (gs *GuildSettings) HasRolePerms(mem *discordgo.Member) bool {
	for _, role := range mem.Roles {
		for _, testRole := range gs.PermissionRoleIDs {
			if testRole == role {
				return true
			}
		}
	}
	return false
}

func (gs *GuildSettings) GetCommandPrefix() string {
	return gs.CommandPrefix
}

func (gs *GuildSettings) SetCommandPrefix(p string) {
	gs.CommandPrefix = p
}

func (gs *GuildSettings) GetAdminUserIDs() []string {
	return gs.AdminUserIDs
}

func (gs *GuildSettings) SetAdminUserIDs(ids []string) {
	gs.AdminUserIDs = ids
}

func (gs *GuildSettings) GetPermissionRoleIDs() []string {
	return gs.PermissionRoleIDs
}

func (gs *GuildSettings) SetPermissionRoleIDs(ids []string) {
	gs.PermissionRoleIDs = ids
}

func (gs *GuildSettings) GetUnmuteDeadDuringTasks() bool {
	return gs.UnmuteDeadDuringTasks
}

func (gs *GuildSettings) GetDeleteGameSummaryMinutes() int {
	return gs.DeleteGameSummaryMinutes
}

func (gs *GuildSettings) SetDeleteGameSummaryMinutes(num int) {
	gs.DeleteGameSummaryMinutes = num
}

func (gs *GuildSettings) GetAutoRefresh() bool {
	return gs.AutoRefresh
}

func (gs *GuildSettings) SetAutoRefresh(n bool) {
	gs.AutoRefresh = n
}

func (gs *GuildSettings) GetMapVersion() string {
	if gs.MapVersion == "" {
		return "simple"
	} else {
		return gs.MapVersion
	}
}

func (gs *GuildSettings) SetMapVersion(n string) {
	gs.MapVersion = n
}

func (gs *GuildSettings) SetUnmuteDeadDuringTasks(v bool) {
	gs.UnmuteDeadDuringTasks = v
}

func (gs *GuildSettings) GetLanguage() string {
	return gs.Language
}

func (gs *GuildSettings) SetLanguage(l string) {
	gs.Language = l
}

func (gs *GuildSettings) GetDelay(oldPhase, newPhase game.Phase) int {
	return gs.Delays.GetDelay(oldPhase, newPhase)
}

func (gs *GuildSettings) SetDelay(oldPhase, newPhase game.Phase, v int) {
	gs.Delays.Delays[oldPhase.ToString()][newPhase.ToString()] = v
}

func (gs *GuildSettings) GetVoiceRule(isMute bool, phase game.Phase, alive string) bool {
	if isMute {
		return gs.VoiceRules.MuteRules[phase.ToString()][alive]
	} else {
		return gs.VoiceRules.DeafRules[phase.ToString()][alive]
	}
}

func (gs *GuildSettings) SetVoiceRule(isMute bool, phase game.Phase, alive string, val bool) {
	if isMute {
		gs.VoiceRules.MuteRules[phase.ToString()][alive] = val
	} else {
		gs.VoiceRules.DeafRules[phase.ToString()][alive] = val
	}
}

func (gs *GuildSettings) GetVoiceState(alive bool, tracked bool, phase game.Phase) (bool, bool) {
	return gs.VoiceRules.GetVoiceState(alive, tracked, phase)
}
