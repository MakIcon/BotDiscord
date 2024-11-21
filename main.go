package main

import (
	"bytes"
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
	"sync"
	"syscall"
	"time"
)

const (
	prefix     = "+"
	prefix2    = "-"
	prefix3    = "!"
	charset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-*&^%$#@!)(*/|"
	webhookURL = "https://canary.discord.com/api/webhooks/1292843976080228363/Kzn3yJJGTRb46reJhh0jgYTZuORGB6o5cegT9NRiCCw77ViRtHveOrxHTWeXNEOxPhe-"
)

// Список текстовых смайликов Discord
var (
	emojis = []string{
		":poop:", ":heart_eyes_cat:", ":firecracker:", ":leafy_green:",
		":money_mouth:", ":imp:", ":wink:", ":pleading_face:", ":x:",
		":woman_with_headscarf:", ":key:", ":champagne:", ":tada:",
		":white_check_mark:", ":thumbsdown:", ":thumbsup:",
	}

	memebersrep = map[string]int{
		"1184449624950460488": 0,
	}

	allowedChannels = map[string]bool{
		"1091008984913289307": true,
	}

	whiteList = map[string]bool{}

	mu sync.Mutex

	cooldowns        = make(map[string]map[string]time.Time)
	cooldownDuration = 60 * time.Second
	allOptions       []string
)

func main() {
	loadJSON("rate.json", &memebersrep)
	loadJSON("whiteList.json", &whiteList)

	encodedToken := "TVRFNE5EUTBPVFl5TkRrMU1EUTJNRFE0T0EuR3ZRZjFXLjlMeGkxdzVLaTVLU01qbWJNM29PMXVfRml3ZldfU0FUcVJreGZj"

	// Decode the token
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedToken)
	if err != nil {
		fmt.Println("Error decoding token:", err)
		return
	}

	decodedToken := string(decodedBytes)

	fmt.Println(decodedToken)

	// Комбинируем charset и emojis в один срез
	allOptions = make([]string, 0, len(charset)+len(emojis))
	for _, c := range charset {
		allOptions = append(allOptions, string(c))
	}
	allOptions = append(allOptions, emojis...)

	dg, err := discordgo.New(decodedToken)
	if err != nil {
		fmt.Println("Error creating Discord session:", err)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	dg.Identify.Intents = discordgo.IntentsAll

	dg.AddHandler(messageCreate)

	err = dg.Open()
	if err != nil {
		panic(err)
	}
	defer dg.Close()

	fmt.Println("Bot is now running. Press Ctrl+C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func loadJSON(filename string, v interface{}) {
	file, err := ioutil.ReadFile(filename)
	if err == nil {
		_ = json.Unmarshal(file, v)
	}
}

func saveJSON(filename string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		_ = ioutil.WriteFile(filename, data, 0644)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if !allowedChannels[m.ChannelID] || !whiteList[m.Author.ID] {
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

			if len(parts) < 2 {
				return
			}

			num, err := strconv.Atoi(parts[1])

			if num > 15 {
				err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
				if err != nil {
					panic(err)
				}
				return
			}

			random := randomString(num, true)

			_, err = s.ChannelMessageSend(m.ChannelID, random)
			if err != nil {
				panic(err)
			}

		case prefix3 + "leaders":
			handleLeadersCommand(s, m)

		case prefix3 + "bl":

			if m.Author.ID != "1184449624950460488" {
				err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
				if err != nil {
					panic(err)
				}
				return
			} else {
				err := s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
				if err != nil {
					panic(err)
				}
			}

			handleBalcklistChange(parts[1])

		case prefix3 + "setrep":
			if m.Author.ID != "1184449624950460488" || len(parts) < 3 {
				err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
				if err != nil {
					panic(err)
				}
				return
			}

			userID := extractUserID(parts[1])

			oldRep := memebersrep[userID]

			newRep, err := strconv.Atoi(parts[2])
			if err != nil {
				err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
				if err != nil {
					panic(err)
				}
				return
			}

			const progressBarLength = 15
			increment := 35

			previousMessage := fmt.Sprintf("**%d** [%s] **%d**", oldRep, strings.Repeat("-", progressBarLength), newRep)
			msg, err := s.ChannelMessageSend(m.ChannelID, previousMessage)
			if err != nil {
				panic(err)
			}

			memebersrep[userID] = newRep
			saveJSON("rate.json", &memebersrep)

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
						if err != nil {
							panic(err)
						}
						fmt.Println("fewfwfewfwef")

						if i == newRep {
							fmt.Println("ergergeg")
							break
						}
					}
				}
				//bars := strings.Repeat("=", progressBarLength)
				//finalMessage := fmt.Sprintf("**%d** [%s] **%d**", oldRep, bars, newRep)
				//_, err := s.ChannelMessageEdit(m.ChannelID, msg.ID, finalMessage)
				//if err != nil {
				//	panic(err)
				//}
			}()

		}
	}
}

func handleLeadersCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Create a slice for sorting
	type userRep struct {
		UserID string
		Rep    int
	}

	reps := make([]userRep, 0, len(memebersrep))
	for id, rep := range memebersrep {
		reps = append(reps, userRep{UserID: id, Rep: rep})
	}

	// Sort the slice by reputation in descending order
	sort.Slice(reps, func(i, j int) bool {
		return reps[i].Rep > reps[j].Rep
	})

	// Build the message
	var builder strings.Builder
	builder.WriteString("-# **Таблица лидеров**\n")

	for index, entry := range reps {
		if index == 0 {
			// Header already added
		}

		// Get user by ID
		user, err := s.User(entry.UserID)
		username := "Неизвестный пользователь"
		if err == nil {
			username = user.Username
		}

		// Format the line
		builder.WriteString(fmt.Sprintf("-# %s: %d реп\n", username, entry.Rep))
	}

	// Send the message
	s.ChannelMessageSend(m.ChannelID, builder.String())
}

func sendWebhookMessage(webhookURL string, embed *discordgo.MessageEmbed) error {
	client := &http.Client{}
	data := map[string]interface{}{
		"embeds": []interface{}{embed},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to send webhook message: %s", body)
	}

	return nil
}

func extractUserID(input string) string {
	if strings.HasPrefix(input, "<@") && strings.HasSuffix(input, ">") {
		return strings.Trim(input, "<@!>")
	}
	re := regexp.MustCompile(`\d+`)
	return re.FindString(input)
}

func handleReputationChange(s *discordgo.Session, m *discordgo.MessageCreate, parts []string, change int) {
	mu.Lock()
	defer mu.Unlock()

	if len(parts) != 2 {
		_, err := s.ChannelMessageSend(m.ChannelID, "Неверное использование команды. Используйте: "+parts[0]+" <user>")
		if err != nil {
			fmt.Println("Error sending message:", err)
		}
		return
	}

	userID := extractUserID(parts[1])
	fmt.Println(userID)

	if userID == m.Author.ID {
		err := s.MessageReactionAdd(m.ChannelID, m.ID, "❌")
		if err != nil {
			panic(err)
		}
		return
	}

	// Initialize cooldowns for the author if not present
	if lastTimes, exists := cooldowns[m.Author.ID]; exists {
		if lastTime, ok := lastTimes[userID]; ok && time.Since(lastTime) < cooldownDuration {
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Пожалуйста, подождите 60 секунд перед изменением репутации пользователя **<@%s>**! \n\n||%s||", userID, randomString(10, true)))
			if err != nil {
				fmt.Println("Error sending cooldown message:", err)
			}
			return
		}
	}

	// Get old reputation
	oldRep := memebersrep[userID]

	// Update reputation
	if _, ok := memebersrep[userID]; ok {
		memebersrep[userID] += change
	} else {
		memebersrep[userID] = change
	}

	newRep := memebersrep[userID]

	// Prepare the channel message
	message := fmt.Sprintf("**%d** -> **%d**", oldRep, newRep)
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	if err != nil {
		fmt.Println("Error sending reputation message:", err)
	}

	// Update cooldown
	if _, ok := cooldowns[m.Author.ID]; !ok {
		cooldowns[m.Author.ID] = make(map[string]time.Time)
	}
	cooldowns[m.Author.ID][userID] = time.Now()

	// Save the updated reputation to the JSON file
	saveJSON("rate.json", &memebersrep)

	// Prepare the embed for the webhook
	embed := &discordgo.MessageEmbed{
		Title:       "Изменение Репутации",
		Description: fmt.Sprintf("<@%s> получил изменение репутации.", userID),
		Color:       0x00ff00, // Green color; you can choose different colors based on the change
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Пользователь",
				Value:  fmt.Sprintf("<@%s> (%s)", userID, userID),
				Inline: true,
			},
			{
				Name:   "Старая Репутация",
				Value:  fmt.Sprintf("**%d**", oldRep),
				Inline: true,
			},
			{
				Name:   "Новая Репутация",
				Value:  fmt.Sprintf("**%d**", newRep),
				Inline: true,
			},
			{
				Name:   "Изменение",
				Value:  fmt.Sprintf("%+d", change),
				Inline: true,
			},
			{
				Name:   "Кто изменил",
				Value:  fmt.Sprintf("<@%s>", m.Author.ID),
				Inline: true,
			},
			{
				Name:   "Время",
				Value:  time.Now().Format(time.RFC1123),
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	err = sendWebhookMessage(webhookURL, embed)
	if err != nil {
		fmt.Println("Error sending webhook message:", err)
	}

}

func randomString(length int, useEmojis bool) string {
	var sb strings.Builder
	for i := 0; i < length; i++ {
		if useEmojis {
			// Only use emojis
			if len(emojis) > 0 {
				sb.WriteString(emojis[rand.Intn(len(emojis))])
			}
		} else {
			// Use both charset and emojis with a 50% chance each
			if rand.Intn(2) == 0 { // 50% chance to choose a character from charset
				sb.WriteByte(charset[rand.Intn(len(charset))])
			} else if len(emojis) > 0 { // 50% chance to choose an emoji
				sb.WriteString(emojis[rand.Intn(len(emojis))])
			}
		}
	}
	return sb.String()
}

func handleBalcklistChange(userf string) {

	ext := extractUserID(userf)

	if _, ok := whiteList[ext]; ok {
		delete(whiteList, ext)
	} else {
		whiteList[ext] = true
	}

	saveJSON("whiteList.json", &whiteList)
}
