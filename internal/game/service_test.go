package game

import "testing"

func testBank() []Question {
	return []Question{
		{ID: 1, Text: "第一題", Category: "測試"},
		{ID: 2, Text: "第二題", Category: "測試"},
	}
}

func TestFullGameFlow(t *testing.T) {
	service := NewService(testBank())
	room := service.CreateRoom("host-1", Settings{
		QuestionMode:  "custom",
		QuestionIDs:   []int{1, 2},
		AnswerSeconds: 60,
		GuessSeconds:  120,
	})

	players := []struct{ id, name string }{{"p1", "小明"}, {"p2", "小美"}, {"p3", "阿華"}}
	for _, item := range players {
		if _, err := service.JoinRoom(room.ID, item.id, item.name); err != nil {
			t.Fatalf("join %s: %v", item.name, err)
		}
	}

	event, err := service.StartGame(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "QUESTION_STARTED" {
		t.Fatalf("unexpected start event: %s", event.Type)
	}

	for questionIndex := 0; questionIndex < 2; questionIndex++ {
		for playerIndex, item := range players {
			_, transition, err := service.SubmitAnswer(room.ID, item.id, questionIndex, item.name+"答案")
			if err != nil {
				t.Fatalf("submit answer: %v", err)
			}
			if playerIndex < len(players)-1 && transition != nil {
				t.Fatal("question transitioned before all players submitted")
			}
			if playerIndex == len(players)-1 {
				if questionIndex == 0 && transition.Type != "QUESTION_STARTED" {
					t.Fatalf("expected next question, got %v", transition)
				}
				if questionIndex == 1 && transition.Type != "REVEAL_STARTED" {
					t.Fatalf("expected reveal, got %v", transition)
				}
			}
		}
	}

	for index := 0; index < len(players); index++ {
		_, done, err := service.NextReveal(room.ID)
		if err != nil {
			t.Fatal(err)
		}
		if index < len(players)-1 && done {
			t.Fatal("reveal completed too early")
		}
		if index == len(players)-1 && !done {
			t.Fatal("reveal should be complete")
		}
	}

	if _, err := service.StartGuessing(room.ID); err != nil {
		t.Fatal(err)
	}

	room.Mu.Lock()
	aliasByPlayer := make(map[string]string, len(room.AliasByPlayer))
	playerByAlias := make(map[string]string, len(room.PlayerByAlias))
	for key, value := range room.AliasByPlayer {
		aliasByPlayer[key] = value
	}
	for key, value := range room.PlayerByAlias {
		playerByAlias[key] = value
	}
	room.Mu.Unlock()

	for index, item := range players {
		guesses := make(map[string]string)
		for alias, targetID := range playerByAlias {
			if alias == aliasByPlayer[item.id] {
				continue
			}
			guesses[alias] = targetID
		}
		_, finished, err := service.SubmitGuesses(room.ID, item.id, guesses)
		if err != nil {
			t.Fatalf("submit guesses: %v", err)
		}
		if index < len(players)-1 && finished != nil {
			t.Fatal("game finished too early")
		}
		if index == len(players)-1 {
			if finished == nil || finished.Type != "GAME_FINISHED" {
				t.Fatalf("expected game finish, got %v", finished)
			}
		}
	}

	snapshot, err := service.Snapshot(room.ID, "p1", false)
	if err != nil {
		t.Fatal(err)
	}
	results := snapshot["results"].([]GuessResult)
	if len(results) != 3 || results[0].Score != 4 {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestJoinRejectedAfterStart(t *testing.T) {
	service := NewService(testBank())
	room := service.CreateRoom("host", DefaultSettings())
	_, _ = service.JoinRoom(room.ID, "p1", "玩家一")
	_, _ = service.JoinRoom(room.ID, "p2", "玩家二")
	if _, err := service.StartGame(room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.JoinRoom(room.ID, "p3", "玩家三"); err != ErrGameStarted {
		t.Fatalf("expected ErrGameStarted, got %v", err)
	}
}
