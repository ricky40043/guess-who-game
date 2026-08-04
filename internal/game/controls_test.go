package game

import (
	"testing"
	"time"
)

func controlTestBank() []Question {
	return []Question{
		{ID: 1, Text: "第一題", Category: "測試"},
		{ID: 2, Text: "第二題", Category: "測試"},
		{ID: 3, Text: "替換題", Category: "測試"},
	}
}

func newControlTestRoom(t *testing.T) (*Service, *Room) {
	t.Helper()
	service := NewService(controlTestBank())
	room := service.CreateRoom("host", Settings{
		QuestionMode:  "custom",
		QuestionIDs:   []int{1, 2},
		AnswerSeconds: 60,
		GuessSeconds:  120,
	})
	for _, player := range []struct{ id, name string }{{"p1", "玩家一"}, {"p2", "玩家二"}} {
		if _, err := service.JoinRoom(room.ID, player.id, player.name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.StartGame(room.ID); err != nil {
		t.Fatal(err)
	}
	return service, room
}

func TestRoomCodeIsFourDigits(t *testing.T) {
	service := NewService(controlTestBank())
	room := service.CreateRoom("host", DefaultSettings())
	if len(room.ID) != 4 {
		t.Fatalf("expected 4 digit room code, got %q", room.ID)
	}
	for _, character := range room.ID {
		if character < '0' || character > '9' {
			t.Fatalf("room code must contain digits only: %q", room.ID)
		}
	}
}

func TestSkipQuestionReplacesCurrentQuestionAndRestartsCountdown(t *testing.T) {
	service, room := newControlTestRoom(t)
	if _, _, err := service.SubmitAnswer(room.ID, "p1", 0, "已作答"); err != nil {
		t.Fatal(err)
	}

	event, err := service.SkipQuestion(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "QUESTION_STARTED" {
		t.Fatalf("expected QUESTION_STARTED, got %s", event.Type)
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.CurrentIndex != 0 {
		t.Fatalf("question index should stay at 0, got %d", room.CurrentIndex)
	}
	if room.Questions[0].ID != 3 {
		t.Fatalf("current question was not replaced: %#v", room.Questions[0])
	}
	if len(room.Submitted) != 0 {
		t.Fatalf("submitted state was not cleared: %#v", room.Submitted)
	}
	if _, exists := room.Answers["p1"][1]; exists {
		t.Fatal("replaced question answer was not deleted")
	}
	remaining := room.AnswerDeadline.Sub(room.UpdatedAt)
	if remaining < 59*time.Second || remaining > 61*time.Second {
		t.Fatalf("countdown was not reset: %v", remaining)
	}
}

func TestForceStartGuessingActuallyStartsReveal(t *testing.T) {
	service, room := newControlTestRoom(t)
	if _, _, err := service.SubmitAnswer(room.ID, "p1", 0, "答案一"); err != nil {
		t.Fatal(err)
	}

	event, err := service.ForceStartGuessing(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "REVEAL_STARTED" {
		t.Fatalf("expected REVEAL_STARTED, got %s", event.Type)
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusRevealing {
		t.Fatalf("expected revealing status, got %s", room.Status)
	}
	if len(room.RevealOrder) != 2 {
		t.Fatalf("expected 2 reveal profiles, got %d", len(room.RevealOrder))
	}
	if room.RevealIndex != 0 {
		t.Fatalf("expected reveal index 0, got %d", room.RevealIndex)
	}
}

func TestResetToWaitingPreservesPlayersAndClearsRound(t *testing.T) {
	service, room := newControlTestRoom(t)
	if _, _, err := service.SubmitAnswer(room.ID, "p1", 0, "答案"); err != nil {
		t.Fatal(err)
	}

	event, err := service.ResetToWaiting(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "ROOM_RESET" {
		t.Fatalf("expected ROOM_RESET, got %s", event.Type)
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusWaiting {
		t.Fatalf("expected waiting, got %s", room.Status)
	}
	if len(room.Players) != 2 {
		t.Fatalf("players should be preserved: %d", len(room.Players))
	}
	if len(room.Questions) != 0 || len(room.Submitted) != 0 || len(room.Results) != 0 {
		t.Fatal("round data was not fully cleared")
	}
	for playerID := range room.Players {
		if len(room.Answers[playerID]) != 0 {
			t.Fatalf("answers for %s were not cleared", playerID)
		}
	}
}
