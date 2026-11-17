package leaderboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const topN = 10

type Entry struct {
	Model string    `json:"model"`
	Score float64   `json:"score"`
	At    time.Time `json:"at"`
}

type Data struct {
	PPModel []Entry `json:"pp_model"`
	PPRun   []Entry `json:"pp_run"`
	TGModel []Entry `json:"tg_model"`
	TGRun   []Entry `json:"tg_run"`
}

type Ranked struct {
	Model string
	Score float64
	Tag   string
}

type Store struct {
	path string
	data Data
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "leaderboard.json"
	}
	return filepath.Join(home, ".local", "state", "hbtop", "leaderboard.json")
}

func Load(path string) *Store {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &s.data)
	}
	return s
}

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

func (s *Store) AddRun(model string, pp, tg float64) (ppRank, tgRank int) {
	now := time.Now().UTC()
	if pp > 0 {
		s.data.PPModel = addBest(s.data.PPModel, Entry{Model: model, Score: pp, At: now})
		s.data.PPRun = addRun(s.data.PPRun, Entry{Model: model, Score: pp, At: now})
	}
	if tg > 0 {
		s.data.TGModel = addBest(s.data.TGModel, Entry{Model: model, Score: tg, At: now})
		s.data.TGRun = addRun(s.data.TGRun, Entry{Model: model, Score: tg, At: now})
	}
	_ = s.Save()
	ppRank, tgRank = 0, 0
	if pp > 0 {
		ppRank = rankCombined(s.CombinedPP(), model, pp)
	}
	if tg > 0 {
		tgRank = rankCombined(s.CombinedTG(), model, tg)
	}
	return ppRank, tgRank
}

func (s *Store) PPModel() []Entry { return append([]Entry(nil), s.data.PPModel...) }
func (s *Store) PPRun() []Entry   { return append([]Entry(nil), s.data.PPRun...) }
func (s *Store) TGModel() []Entry { return append([]Entry(nil), s.data.TGModel...) }
func (s *Store) TGRun() []Entry   { return append([]Entry(nil), s.data.TGRun...) }

func (s *Store) CombinedPP() []Ranked {
	return combine(s.data.PPModel, s.data.PPRun)
}

func (s *Store) CombinedTG() []Ranked {
	return combine(s.data.TGModel, s.data.TGRun)
}

func combine(model, run []Entry) []Ranked {
	seen := map[string]string{}
	var out []Ranked
	add := func(items []Entry, tag string) {
		for _, e := range items {
			key := fmt.Sprintf("%s|%.6f", e.Model, e.Score)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = tag
			out = append(out, Ranked{Model: e.Model, Score: e.Score, Tag: tag})
		}
	}
	add(model, "M")
	add(run, "R")
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > topN {
		out = out[:topN]
	}
	return out
}

func addBest(list []Entry, e Entry) []Entry {
	for i := range list {
		if list[i].Model == e.Model {
			if e.Score > list[i].Score {
				list[i] = e
			}
			sort.SliceStable(list, func(i, j int) bool { return list[i].Score > list[j].Score })
			return list
		}
	}
	list = append(list, e)
	sort.SliceStable(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	if len(list) > topN {
		list = list[:topN]
	}
	return list
}

func addRun(list []Entry, e Entry) []Entry {
	list = append(list, e)
	sort.SliceStable(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	if len(list) > topN {
		list = list[:topN]
	}
	return list
}

func rankCombined(list []Ranked, model string, score float64) int {
	for i, e := range list {
		if e.Model == model && e.Score == score {
			return i + 1
		}
	}
	return 0
}
