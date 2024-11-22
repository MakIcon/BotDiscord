package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	prefix     = "+"
	prefix2    = "-"
	prefix3    = "!"
	charset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-*&^%$#@!)(*/|"
	webhookURL = "https://canary.discord.com/api/webhooks/..."
)

var (
	emojis           = []string{":poop:", ":heart_eyes_cat:", ":firecracker:", ":leafy_green:", ":money_mouth:", ":imp:", ":wink:", ":pleading_face:", ":x:", ":woman_with_headscarf:", ":key:", ":champagne:", ":tada:", ":white_check_mark:", ":thumbsdown:", ":thumbsup:"}
	membersRep       = map[string]int{}
	allowedChannels  = map[string]bool{"1091008984913289307": true}
	blackList        = map[string]bool{}
	mu               sync.Mutex
	cooldowns        = make(map[string]map[string]time.Time)
	cooldownDuration = 60 * time.Second
)

func main() {
	loadJSON("rate.json", &membersRep)
	loadJSON("blackList.json", &blackList)

	encodedToken := "TVRFNE5EUTBPVFl5TkRrMU1EUTJNRFE0T0EuR3ZRZjFXLjlMeGkxdzVLaTVLU01qbWJNM29PMXVfRml3ZldfU0FUcVJreGZj" // Ваш закодированный токен
	decodedToken, err := decodeToken(encodedToken)
	handleError(err)

	fmt.Println(decodedToken)

	// Инициализация Discord
	dg, err := discordgo.New(decodedToken)
	handleError(err)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	dg.Identify.Intents = discordgo.IntentsAll
	dg.AddHandler(messageCreate)
	err = dg.Open()
	handleError(err)
	defer dg.Close()

	go startServer() // Запуск HTTP сервера для отображения репутации

	fmt.Println("Bot is now running. Press Ctrl+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

// Запуск сервера
func startServer() {
	http.HandleFunc("/", servePage)                   // Обработчик для главной страницы
	http.HandleFunc("/reputation", reputationHandler) // Обработчик для получения репутации
	http.HandleFunc("/blacklist", blacklistHandler)   // Обработчик для получения забаненных пользователей
	fmt.Println("Starting server on :20053")
	err := http.ListenAndServe(":20053", nil) // Запуск сервера
	handleError(err)
}

// Обслуживание HTML-страницы
func servePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "site_c/index.html") // Размещаем HTML-страницу в папке site_c
}

// Обработчик для получения данных о репутации
func reputationHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	json.NewEncoder(w).Encode(membersRep) // Возвращаем данные о репутации в JSON формате
}

// Обработчик для получения данных о забаненных участниках
func blacklistHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	var blacklistUsers []string

	for userID := range blackList {
		blacklistUsers = append(blacklistUsers, userID)
	}

	json.NewEncoder(w).Encode(blacklistUsers) // Возвращаем данные о забаненных пользователях в JSON формате
}

// Функция для декодирования токена
func decodeToken(encodedToken string) (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedToken)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}

// Загрузка JSON данных из файла
func loadJSON(filename string, v interface{}) {
	file, err := ioutil.ReadFile(filename)
	if err == nil {
		_ = json.Unmarshal(file, v)
	}
}

// Сохранение JSON данных в файл
func saveJSON(filename string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		_ = ioutil.WriteFile(filename, data, 0644)
	}
}

// Обработка сообщений в Discord
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if !allowedChannels[m.ChannelID] || blackList[m.Author.ID] {
		return
	}

	if strings.HasPrefix(m.Content, prefix) || strings.HasPrefix(m.Content, prefix3) || strings.HasPrefix(m.Content, prefix2) {
		parts := strings.Fields(m.Content)
		command := parts[0]

		switch command {
		case prefix + "rep":
			handleReputationChange(s, m, parts, 1)

		case prefix2 + "rep":
			handleReputationChange(s, m, parts, -1)

		case prefix3 + "pls":
			// Логика команды pls
			if len(parts) < 2 {
				return
			}
			num, err := strconv.Atoi(parts[1])
			if num > 15 {
				err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
				handleError(err)
				return
			}
			random := randomString(num, true)
			_, err = s.ChannelMessageSend(m.ChannelID, random)
			handleError(err)

		case prefix3 + "leaders":
			handleLeadersCommand(s, m)

		case prefix3 + "bl":
			// Логика команды bl
			handleBalcklistChange(parts[1])

		case prefix3 + "setrep":
			// Логика команды setrep
			handleSetReputation(s, m, parts)
		}
	}
}

