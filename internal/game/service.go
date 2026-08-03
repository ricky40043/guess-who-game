package game

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrRoomNotFound   = errors.New("找不到房間")
	ErrGameStarted    = errors.New("遊戲已開始，無法加入")
	ErrNameRequired   = errors.New("請輸入暱稱")
	ErrNameTaken      = errors.New("這個暱稱已經有人使用")
	ErrNotHost        = errors.New("只有房主可以執行")
	ErrInvalidState   = errors.New("目前遊戲階段不允許這個操作")
	ErrPlayerNotFound = errors.New("找不到玩家")
)

type Event struct {
	Type    string
	Payload map[string]any
	Seq     int
}

type Service struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	bank  []Question
	byID  map[int]Question
}

func NewService(bank []Question) *Service {
	byID := make(map[int]Question, len(bank))
	for _, q := range bank {
		byID[q.ID] = q
	}
	return &Service{rooms: make(map[string]*Room), bank: bank, byID: byID}
}

func randomHex(bytes int) string {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func randomRoomID() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buffer := make([]byte, 6)
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return strings.ToUpper(randomHex(3))
	}
	for i := range buffer {
		buffer[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(buffer)
}

func normalizeSettings(input Settings) Settings {
	if input.QuestionCount < 1 || input.QuestionCount > 20 {
		input.QuestionCount = 5
	}
	if input.AnswerSeconds < 15 || input.AnswerSeconds > 300 {
		input.AnswerSeconds = 60
	}
	if input.GuessSeconds < 30 || input.GuessSeconds > 600 {
		input.GuessSeconds = 120
	}
	if input.QuestionMode != "custom" {
		input.QuestionMode = "random"
	}
	return input
}

func (s *Service) CreateRoom(hostClientID string, settings Settings) *Room {
	settings = normalizeSettings(settings)
	for {
		id := randomRoomID()
		s.mu.Lock()
		if _, exists := s.rooms[id]; exists {
			s.mu.Unlock()
			continue
		}
		now := time.Now()
		room := &Room{
			ID:               id,
			HostClientID:     hostClientID,
			HostToken:        randomHex(24),
			HostConnected:    true,
			Status:           StatusWaiting,
			Settings:         settings,
			Players:          make(map[string]*Player),
			CurrentIndex:     -1,
			Answers:          make(map[string]map[int]string),
			Submitted:        make(map[string]bool),
			AliasByPlayer:    make(map[string]string),
			PlayerByAlias:    make(map[string]string),
			Guesses:          make(map[string]map[string]string),
			GuessSubmittedAt: make(map[string]time.Time),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		s.rooms[id] = room
		s.mu.Unlock()
		return room
	}
}

func (s *Service) GetRoom(id string) (*Room, error) {
	s.mu.RLock()
	room, ok := s.rooms[strings.ToUpper(strings.TrimSpace(id))]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func (s *Service) DeleteRoom(id string) {
	s.mu.Lock()
	delete(s.rooms, id)
	s.mu.Unlock()
}

func (s *Service) JoinRoom(roomID, playerID, name string) (*Player, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 12 {
		return nil, ErrNameRequired
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusWaiting {
		return nil, ErrGameStarted
	}
	for _, existing := range room.Players {
		if strings.EqualFold(existing.Name, name) {
			return nil, ErrNameTaken
		}
	}
	player := &Player{ID: playerID, Name: name, Connected: true, JoinedAt: time.Now()}
	room.Players[playerID] = player
	room.Answers[playerID] = make(map[int]string)
	room.UpdatedAt = time.Now()
	copyPlayer := *player
	return &copyPlayer, nil
}

func (s *Service) SetPlayerConnected(roomID, playerID string, connected bool) (*Player, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	player, ok := room.Players[playerID]
	if !ok {
		return nil, ErrPlayerNotFound
	}
	player.Connected = connected
	room.UpdatedAt = time.Now()
	copyPlayer := *player
	return &copyPlayer, nil
}

func (s *Service) ReconnectHost(roomID, token, newClientID string) (*Room, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if token == "" || token != room.HostToken {
		return nil, ErrNotHost
	}
	room.HostClientID = newClientID
	room.HostConnected = true
	room.UpdatedAt = time.Now()
	return room, nil
}

func (s *Service) SetHostConnected(roomID string, connected bool) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return
	}
	room.Mu.Lock()
	room.HostConnected = connected
	room.UpdatedAt = time.Now()
	room.Mu.Unlock()
}

func (s *Service) UpdateSettings(roomID string, settings Settings) (Settings, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return Settings{}, err
	}
	settings = normalizeSettings(settings)
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusWaiting {
		return Settings{}, ErrInvalidState
	}
	room.Settings = settings
	room.UpdatedAt = time.Now()
	return room.Settings, nil
}

func (s *Service) buildQuestions(settings Settings) ([]Question, error) {
	if settings.QuestionMode == "custom" {
		selected := make([]Question, 0, len(settings.QuestionIDs)+len(settings.CustomTexts))
		seen := make(map[int]bool)
		for _, id := range settings.QuestionIDs {
			if seen[id] {
				continue
			}
			if q, ok := s.byID[id]; ok {
				seen[id] = true
				selected = append(selected, q)
			}
		}
		customID := 9000
		for _, text := range settings.CustomTexts {
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			selected = append(selected, Question{ID: customID, Text: text, Category: "自訂題目", Custom: true})
			customID++
		}
		if len(selected) == 0 {
			return nil, errors.New("請至少選擇或自訂一題")
		}
		if len(selected) > 20 {
			selected = selected[:20]
		}
		return selected, nil
	}

	count := settings.QuestionCount
	if count > len(s.bank) {
		count = len(s.bank)
	}
	pool := append([]Question(nil), s.bank...)
	mathrand.New(mathrand.NewSource(time.Now().UnixNano())).Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	return pool[:count], nil
}

func (s *Service) StartGame(roomID string) (*Event, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusWaiting {
		return nil, ErrInvalidState
	}
	if len(room.Players) < 2 {
		return nil, errors.New("至少需要 2 位玩家")
	}
	selected, err := s.buildQuestions(room.Settings)
	if err != nil {
		return nil, err
	}
	room.Questions = selected
	room.CurrentIndex = 0
	room.Status = StatusAnswering
	room.Submitted = make(map[string]bool)
	room.AnswerDeadline = time.Now().Add(time.Duration(room.Settings.AnswerSeconds) * time.Second)
	room.Seq++
	room.UpdatedAt = time.Now()
	return &Event{Type: "QUESTION_STARTED", Seq: room.Seq, Payload: questionPayloadLocked(room)}, nil
}

func questionPayloadLocked(room *Room) map[string]any {
	question := room.Questions[room.CurrentIndex]
	return map[string]any{
		"roomId":         room.ID,
		"question":       question,
		"questionIndex":  room.CurrentIndex,
		"questionNumber": room.CurrentIndex + 1,
		"totalQuestions": len(room.Questions),
		"deadlineAt":     room.AnswerDeadline.UnixMilli(),
		"answerSeconds":  room.Settings.AnswerSeconds,
		"submittedCount": len(room.Submitted),
		"playerCount":    len(room.Players),
	}
}

func (s *Service) SubmitAnswer(roomID, playerID string, questionIndex int, answer string) (map[string]any, *Event, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, nil, err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil, nil, errors.New("答案不能空白")
	}
	if len([]rune(answer)) > 200 {
		return nil, nil, errors.New("答案最多 200 字")
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusAnswering || room.CurrentIndex != questionIndex || time.Now().After(room.AnswerDeadline) {
		return nil, nil, ErrInvalidState
	}
	if _, ok := room.Players[playerID]; !ok {
		return nil, nil, ErrPlayerNotFound
	}
	question := room.Questions[room.CurrentIndex]
	if room.Answers[playerID] == nil {
		room.Answers[playerID] = make(map[int]string)
	}
	room.Answers[playerID][question.ID] = answer
	room.Submitted[playerID] = true
	room.UpdatedAt = time.Now()
	progress := map[string]any{
		"submittedCount": len(room.Submitted),
		"playerCount":    len(room.Players),
		"playerId":       playerID,
	}
	if len(room.Submitted) < len(room.Players) {
		return progress, nil, nil
	}
	event := s.advanceQuestionLocked(room)
	return progress, event, nil
}

func (s *Service) advanceQuestionLocked(room *Room) *Event {
	if room.CurrentIndex+1 < len(room.Questions) {
		room.CurrentIndex++
		room.Submitted = make(map[string]bool)
		room.AnswerDeadline = time.Now().Add(time.Duration(room.Settings.AnswerSeconds) * time.Second)
		room.Seq++
		room.UpdatedAt = time.Now()
		return &Event{Type: "QUESTION_STARTED", Seq: room.Seq, Payload: questionPayloadLocked(room)}
	}
	return s.beginRevealLocked(room)
}

func aliasFor(index int) string {
	letter := string(rune('A' + index%26))
	if index < 26 {
		return "同學 " + letter
	}
	return fmt.Sprintf("同學 %s%d", letter, index/26+1)
}

func (s *Service) beginRevealLocked(room *Room) *Event {
	room.Status = StatusRevealing
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
	room.RevealIndex = 0
	room.Seq++
	room.UpdatedAt = time.Now()
	return &Event{Type: "REVEAL_STARTED", Seq: room.Seq, Payload: revealPayloadLocked(room, 0)}
}

func revealPayloadLocked(room *Room, index int) map[string]any {
	playerID := room.RevealOrder[index]
	items := make([]map[string]any, 0, len(room.Questions))
	for _, q := range room.Questions {
		answer := strings.TrimSpace(room.Answers[playerID][q.ID])
		if answer == "" {
			answer = "未作答"
		}
		items = append(items, map[string]any{"question": q.Text, "answer": answer})
	}
	return map[string]any{
		"profile": map[string]any{
			"alias":   room.AliasByPlayer[playerID],
			"answers": items,
		},
		"revealNumber":  index + 1,
		"totalProfiles": len(room.RevealOrder),
		"isLast":        index == len(room.RevealOrder)-1,
	}
}

func (s *Service) TickAnswer(roomID string, seq int) (int, *Event, bool) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return 0, nil, true
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusAnswering || room.Seq != seq {
		return 0, nil, true
	}
	remainingDuration := time.Until(room.AnswerDeadline)
	remaining := int((remainingDuration + time.Second - 1) / time.Second)
	if remaining > 0 {
		return remaining, nil, false
	}
	event := s.advanceQuestionLocked(room)
	return 0, event, false
}

