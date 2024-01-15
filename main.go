package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/edkliff/polako-bot/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	UserFile string `yaml:"userfile"`
	DataFile string `yaml:"datafile"`
	SaveTime string `yaml:"savetime"`
	APIKey   string `yaml:"apikey"`
}

func main() {
	l := logrus.New()
	confpath := flag.String("conf", "config.yml", "config path")
	cf, err := os.Open(*confpath)
	if err != nil {
		log.Fatal(err)
	}
	c := Config{}
	b, err := io.ReadAll(cf)
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		log.Fatal(err)
	}
	data, err := db.ReadStorage(c.DataFile, c.UserFile)
	if err != nil {
		l.Fatal(err)
	}

	fmt.Printf("DB.Users: %+v\n", data.Users)
	d, err := time.ParseDuration(c.SaveTime)
	if err != nil {
		l.Fatal(err)
	}
	data.SaveToDisk(d, c.UserFile, l)
	bot, err := tgbotapi.NewBotAPI(c.APIKey)
	if err != nil {
		l.Fatal(err)
	}

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			// log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
			id := int(update.Message.From.ID)
			data.CheckAndCreateUser(update.Message.From.FirstName, update.Message.From.LastName, id)
			textSplitted := strings.Split(strings.TrimSpace(update.Message.Text), " ")
			c := answer
			if len(textSplitted) != 0 {
				c = commands[textSplitted[0]]
			}
			text := ""
			switch c {
			case answer:
				switch data.HasTask(id) {
				case false:
					task := data.CreateTask(id)
					text += fmt.Sprintf("Переведи на сербский %d слов из нашего словаря (в столбик на латинице):\n\n", len(task))
					text += strings.Join(task, "\n")
				case true:
					resp := update.Message.Text
					respByWords := strings.Split(resp, "\n")
					text = data.CheckTask(id, respByWords)
					if data.IsOnLearn(id) {
						text += "\n\n"
						task := data.CreateTask(id)
						text += fmt.Sprintf("Переведи на сербский %d слов из нашего словаря (в столбик на латинице):\n\n", len(task))
						text += strings.Join(task, "\n")
					}
				}
			case help:
				text = `
					Данный бот отправляет вам несколько слов на русском, и в ответ ожидает те же слова на сербском.
					Ответ должен быть написан на латинице, регистр значим. Для глаголов, ожидается, что будет написан инфинитив и форма первого лица единственного числа.
					Каждый ответ в новой строке. Слова должны быть в том же порядке, в каком заданы.
					Примеры вопросов и ответов:
					- убираться, драить, успокаиваться - sređivati sređujem
					- снеговик - Sneško Belić
					- спортивного телосложения - sportski građena
					- ж. комната - sobа
					- Сербия (серб, сербка) - Srbija (Srbin Srpkinja)

					Команды:
					/help - вывод этой подсказки
					/set X - в одном задании выдавать Х слов 
					/learn или /set 1 - переводит бота в режим обучения. В этом режиме он будет отдавать только одно слово, и, после ответа, сразу спрашивать новое. Для возврата в обычный режим - /set X
					любая другая команда - возвращает упражнение для решения
					/rate - показывает процент правильных ответов на последних 100 словах
				`
			case set:
				size := 10
				if len(textSplitted) >= 2 {
					v := textSplitted[1]
					s, err := strconv.Atoi(v)
					if err == nil {
						size = s
					}
				}
				data.SetTaskSize(id, size)
			case learn:
				data.SetTaskSize(id, 1)
				if !data.HasTask(id) {
					task := data.CreateTask(id)
					text += fmt.Sprintf("Переведи на сербский %d слов из нашего словаря (в столбик на латинице):\n\n", len(task))
					text += strings.Join(task, "\n")
				}
			case rate:
				r := data.Rate(id)
				text = fmt.Sprintf("Ваш процент правильных ответов - %d%%!", r)
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
		}
	}
}

const (
	answer int = iota
	help
	set
	learn
	rate
)

var (
	commands = map[string]int{
		"/help":  help,
		"/set":   set,
		"/start": help,
		"/старт": help,
		"/learn": learn,
		"/rate":  rate,
	}
)
