package main

import (
	"BotDiscord/chat"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
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
	prefix  = "+"
	prefix2 = "-"
	prefix3 = "!"
	prefix4 = ">"

	prefixA = "a"
	prefixD = "d"

	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-*&^%$#@!)(*/|"
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients []*websocket.Conn

func main() {
	loadJSON("rate.json", &membersRep)
	loadJSON("blackList.json", &blackList)
	loadJSON("messagedata.json", &messageCounts)

	encodedToken := "TVRFNE5EUTBPVFl5TkRrMU1EUTJNRFE0T0EuR3ZRZjFXLjlMeGkxdzVLaTVLU01qbWJNM29PMXVfRml3ZldfU0FUcVJreGZj" // Ваш закодированный токен
	decodedToken, err := decodeToken(encodedToken)
	handleError(err)

	fmt.Println(decodedToken)

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
	http.HandleFunc("/", servePage)
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/save-color", SaveColorHandler)
	http.HandleFunc("/load-colors", LoadColorsHandler)
	log.Println("Server started on :20053")
	err := http.ListenAndServe(":20053", nil)
	handleError(err)
}

// Обслуживание HTML-страницы
func servePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "site_c/index.html") // Размещаем HTML-страницу в папке site_c
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Error during connection upgrade:", err)
		return
	}
	defer conn.Close()

	clients = append(clients, conn)

	for {
		var msg map[string]string
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}
		broadcast(msg) // Уведомляем других клиентов об изменении
	}
}

func broadcast(msg map[string]string) {
	for _, client := range clients {
		err := client.WriteJSON(msg)
		if err != nil {
			client.Close()
			removeClient(client)
		}
	}
}

func removeClient(client *websocket.Conn) {
	for i, c := range clients {
		if c == client {
			clients[i] = clients[len(clients)-1]
			clients[len(clients)-1] = nil
			clients = clients[:len(clients)-1]
			break
		}
	}
}

func SaveColorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var colors map[string]string
		if err := json.NewDecoder(r.Body).Decode(&colors); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		existingColors := make(map[string]string)
		loadJSON("colors.json", &existingColors)

		for key, value := range colors {
			existingColors[key] = value
		}

		saveJSON("colors.json", existingColors)

		w.WriteHeader(http.StatusOK)
		return
	}
	http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}

func LoadColorsHandler(w http.ResponseWriter, r *http.Request) {
	existingColors := make(map[string]string)
	loadJSON("colors.json", &existingColors)
	json.NewEncoder(w).Encode(existingColors)
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

	if strings.HasPrefix(m.Content, prefix) || strings.HasPrefix(m.Content, prefix3) || strings.HasPrefix(m.Content, prefix2) || strings.HasPrefix(m.Content, prefix4) || strings.HasPrefix(m.Content, prefixA) || strings.HasPrefix(m.Content, prefixD) {
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

			//if lastTime, exists := topCooldown[m.Author.ID]; exists && time.Since(lastTime) < topCooldownDuration {
			//	remaining := topCooldownDuration - time.Since(lastTime)
			//	seconds := int(remaining.Seconds())
			//	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Пожалуйста, подождите %d секунд перед повторным использованием команды **>top**!\n||%s||", seconds, randomString(10, false)))
			//	handleError(err)
			//	return
			//}
			//
			//topCooldown[m.Author.ID] = time.Now()
			//handleTopReputation(s, m)

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

		case prefixA + "lo":
			if len(parts) < 2 {
				return
			}
			messageToSay := strings.Join(parts[1:], " ")

			response := airesponce(messageToSay, false)

			_, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference())
			handleError(err)

		case prefixD + "ai":
			if len(parts) < 2 {
				return
			}

			prompt := strings.Join(parts[1:], " ")

			response := airesponce(prompt, true)

			_, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference())
			handleError(err)

		}

	}
}

func airesponce(prompt string, img bool) string {
	model := chat.MODELS.Gpt4o

	chatInstance := chat.New(&model, true, 0.7)

	if img {
		chatInstance.AgentMode = chat.MODES.ImageGeneration
	}

	userMessage := chat.Message{
		Role:    "user",
		Content: prompt,
	}

	response := chatInstance.SendMessage(userMessage)

	return response
}

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
