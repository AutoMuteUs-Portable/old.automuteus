package discord

import (
	"container/heap"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"sync"
	"time"
)

type HandlePriority int

const (
	NoPriority    HandlePriority = 0
	AlivePriority HandlePriority = 1
	DeadPriority  HandlePriority = 2
)

type PrioritizedPatchParams struct {
	priority    int
	patchParams UserPatchParameters
}

type PatchPriority []PrioritizedPatchParams

func (h PatchPriority) Len() int { return len(h) }

//NOTE this is inversed so HIGHER numbers are pulled FIRST
func (h PatchPriority) Less(i, j int) bool { return h[i].priority > h[j].priority }
func (h PatchPriority) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *PatchPriority) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(PrioritizedPatchParams))
}

func (h *PatchPriority) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (dgs *DiscordGameState) verifyVoiceStateChanges(s *discordgo.Session, sett *storage.GuildSettings, phase game.Phase) *discordgo.Guild {
	g, err := s.State.Guild(dgs.GuildID)
	if err != nil {
		log.Println(err)
		return nil
	}

	for _, voiceState := range g.VoiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)

		if err != nil {
			//the User doesn't exist in our userdata cache; add them
			added := false
			userData, added = dgs.checkCacheAndAddUser(g, s, voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := voiceState.ChannelID != "" && dgs.Tracking.ChannelID == voiceState.ChannelID

		auData, linked := dgs.GetByName(userData.InGameName)
		//only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && linked
		mute, deaf := sett.GetVoiceState(auData.IsAlive, tracked, phase)

		//still have to check if the player is linked
		//(music bots are untracked so mute/deafen = false, but they dont have playerdata...)
		if linked && !userData.IsVoiceChangeReady() && voiceState.Mute == mute && voiceState.Deaf == deaf {
			userData.SetVoiceChangeReady(true)

			dgs.UpdateUserData(voiceState.UserID, userData)
		}
	}
	return g
}

//handleTrackedMembers moves/mutes players according to the current game state
func (dgs *DiscordGameState) handleTrackedMembers(sm *SessionManager, sett *storage.GuildSettings, delay int, handlePriority HandlePriority, phase game.Phase) {

	g := dgs.verifyVoiceStateChanges(sm.GetPrimarySession(), sett, phase)

	if g == nil {
		return
	}

	priorityQueue := &PatchPriority{}
	heap.Init(priorityQueue)

	for _, voiceState := range g.VoiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)
		if err != nil {
			//the User doesn't exist in our userdata cache; add them
			added := false
			userData, added = dgs.checkCacheAndAddUser(g, sm.GetPrimarySession(), voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := voiceState.ChannelID != "" && dgs.Tracking.ChannelID == voiceState.ChannelID

		auData, linked := dgs.GetByName(userData.InGameName)
		//only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && linked
		shouldMute, shouldDeaf := sett.GetVoiceState(auData.IsAlive, tracked, phase)

		nick := userData.GetPlayerName()
		if !sett.GetApplyNicknames() {
			nick = ""
		}

		//only issue a change if the User isn't in the right state already
		//nicksmatch can only be false if the in-game data is != nil, so the reference to .audata below is safe
		//check the userdata is linked here to not accidentally undeafen music bots, for example
		if linked && (shouldMute != voiceState.Mute || shouldDeaf != voiceState.Deaf || (nick != "" && userData.GetNickName() != userData.GetPlayerName())) {
			//only issue the req to discord if we're not waiting on another one
			if userData.IsVoiceChangeReady() {
				priority := 0

				if handlePriority != NoPriority {
					if handlePriority == AlivePriority && auData.IsAlive {
						priority++
					} else if handlePriority == DeadPriority && !auData.IsAlive {
						priority++
					}
				}

				params := UserPatchParameters{dgs.GuildID, userData, shouldDeaf, shouldMute, nick}

				heap.Push(priorityQueue, PrioritizedPatchParams{
					priority:    priority,
					patchParams: params,
				})
			}

		} else if linked {
			if shouldMute {
				log.Print(fmt.Sprintf("Not muting %s because they're already muted\n", userData.GetUserName()))
			} else {
				log.Print(fmt.Sprintf("Not unmuting %s because they're already unmuted\n", userData.GetUserName()))
			}
		}
	}
	wg := sync.WaitGroup{}
	waitForHigherPriority := false

	if delay > 0 {
		log.Printf("Sleeping for %d seconds before applying changes to users\n", delay)
		time.Sleep(time.Second * time.Duration(delay))
	}
	log.Printf("Mute queue length: %d", priorityQueue.Len())

	for priorityQueue.Len() > 0 {
		p := heap.Pop(priorityQueue).(PrioritizedPatchParams)

		if p.priority > 0 {
			waitForHigherPriority = true
			log.Print(fmt.Sprintf("User %s has higher priority: %d\n", p.patchParams.Userdata.GetID(), p.priority))
		} else if waitForHigherPriority {
			//wait for all the other users to get muted/unmuted completely, first
			//log.Println("Waiting for high priority User changes first")
			wg.Wait()
			waitForHigherPriority = false
		}

		wg.Add(1)

		//wait until it goes through
		p.patchParams.Userdata.SetVoiceChangeReady(false)

		dgs.UpdateUserData(p.patchParams.Userdata.GetID(), p.patchParams.Userdata)

		//we can issue mutes/deafens from ANY session, not just the primary
		go muteWorker(sm.GetSessionForRequest(p.patchParams.GuildID), &wg, p.patchParams)
	}
	wg.Wait()
}

func muteWorker(s *discordgo.Session, wg *sync.WaitGroup, parameters UserPatchParameters) {
	guildMemberUpdate(s, parameters)
	wg.Done()
}