func (s *Service) NextReveal(roomID string) (map[string]any, bool, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, false, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusRevealing {
		return nil, false, ErrInvalidState
	}
	room.RevealIndex++
	room.UpdatedAt = time.Now()
	if room.RevealIndex >= len(room.RevealOrder) {
		return map[string]any{"totalProfiles": len(room.RevealOrder)}, true, nil
	}
	return revealPayloadLocked(room, room.RevealIndex), false, nil
}

func guessingPayloadLocked(room *Room) map[string]any {
	aliases := make([]string, 0, len(room.PlayerByAlias))
	for alias := range room.PlayerByAlias {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	players := make([]map[string]string, 0, len(room.Players))
	for _, player := range room.PlayerList() {
		players = append(players, map[string]string{"id": player.ID, "name": player.Name})
	}
	return map[string]any{
		"aliases":        aliases,
		"players":        players,
		"deadlineAt":     room.GuessDeadline.UnixMilli(),
		"guessSeconds":   room.Settings.GuessSeconds,
		"submittedCount": len(room.Guesses),
		"playerCount":    len(room.Players),
	}
}

func (s *Service) StartGuessing(roomID string) (*Event, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusRevealing || room.RevealIndex < len(room.RevealOrder) {
		return nil, errors.New("請先公布完所有匿名答案")
	}
	room.Status = StatusGuessing
	room.Guesses = make(map[string]map[string]string)
	room.GuessSubmittedAt = make(map[string]time.Time)
	room.GuessDeadline = time.Now().Add(time.Duration(room.Settings.GuessSeconds) * time.Second)
	room.Seq++
	room.UpdatedAt = time.Now()
	return &Event{Type: "GUESSING_STARTED", Seq: room.Seq, Payload: guessingPayloadLocked(room)}, nil
}

func (s *Service) PersonalizedGuessInfo(roomID, playerID string) (map[string]any, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if _, ok := room.Players[playerID]; !ok {
		return nil, ErrPlayerNotFound
	}
	payload := guessingPayloadLocked(room)
	payload["ownAlias"] = room.AliasByPlayer[playerID]
	payload["ownPlayerId"] = playerID
	return payload, nil
}

func (s *Service) SubmitGuesses(roomID, playerID string, guesses map[string]string) (map[string]any, *Event, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusGuessing || time.Now().After(room.GuessDeadline) {
		return nil, nil, ErrInvalidState
	}
	if _, ok := room.Players[playerID]; !ok {
		return nil, nil, ErrPlayerNotFound
	}
	ownAlias := room.AliasByPlayer[playerID]
	expected := len(room.Players) - 1
	if len(guesses) != expected {
		return nil, nil, errors.New("請完成所有配對")
	}
	usedTargets := make(map[string]bool, expected)
	clean := make(map[string]string, expected)
	for alias, targetID := range guesses {
		if alias == ownAlias || room.PlayerByAlias[alias] == "" {
			return nil, nil, errors.New("配對資料包含無效匿名代號")
		}
		if targetID == playerID || room.Players[targetID] == nil || usedTargets[targetID] {
			return nil, nil, errors.New("每個名字只能配對一次，且不能選自己")
		}
		usedTargets[targetID] = true
		clean[alias] = targetID
	}
	now := time.Now()
	room.Guesses[playerID] = clean
	room.GuessSubmittedAt[playerID] = now
	room.UpdatedAt = now
	progress := map[string]any{
		"submittedCount": len(room.Guesses),
		"playerCount":    len(room.Players),
		"playerId":       playerID,
	}
	if len(room.Guesses) < len(room.Players) {
		return progress, nil, nil
	}
	event := s.finishGameLocked(room)
	return progress, event, nil
}

func (s *Service) TickGuess(roomID string, seq int) (int, *Event, bool) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return 0, nil, true
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if room.Status != StatusGuessing || room.Seq != seq {
		return 0, nil, true
	}
	remainingDuration := time.Until(room.GuessDeadline)
	remaining := int((remainingDuration + time.Second - 1) / time.Second)
	if remaining > 0 {
		return remaining, nil, false
	}
	return 0, s.finishGameLocked(room), false
}

