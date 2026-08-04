package game

import (
	"errors"
	mathrand "math/rand"
	"time"
)

// SkipQuestion 保留既有 WebSocket 指令名稱，但行為改為替換目前題目。
// 題號不變、目前題目的所有答案清除，並從完整秒數重新倒數。
func (s *Service) SkipQuestion(roomID string) (*Event, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusAnswering || room.CurrentIndex < 0 || room.CurrentIndex >= len(room.Questions) {
		return nil, ErrInvalidState
	}

	used := make(map[int]bool, len(room.Questions))
	for _, question := range room.Questions {
		used[question.ID] = true
	}
	candidates := make([]Question, 0, len(s.bank))
	for _, question := range s.bank {
		if !used[question.ID] {
			candidates = append(candidates, question)
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("題庫已沒有可替換的新題目")
	}

	oldQuestionID := room.Questions[room.CurrentIndex].ID
	random := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	room.Questions[room.CurrentIndex] = candidates[random.Intn(len(candidates))]
	for playerID := range room.Players {
		if room.Answers[playerID] != nil {
			delete(room.Answers[playerID], oldQuestionID)
		}
	}
	room.Submitted = make(map[string]bool)
	room.AnswerDeadline = time.Now().Add(time.Duration(room.Settings.AnswerSeconds) * time.Second)
	room.Seq++
	room.UpdatedAt = time.Now()
	return &Event{Type: "QUESTION_STARTED", Seq: room.Seq, Payload: questionPayloadLocked(room)}, nil
}

// ForceStartGuessing skips the remaining answering/reveal flow and starts guessing.
func (s *Service) ForceStartGuessing(roomID string) (*Event, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusAnswering && room.Status != StatusRevealing {
		return nil, ErrInvalidState
	}
	if len(room.Players) < 2 {
		return nil, ErrInvalidState
	}

	room.RevealOrder = room.RevealOrder[:0]
	for playerID := range room.Players {
		room.RevealOrder = append(room.RevealOrder, playerID)
	}
	mathrand.New(mathrand.NewSource(time.Now().UnixNano())).Shuffle(len(room.RevealOrder), func(i, j int) {
		room.RevealOrder[i], room.RevealOrder[j] = room.RevealOrder[j], room.RevealOrder[i]
	})
	room.AliasByPlayer = make(map[string]string, len(room.RevealOrder))
	room.PlayerByAlias = make(map[string]string, len(room.RevealOrder))
	for index, playerID := range room.RevealOrder {
		alias := aliasFor(index)
		room.AliasByPlayer[playerID] = alias
		room.PlayerByAlias[alias] = playerID
	}

	room.Status = StatusGuessing
	room.RevealIndex = len(room.RevealOrder)
	room.Guesses = make(map[string]map[string]string)
	room.GuessSubmittedAt = make(map[string]time.Time)
	room.GuessDeadline = time.Now().Add(time.Duration(room.Settings.GuessSeconds) * time.Second)
	room.Seq++
	room.UpdatedAt = time.Now()
	return &Event{Type: "GUESSING_STARTED", Seq: room.Seq, Payload: guessingPayloadLocked(room)}, nil
}

// ResetToWaiting ends the current round while preserving the room and players.
func (s *Service) ResetToWaiting(roomID string) (*Event, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()

	room.Status = StatusWaiting
	room.Questions = nil
	room.CurrentIndex = -1
	room.Answers = make(map[string]map[int]string, len(room.Players))
	for playerID, player := range room.Players {
		room.Answers[playerID] = make(map[int]string)
		player.GuessScore = 0
		player.PerfectBonus = 0
		player.GuessDuration = 0
	}
	room.Submitted = make(map[string]bool)
	room.AnswerDeadline = time.Time{}
	room.RevealOrder = nil
	room.AliasByPlayer = make(map[string]string)
	room.PlayerByAlias = make(map[string]string)
	room.RevealIndex = 0
	room.Guesses = make(map[string]map[string]string)
	room.GuessSubmittedAt = make(map[string]time.Time)
	room.GuessDeadline = time.Time{}
	room.Results = nil
	room.Seq++
	room.UpdatedAt = time.Now()

	return &Event{Type: "ROOM_RESET", Seq: room.Seq, Payload: map[string]any{
		"roomId":   room.ID,
		"status":   room.Status,
		"settings": room.Settings,
		"players":  room.PlayerList(),
	}}, nil
}
