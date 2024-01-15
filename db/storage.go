package db

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shakinm/xlsReader/xls"
	"github.com/sirupsen/logrus"
)

type Storage struct {
	Words Words
	Users UserData
	mu    sync.Mutex
}

type UserData struct {
	Tasks map[int][]string
	Users map[int]User
}

func ReadStorage(wordsFile, usersFile string) (*Storage, error) {
	w := Storage{
		Words: Words{
			RusToSrb: make(map[string]string),
			SrbToRus: make(map[string]string),
			Indexes:  make(map[int]string),
		},
		Users: UserData{
			Tasks: make(map[int][]string),
			Users: map[int]User{},
		},
		mu: sync.Mutex{},
	}
	workbook, err := xls.OpenFile(wordsFile)
	if err != nil {
		return nil, err
	}
	sheet, err := workbook.GetSheet(0)
	if err != nil {
		return nil, err
	}
	for i := 0; i <= sheet.GetNumberRows()-1; i++ {
		if row, err := sheet.GetRow(i); err == nil {
			serbianRaw, err := row.GetCol(0)
			if err != nil {
				return nil, err
			}
			russianRaw, err := row.GetCol(1)
			if err != nil {
				return nil, err
			}
			w.Words.Indexes[i] = russianRaw.GetString()
			w.Words.SrbToRus[serbianRaw.GetString()] = russianRaw.GetString()
			w.Words.RusToSrb[russianRaw.GetString()] = serbianRaw.GetString()
		}
	}

	f, err := os.Open(usersFile)
	if err != nil {
		return &w, nil
	}

	b, err := io.ReadAll(f)
	if err != nil {
		fmt.Println(err)
		return &w, nil
	}
	u := UserData{}
	err = json.Unmarshal(b, &u)
	if err != nil {
		fmt.Println(err)
		return &w, nil
	}
	w.Users = u
	fmt.Println(w)
	return &w, nil
}

func (s *Storage) SaveToDisk(d time.Duration, filename string, l *logrus.Logger) {
	c := time.NewTicker(d)
	go func() {
		for range c.C {

			b, err := json.Marshal(s.Users)
			if err != nil {
				l.Error(err)
			}
			err = os.WriteFile(filename, b, 0644)
			if err != nil {
				l.Error(err)
			}

		}
	}()
}

func (s *Storage) CheckAndCreateUser(firstName, secondName string, id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.Users.Users[id]
	if ok {
		return
	}
	s.Users.Users[id] = User{
		Name:     firstName + " " + secondName,
		TaskSize: 10,
		History:  make([]bool, 0, 100),
	}
}

func (s *Storage) SetTaskSize(id, size int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	usr := s.Users.Users[id]
	usr.TaskSize = size
	s.Users.Users[id] = usr
}

func (s *Storage) CreateTask(id int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.Users.Users[id].TaskSize
	portion := make(map[int]struct{})
	for len(portion) != n {
		i := rand.Intn(len(s.Words.RusToSrb))
		portion[i] = struct{}{}
	}
	answer := make([]string, n)
	question := make([]string, n)
	i := 0
	for k, _ := range portion {
		rus := s.Words.Indexes[k]
		answer[i] = s.Words.RusToSrb[rus]
		question[i] = rus
		i++
	}
	s.Users.Tasks[id] = answer
	return question
}

func (s *Storage) CheckTask(id int, answers []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	answerList := s.Users.Tasks[id]
	corrects := 0
	words := ""
	resultsLog := make([]bool, len(answerList))
	for i, v := range answerList {
		etalon := removeSimbols(strings.TrimSpace(v))
		rus := s.Words.SrbToRus[v]
		if len(answers)-1 < i {
			words += fmt.Sprintf("%s - %s\n", rus, etalon)
			continue
		}
		a := answers[i]
		answer := removeSimbols(strings.TrimSpace(a))
		if answer == etalon {
			corrects++
			resultsLog[i] = true
		} else {
			words += fmt.Sprintf("%s - %s\n", rus, etalon)
		}
	}
	text := fmt.Sprintf("Правильные ответы - %d/%d\n", corrects, len(answerList))
	text += words
	uh := append(s.Users.Users[id].History, resultsLog...)
	u := s.Users.Users[id]
	u.History = uh
	s.Users.Users[id] = u
	delete(s.Users.Tasks, id)
	return text
}

func (s *Storage) HasTask(id int) bool {
	_, hasTask := s.Users.Tasks[id]
	return hasTask
}

func (s *Storage) IsOnLearn(id int) bool {
	if s.Users.Users[id].TaskSize == 1 {
		return true
	}
	return false
}

func (s *Storage) Rate(id int) int {
	answers := s.Users.Users[id].History
	t := 0
	for _, a := range answers {
		if a {
			t++
		}
	}
	p := t * 100 / len(answers)
	return p
}

func removeSimbols(s string) string {
	rs := make([]rune, 0, len(s))
	for _, r := range s {
		_, ok := dict[r]
		if !ok {
			rs = append(rs, r)
		}
	}
	return string(rs)
}

var (
	dict map[rune]struct{} = map[rune]struct{}{
		'-': {},
		',': {},
		'.': {},
		';': {},
		':': {},
		'!': {},
		'?': {},
		'(': {},
		')': {},
		'{': {},
		'}': {},
		'[': {},
		']': {},
	}
)
