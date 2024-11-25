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
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	prefix     = "+"
	prefix2    = "-"
	prefix3    = "!"
	prefix4    = ">"
	charset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-*&^%$#@!)(*/|"
	webhookURL = "https://canary.discord.com/api/webhooks/..."
)

var (
	emojis          = []string{":poop:", ":heart_eyes_cat:", ":firecracker:", ":leafy_green:", ":money_mouth:", ":imp:", ":wink:", ":pleading_face:", ":x:", ":woman_with_headscarf:", ":key:", ":champagne:", ":tada:", ":white_check_mark:", ":thumbsdown:", ":thumbsup:"}
	membersRep      = map[string]int{}
	allowedChannels = map[string]bool{"1091008984913289307": true}
	blackList       = map[string]bool{}

	cooldowns        = make(map[string]map[string]time.Time)
	cooldownDuration = 60 * time.Second

	topCooldown         = make(map[string]time.Time)
	topCooldownDuration = 60 * time.Second

	messageCounts = make(map[string]int)
	totalMessages = 0
)

type UserCount struct {
	ID    string
	Count int
}

var userCounts []UserCount

func main() {
	loadJSON("rate.json", &membersRep)
	loadJSON("blackList.json", &blackList)
	loadJSON("messagedata.json", &messageCounts)

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
	//dg.AddHandler(handleDayTop)
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

	json.NewEncoder(w).Encode(membersRep) // Возвращаем данные о репутации в JSON формате
}

// Обработчик для получения данных о забаненных участниках
func blacklistHandler(w http.ResponseWriter, r *http.Request) {

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

	messageCounts[m.Author.ID]++
	totalMessages++

	if totalMessages%30 == 0 {
		saveJSON("messagedata.json", &messageCounts)
	}

	if strings.HasPrefix(m.Content, prefix) || strings.HasPrefix(m.Content, prefix3) || strings.HasPrefix(m.Content, prefix2) || strings.HasPrefix(m.Content, prefix4) {
		parts := strings.Fields(m.Content)
		command := parts[0]

		switch command {
		case prefix + "rep":
			handleReputationChange(s, m, parts, 1)

		case prefix2 + "rep":
			handleReputationChange(s, m, parts, -1)

		case prefix3 + "pls":

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

		case prefix4 + "top":

			if lastTime, exists := topCooldown[m.Author.ID]; exists && time.Since(lastTime) < topCooldownDuration {
				remaining := topCooldownDuration - time.Since(lastTime)
				seconds := int(remaining.Seconds())
				_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Пожалуйста, подождите %d секунд перед повторным использованием команды **>top**!\n||%s||", seconds, randomString(10, false)))
				handleError(err)
				return
			}

			topCooldown[m.Author.ID] = time.Now()
			handleTopReputation(s, m)

		case prefix4 + "ping":

			ping := s.HeartbeatLatency()
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏓 Понг: **%v**\n||%s||", ping.Milliseconds(), randomString(5, false)))
			handleError(err)

		case prefix3 + "bl":

			err := s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			handleError(err)

			ext := extractUserID(parts[1])

			nam := GetNameID(s, m, ext)
			handleBalcklistChange(ext, nam)

		case prefix3 + "setrep":
			handleSetReputation(s, m, parts)
		}
	}
}

// Command to show top users by message count
//func handleDayTop(s *discordgo.Session, m *discordgo.MessageCreate) {
//
//	now := time.Now()
//	nextReset := time.Date(now.Year(), now.Month(), now.Day(), 23, 30, 59, 0, time.Local)
//	if now.After(nextReset) {
//		nextReset = nextReset.Add(24 * time.Hour)
//	}
//	time.Sleep(nextReset.Sub(now))
//
//	for id, count := range messageCounts {
//		userCounts = append(userCounts, UserCount{ID: id, Count: count})
//	}
//
//	sort.Slice(userCounts, func(i, j int) bool {
//		return userCounts[i].Count > userCounts[j].Count
//	})
//
//	response := "-# ### 😈**Top Users Today:**\n"
//	for _, uc := range userCounts {
//		name := GetNameID(s, m, uc.ID)
//		response += fmt.Sprintf("-# **%s**: **%d** сообщений\n", name, uc.Count)
//	}
//
//	_, err := s.ChannelMessageSend("1091008984913289307", response)
//	handleError(err)
//
//	clear(messageCounts)
//	saveJSON("messagedata.json", &messageCounts)
//	totalMessages = 0
//
//	_, err = s.ChannelMessageSend("1091008984913289307", "Данные сообщений очищены, **пишите**, **веселитесь**, ~~не~~ **нарушайте**")
//	handleError(err)
//
//}

// Обработка команды для получения топ репутации
func handleTopReputation(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Создаем слайс для хранения пар (имя, репутация)
	type UserReputation struct {
		ID  string
		Rep int
	}
	var userReps []UserReputation

	// Заполняем слайс данными о репутации
	for id, rep := range membersRep {
		userReps = append(userReps, UserReputation{ID: id, Rep: rep})
	}

	// Сортируем данные по убыванию репутации
	sort.Slice(userReps, func(i, j int) bool {
		return userReps[i].Rep > userReps[j].Rep
	})

	// Формируем ответ
	response := "-# ### 🏆**Топ репутации:**🏆\n"
	for _, userRep := range userReps {
		name := GetNameID(s, m, userRep.ID)
		response += fmt.Sprintf("-# **%s** -> **%d**\n", name, userRep.Rep)
	}
	response += fmt.Sprintf("||%s||", randomString(5, false))
	// Отправляем сообщение с топом пользователей
	_, err := s.ChannelMessageSend(m.ChannelID, response)
	handleError(err)
}

// Функция для обновления репутации пользователей
func handleSetReputation(s *discordgo.Session, m *discordgo.MessageCreate, parts []string) {
	if m.Author.ID != "1184449624950460488" || len(parts) < 3 { // Замените на свой ID
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

	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("http://c12.play2go.cloud:20053\n||%s||", randomString(10, false)))
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
			remaining := cooldownDuration - time.Since(lastTime)
			seconds := int(remaining.Seconds())
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Пожалуйста, подождите %d секунд перед изменением репутации пользователя **<@%s>**!\n||%s||", seconds, userID, randomString(10, true)))
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

func GetNameID(s *discordgo.Session, m *discordgo.MessageCreate, id string) string {
	user, err := s.GuildMember(m.GuildID, id)
	handleError(err)
	return user.User.Username
}

// Изменение черного списка
func handleBalcklistChange(userf string, name string) {

	// Create the key in the format "userID, name"
	key := fmt.Sprintf("%s, %s", userf, name)

	if _, ok := blackList[key]; ok {
		delete(blackList, key)
	} else {
		blackList[key] = true
	}

	saveJSON("blackList.json", &blackList)
}
