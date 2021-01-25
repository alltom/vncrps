package main

import (
	"testing"
	"time"
)

func getState(s *GameServer, playerId PlayerId, t *testing.T) *GameState {
	state, err := s.GetState(playerId)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestBasic(t *testing.T) {
	now := time.Now()
	s := NewGameServer(func() time.Time { return now })

	p1 := s.AddPlayer()
	state := getState(s, p1, t)
	if state.Phase != PhaseWaiting {
		t.Fatalf("phase should be PhaseWaiting, but is %d", state.Phase)
	}

	p2 := s.AddPlayer()
	state = getState(s, p2, t)
	if state.Phase != PhasePicking {
		t.Fatalf("phase should be PhasePicking, but is %d", state.Phase)
	}
	if state.TimeLeftInPhase != time.Second*10 {
		t.Fatalf("time left should be 10 seconds, bu tit's %v", state.TimeLeftInPhase)
	}
	if state.Opponent.PlayerId != p1 {
		t.Fatalf("opponent should be %d, but it's %d", p1, state.Opponent.PlayerId)
	}
	if state.OpponentMove != nil {
		t.Fatalf("opponent move should be nil, but it's %v", state.OpponentMove)
	}

	s.Pick(p1, MoveRock)
	s.Pick(p2, MoveScissors)
	now = now.Add(time.Second * 11)

	state = getState(s, p1, t)
	if state.Player.PlayerId != p1 {
		t.Fatalf("player ID should be %d, but it's %d", p1, state.Player.PlayerId)
	}
	if state.Phase != PhaseReview {
		t.Fatalf("phase should be PhaseReview, but is %d", state.Phase)
	}
	if state.TimeLeftInPhase != time.Second*5 {
		t.Fatalf("time left should be 5 seconds, but it's %v", state.TimeLeftInPhase)
	}
	if state.Opponent.PlayerId != p2 {
		t.Fatalf("opponent should be %d, but it's %d", p2, state.Opponent.PlayerId)
	}
	if *state.OpponentMove != MoveScissors {
		t.Fatalf("opponent picked scissors, but recorded move is %v", *state.OpponentMove)
	}

	now = now.Add(time.Second)
	state = getState(s, p2, t)
	if state.Player.PlayerId != p2 {
		t.Fatalf("player ID should be %d, but it's %d", p2, state.Player.PlayerId)
	}
	if state.Phase != PhaseReview {
		t.Fatalf("phase should be PhaseReview, but is %d", state.Phase)
	}
	if state.TimeLeftInPhase != time.Second*4 {
		t.Fatalf("time left should be 4 seconds, but it's %v", state.TimeLeftInPhase)
	}
	if state.Opponent.PlayerId != p1 {
		t.Fatalf("opponent should be %d, but it's %d", p1, state.Opponent.PlayerId)
	}
	if *state.OpponentMove != MoveRock {
		t.Fatalf("opponent picked scissors, but recorded move is %v", *state.OpponentMove)
	}

	s.RemovePlayer(p2)
	state = getState(s, p1, t)
	if state.Phase != PhaseReview {
		t.Fatalf("phase should be PhaseReview, but is %d", state.Phase)
	}

	now = now.Add(time.Second * 5)
	state = getState(s, p1, t)
	if state.Phase != PhaseWaiting {
		t.Fatalf("phase should be PhaseWaiting, but is %d", state.Phase)
	}
}
