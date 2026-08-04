package game

import (
	"testing"
	"time"
)

func newControlTestRoom(t *testing.T) (*Service, *Room) {
	t.Helper()
	service := NewService(testBank())
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

func TestSkipQuestionClearsAnswersAndRestartsCountdown(t *testing.T) {
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
	if room.CurrentIndex != 1 {
		t.Fatalf("expected question index 1, got %d", room.CurrentIndex)
	}
	if len(room.Submitted) != 0 {
		t.Fatalf("submitted state was not cleared: %#v", room.Submitted)
	}
	if _, exists := room.Answers["p1"][1]; exists {
		t.Fatal("skipped question answer was not deleted")
	}
	remaining := room.AnswerDeadline.Sub(room.UpdatedAt)
	if remaining < 59*time.Second || remaining > 61*time.Second {
		t.Fatalf("countdown was not reset: %v", remaining)
	}
}

func TestForceStartGuessingAndFinishWhenAllSubmit(t *testing.T) {
	service, room := newControlTestRoom(t)
	event, err := service.ForceStartGuessing(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "GUESSING_STARTED" {
		t.Fatalf("expected GUESSING_STARTED, got %s", event.Type)
	}

	room.Mu.Lock()
	aliasP1 := room.AliasByPlayer["p1"]
	aliasP2 := room.AliasByPlayer["p2"]
	room.Mu.Unlock()

	if _, finished, err := service.SubmitGuesses(room.ID, "p1", map[string]string{aliasP2: "p2"}); err != nil || finished != nil {
		t.Fatalf("first guess submission failed or finished early: event=%v err=%v", finished, err)
	}
	_, finished, err := service.SubmitGuesses(room.ID, "p2", map[string]string{aliasP1: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if finished == nil || finished.Type != "GAME_FINISHED" {
		t.Fatalf("expected GAME_FINISHED, got %#v", finished)
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
