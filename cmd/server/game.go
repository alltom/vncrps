package main

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"
)

type GameServer struct {
	lock   sync.Mutex
	getNow func() time.Time

	nextPlayerId int
	players      map[PlayerId]*PlayerInfo
	matchups     []*Matchup

	phase         Phase
	phaseDeadline time.Time
}

type Matchup struct {
	Players [2]PlayerId
	Moves   [2]*Move
	Winner  *PlayerId
}

type Move int

const (
	MoveRock Move = iota
	MovePaper
	MoveScissors
)

func (m Move) Beats(m2 Move) bool {
	switch m {
	case MoveRock:
		return m2 == MoveScissors
	case MovePaper:
		return m2 == MoveRock
	case MoveScissors:
		return m2 == MovePaper
	default:
		panic(fmt.Sprintf("unrecognized move: %v", m))
	}
}

func (m Move) String() string {
	switch m {
	case MoveRock:
		return "ROCK"
	case MovePaper:
		return "PAPER"
	case MoveScissors:
		return "SCISSORS"
	default:
		panic(fmt.Sprintf("unrecognized move: %v", m))
	}
}

type PlayerId int64

type PlayerInfo struct {
	PlayerId     PlayerId
	Disconnected bool
	Name         string
	Rank         int
}

type Phase int

const (
	PhaseWaiting Phase = iota
	PhasePicking
	PhaseReview
)

type GameState struct {
	Player          PlayerInfo
	Phase           Phase
	TimeLeftInPhase time.Duration

	PlayerMove   *Move
	Opponent     *PlayerInfo
	OpponentMove *Move
	Winner       *PlayerId

	Rankings []PlayerInfo
}

func NewGameServer(getNow func() time.Time) *GameServer {
	s := &GameServer{getNow: getNow, nextPlayerId: 1}
	s.players = make(map[PlayerId]*PlayerInfo)
	return s
}

func (s *GameServer) AddPlayer() PlayerId {
	s.lock.Lock()
	defer s.lock.Unlock()

	player := &PlayerInfo{
		PlayerId: PlayerId(s.nextPlayerId),
		Name:     fmt.Sprintf("P%d", s.nextPlayerId),
	}
	s.nextPlayerId++
	s.players[player.PlayerId] = player

	if s.phase == PhaseWaiting && len(s.players) >= 2 {
		s.startRound(s.getNow())
	}

	active, total := s.playerCount()
	log.Printf("player %d connected (%d players active, %d total)", player.PlayerId, active, total)

	return player.PlayerId
}

func (s *GameServer) RemovePlayer(playerId PlayerId) {
	s.lock.Lock()
	defer s.lock.Unlock()

	active, total := s.playerCount()
	log.Printf("player %d disconnected (%d players active, %d total)", playerId, active, total)

	if s.phase == PhaseWaiting {
		delete(s.players, playerId)
	} else {
		if player, ok := s.players[playerId]; ok {
			player.Disconnected = true
		}
	}
}