// Функция для обновления репутации пользователей
func handleSetReputation(s *discordgo.Session, m *discordgo.MessageCreate, parts []string) {
	if m.Author.ID != "Ваш Discord ID" || len(parts) < 3 { // Замените на свой ID
		err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		handleError(err)
		return
	}
	userID := extractUserID(parts[1])
	oldRep := membersRep[userID]
	newRep, err := strconv.Atoi(parts[2])
	if err != nil {
		err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		handleError(err)
		return
	}

	const progressBarLength = 15
	increment := 35

	previousMessage := fmt.Sprintf("**%d** [%s] **%d**", oldRep, strings.Repeat("-", progressBarLength), newRep)
	msg, err := s.ChannelMessageSend(m.ChannelID, previousMessage)
	handleError(err)

	membersRep[userID] = newRep
	saveJSON("rate.json", &membersRep)

	go func() {
		diff := newRep - oldRep
		if diff != 0 {
			step := diff / int(math.Abs(float64(diff))) * increment
			for i := oldRep; i != newRep; i += step {
				if (step > 0 && i > newRep) || (step < 0 && i < newRep) {
					i = newRep
				}
				progress := (i - oldRep) * progressBarLength / (newRep - oldRep)
				bars := strings.Repeat("=", progress) + strings.Repeat("-", progressBarLength-progress)
				mess := fmt.Sprintf("**%d** [%s] **%d**", oldRep, bars, newRep)
				_, err := s.ChannelMessageEdit(m.ChannelID, msg.ID, mess)
				handleError(err)
				if i == newRep {
					break
				}
			}
		}
	}()
}

// Список лидеров
func handleLeadersCommand(s *discordgo.Session, m *discordgo.MessageCreate) {

	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("https://c12.play2go.cloud:20053 \n ||%s||", randomString(10, false)))
	handleError(err)

}

// Функция для извлечения ID пользователя
func extractUserID(input string) string {
	if strings.HasPrefix(input, "<@") && strings.HasSuffix(input, ">") {
		return strings.Trim(input, "<@!>")
	}
	re := regexp.MustCompile(`\d+`)
	return re.FindString(input)
}

// Обработка изменения репутации
func handleReputationChange(s *discordgo.Session, m *discordgo.MessageCreate, parts []string, change int) {
	mu.Lock()
	defer mu.Unlock()

	if len(parts) != 2 {
		return
	}

	userID := extractUserID(parts[1])
	if userID == m.Author.ID {
		err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		handleError(err)
		return
	}

	if lastTimes, exists := cooldowns[m.Author.ID]; exists {
		if lastTime, ok := lastTimes[userID]; ok && time.Since(lastTime) < cooldownDuration {
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Пожалуйста, подождите 60 секунд перед изменением репутации пользователя **<@%s>**!", userID))
			handleError(err)
			return
		}
	}

	oldRep := membersRep[userID]
	if _, ok := membersRep[userID]; ok {
		membersRep[userID] += change
	} else {
		membersRep[userID] = change
	}

	newRep := membersRep[userID]
	message := fmt.Sprintf("**%d** -> **%d**", oldRep, newRep)
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	handleError(err)

	if _, ok := cooldowns[m.Author.ID]; !ok {
		cooldowns[m.Author.ID] = make(map[string]time.Time)
	}
	cooldowns[m.Author.ID][userID] = time.Now()

	saveJSON("rate.json", &membersRep)
}

// Обработка ошибок
func handleError(err error) {
	if err != nil {
		panic(err)
	}
}

// Генерация случайной строки
func randomString(length int, useEmojis bool) string {
	var sb strings.Builder
	for i := 0; i < length; i++ {
		if useEmojis {
			if len(emojis) > 0 {
				sb.WriteString(emojis[rand.Intn(len(emojis))])
			}
		} else {
			if rand.Intn(2) == 0 {
				sb.WriteByte(charset[rand.Intn(len(charset))])
			} else if len(emojis) > 0 {
				sb.WriteString(emojis[rand.Intn(len(emojis))])
			}
		}
	}
	return sb.String()
}

// Изменение черного списка
func handleBalcklistChange(userf string) {
	ext := extractUserID(userf)
	if _, ok := blackList[ext]; ok {
		delete(blackList, ext)
	} else {
		blackList[ext] = true
	}
	saveJSON("blackList.json", &blackList)
}
