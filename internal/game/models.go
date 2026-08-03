package game

import (
	"sort"
	"sync"
	"time"
)

type Status string

const (
	StatusWaiting   Status = "waiting"
	StatusAnswering Status = "answering"
	StatusRevealing Status = "revealing"
	StatusGuessing  Status = "guessing"
	StatusFinished  Status = "finished"
)

type Question struct {
	ID       int    `json:"id"`
	Text     string `json:"text"`
	Category string `json:"category"`
	Custom   bool   `json:"custom"`
}

type Settings struct {
	QuestionCount int      `json:"questionCount"`
	AnswerSeconds int      `json:"answerSeconds"`
	GuessSeconds  int      `json:"guessSeconds"`
	QuestionMode  string   `json:"questionMode"`
	QuestionIDs   []int    `json:"questionIds"`
	CustomTexts   []string `json:"customTexts"`
}

type Player struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Connected     bool      `json:"connected"`
	JoinedAt      time.Time `json:"joinedAt"`
	GuessScore    int       `json:"guessScore"`
	PerfectBonus  int       `json:"perfectBonus"`
	GuessDuration int64     `json:"guessDurationMs"`
}

type GuessResult struct {
	PlayerID      string `json:"playerId"`
	PlayerName    string `json:"playerName"`
	Correct       int    `json:"correct"`
	Possible      int    `json:"possible"`
	PerfectBonus  int    `json:"perfectBonus"`
	Score         int    `json:"score"`
	Rank          int    `json:"rank"`
	DurationMilli int64  `json:"durationMs"`
}

type RevealProfile struct {
	Alias   string            `json:"alias"`
	Answers map[string]string `json:"answers"`
}

type Room struct {
	Mu sync.Mutex `json:"-"`

	ID               string                         `json:"id"`
	HostClientID     string                         `json:"hostClientId"`
	HostToken        string                         `json:"hostToken"`
	HostConnected    bool                           `json:"hostConnected"`
	Status           Status                         `json:"status"`
	Settings         Settings                       `json:"settings"`
	Players          map[string]*Player             `json:"players"`
	Questions        []Question                     `json:"questions"`
	CurrentIndex     int                            `json:"currentIndex"`
	Answers          map[string]map[int]string      `json:"answers"`
	Submitted        map[string]bool                `json:"submitted"`
	AnswerDeadline   time.Time                      `json:"answerDeadline"`
	RevealOrder      []string                       `json:"revealOrder"`
	AliasByPlayer    map[string]string              `json:"aliasByPlayer"`
	PlayerByAlias    map[string]string              `json:"playerByAlias"`
	RevealIndex      int                            `json:"revealIndex"`
	Guesses          map[string]map[string]string   `json:"guesses"`
	GuessSubmittedAt map[string]time.Time           `json:"guessSubmittedAt"`
	GuessDeadline    time.Time                      `json:"guessDeadline"`
	Results          []GuessResult                  `json:"results"`
	Seq              int                            `json:"seq"`
	CreatedAt        time.Time                      `json:"createdAt"`
	UpdatedAt        time.Time                      `json:"updatedAt"`
}

func DefaultSettings() Settings {
	return Settings{
		QuestionCount: 5,
		AnswerSeconds: 60,
		GuessSeconds:  120,
		QuestionMode:  "random",
	}
}

func (r *Room) PlayerList() []*Player {
	list := make([]*Player, 0, len(r.Players))
	for _, player := range r.Players {
		copyPlayer := *player
		list = append(list, &copyPlayer)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].JoinedAt.Before(list[j].JoinedAt)
	})
	return list
}