func (s *Service) finishGameLocked(room *Room) *Event {
	results := make([]GuessResult, 0, len(room.Players))
	possible := len(room.Players) - 1
	for playerID, player := range room.Players {
		correct := 0
		for alias, targetID := range room.Guesses[playerID] {
			if room.PlayerByAlias[alias] == targetID {
				correct++
			}
		}
		bonus := 0
		if possible > 0 && correct == possible {
			bonus = 2
		}
		duration := room.Settings.GuessSeconds * 1000
		if submittedAt, ok := room.GuessSubmittedAt[playerID]; ok {
			duration = int(submittedAt.Sub(room.GuessDeadline.Add(-time.Duration(room.Settings.GuessSeconds) * time.Second)).Milliseconds())
			if duration < 0 {
				duration = 0
			}
		}
		player.GuessScore = correct
		player.PerfectBonus = bonus
		player.GuessDuration = int64(duration)
		results = append(results, GuessResult{
			PlayerID: playerID, PlayerName: player.Name, Correct: correct, Possible: possible,
			PerfectBonus: bonus, Score: correct + bonus, DurationMilli: int64(duration),
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Correct != results[j].Correct {
			return results[i].Correct > results[j].Correct
		}
		if results[i].DurationMilli != results[j].DurationMilli {
			return results[i].DurationMilli < results[j].DurationMilli
		}
		return results[i].PlayerName < results[j].PlayerName
	})
	for i := range results {
		results[i].Rank = i + 1
	}
	identities := make([]map[string]string, 0, len(room.PlayerByAlias))
	for alias, playerID := range room.PlayerByAlias {
		identities = append(identities, map[string]string{"alias": alias, "playerId": playerID, "playerName": room.Players[playerID].Name})
	}
	sort.Slice(identities, func(i, j int) bool { return identities[i]["alias"] < identities[j]["alias"] })
	room.Status = StatusFinished
	room.Results = results
	room.Seq++
	room.UpdatedAt = time.Now()
	return &Event{Type: "GAME_FINISHED", Seq: room.Seq, Payload: map[string]any{"results": results, "identities": identities}}
}

func (s *Service) PlayerList(roomID string) ([]*Player, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	return room.PlayerList(), nil
}

func (s *Service) Snapshot(roomID, playerID string, isHost bool) (map[string]any, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Mu.Lock()
	defer room.Mu.Unlock()
	payload := map[string]any{
		"roomId":        room.ID,
		"status":        room.Status,
		"settings":      room.Settings,
		"players":       room.PlayerList(),
		"isHost":        isHost,
		"hostConnected": room.HostConnected,
	}
	if !isHost {
		player, ok := room.Players[playerID]
		if !ok {
			return nil, ErrPlayerNotFound
		}
		payload["playerId"] = player.ID
		payload["playerName"] = player.Name
	}
	switch room.Status {
	case StatusAnswering:
		for key, value := range questionPayloadLocked(room) {
			payload[key] = value
		}
		if !isHost {
			question := room.Questions[room.CurrentIndex]
			payload["myAnswer"] = room.Answers[playerID][question.ID]
			payload["hasSubmitted"] = room.Submitted[playerID]
		}
	case StatusRevealing:
		if room.RevealIndex < len(room.RevealOrder) {
			payload["reveal"] = revealPayloadLocked(room, room.RevealIndex)
		} else {
			payload["revealComplete"] = true
		}
	case StatusGuessing:
		for key, value := range guessingPayloadLocked(room) {
			payload[key] = value
		}
		if !isHost {
			payload["ownAlias"] = room.AliasByPlayer[playerID]
			payload["ownPlayerId"] = playerID
			_, payload["hasGuessed"] = room.Guesses[playerID]
		}
	case StatusFinished:
		payload["results"] = room.Results
		identities := make([]map[string]string, 0, len(room.PlayerByAlias))
		for alias, id := range room.PlayerByAlias {
			identities = append(identities, map[string]string{"alias": alias, "playerId": id, "playerName": room.Players[id].Name})
		}
		sort.Slice(identities, func(i, j int) bool { return identities[i]["alias"] < identities[j]["alias"] })
		payload["identities"] = identities
	}
	return payload, nil
}