func (s *GameServer) GetState(playerId PlayerId) (*GameState, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	now := s.getNow()

	// Make time-based state transitions.
	switch s.phase {
	case PhaseWaiting:
	case PhasePicking:
		if now.After(s.phaseDeadline) {
			s.judge()
			s.phase = PhaseReview
			s.phaseDeadline = now.Add(time.Second * 5)
		}
	case PhaseReview:
		if now.After(s.phaseDeadline) {
			s.resetPlayers()
			if len(s.players) >= 2 {
				s.startRound(now)
			} else {
				s.matchups = nil
				s.phase = PhaseWaiting
			}
		}
	}

	player, ok := s.players[playerId]
	if !ok {
		return nil, fmt.Errorf("could not find player with id %v", playerId)
	}

	timeLeft := time.Duration(0)
	if s.phase != PhaseWaiting {
		timeLeft = s.phaseDeadline.Sub(now)
	}

	var playerMove *Move
	var opponent *PlayerInfo
	var opponentMove *Move
	var winner *PlayerId
	for _, m := range s.matchups {
		// For cloning.
		var opp PlayerInfo
		var pmove, oppmove Move
		var w PlayerId

		if m.Players[0] == playerId {
			if m.Moves[0] != nil {
				pmove = *m.Moves[0]
				playerMove = &pmove
			}

			if o, ok := s.players[m.Players[1]]; ok {
				opp = *o
				opponent = &opp

				if m.Moves[1] != nil {
					oppmove = *m.Moves[1]
					opponentMove = &oppmove
				}
			} else {
				log.Printf("player %d is in matchup but not player map", m.Players[1])
			}

			if m.Winner != nil {
				w = *m.Winner
				winner = &w
			}
			break
		} else if m.Players[1] == playerId {
			if m.Moves[1] != nil {
				pmove = *m.Moves[1]
				playerMove = &pmove
			}

			if o, ok := s.players[m.Players[0]]; ok {
				opp = *o
				opponent = &opp

				if m.Moves[0] != nil {
					oppmove = *m.Moves[0]
					opponentMove = &oppmove
				}
			} else {
				log.Printf("player %d is in matchup but not player map", m.Players[0])
			}

			if m.Winner != nil {
				w = *m.Winner
				winner = &w
			}
			break
		}
	}

	var rankings []PlayerInfo
	for _, player := range s.players {
		rankings = append(rankings, *player)
	}
	sort.Slice(rankings, func(i, j int) bool { return rankings[i].PlayerId < rankings[j].PlayerId })
	sort.SliceStable(rankings, func(i, j int) bool { return rankings[j].Rank < rankings[i].Rank })

	state := &GameState{
		Player:          *player,
		Phase:           s.phase,
		TimeLeftInPhase: timeLeft,
		PlayerMove:      playerMove,
		Opponent:        opponent,
		OpponentMove:    opponentMove,
		Winner:          winner,
		Rankings:        rankings,
	}

	return state, nil
}

func (s *GameServer) Pick(playerId PlayerId, move Move) {
	for _, m := range s.matchups {
		if m.Players[0] == playerId {
			m.Moves[0] = &move
			return
		} else if m.Players[1] == playerId {
			m.Moves[1] = &move
			return
		}
	}
}

// Assumes s.lock has been obtained.
func (s *GameServer) recordWin(winnerId, loserId PlayerId) {
	for _, player := range s.players {
		if player.PlayerId == winnerId {
			player.Rank++
			return
		}
	}
}

// Assumes s.lock has been obtained.
func (s *GameServer) recordDraw(playerId1, playerId2 PlayerId) {
}

func (s *GameServer) resetPlayers() {
	for id, player := range s.players {
		if player.Disconnected {
			delete(s.players, id)
		}
	}
}

// Assumes s.lock has been obtained.
func (s *GameServer) startRound(now time.Time) {
	var ids []PlayerId
	for id := range s.players {
		ids = append(ids, id)
	}
	rand.Shuffle(len(ids), func(i, j int) {
		ids[i], ids[j] = ids[j], ids[i]
	})

	s.matchups = nil
	for i := 0; i < len(ids)-1; i += 2 {
		s.matchups = append(s.matchups, &Matchup{
			Players: [2]PlayerId{ids[i], ids[i+1]},
		})
	}

	s.phase = PhasePicking
	s.phaseDeadline = now.Add(time.Second * 10)
}

// Assumes s.lock has been obtained.
func (s *GameServer) judge() {
	var winner PlayerId
	for _, m := range s.matchups {
		if _, ok := s.players[m.Players[0]]; ok && m.Moves[0] != nil {
			if _, ok := s.players[m.Players[1]]; ok && m.Moves[1] != nil {
				if m.Moves[0].Beats(*m.Moves[1]) {
					winner = m.Players[0]
					m.Winner = &winner
					s.recordWin(m.Players[0], m.Players[1])
				} else if m.Moves[1].Beats(*m.Moves[0]) {
					winner = m.Players[1]
					m.Winner = &winner
					s.recordWin(m.Players[1], m.Players[0])
				} else {
					s.recordDraw(m.Players[0], m.Players[1])
				}
			} else {
				// TODO: Need player2's rank.
				winner = m.Players[0]
				m.Winner = &winner
				s.recordWin(m.Players[0], m.Players[1])
			}
		} else {
			if _, ok := s.players[m.Players[1]]; ok && m.Moves[1] != nil {
				// TODO: Need player1's rank.
				winner = m.Players[1]
				m.Winner = &winner
				s.recordWin(m.Players[1], m.Players[0])
			} else {
				s.recordDraw(m.Players[0], m.Players[1])
			}
		}
	}
}

// Assumes s.lock has been obtained.
func (s *GameServer) playerCount() (int, int) {
	var active, total int
	for _, player := range s.players {
		if !player.Disconnected {
			active++
		}
		total++
	}
	return active, total
}
