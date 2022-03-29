package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	gogpt "github.com/sashabaranov/go-gpt3"
	"gopkg.in/ini.v1"
)

const rootURL string = "https://api.groupme.com/v3"

// Bunch of structs to represent group and message data
type Group struct {
	Meta     Meta
	Response Response
}

// Metadata - response code
type Meta struct {
	Code int
}

// Group response
type Response struct {
	Id              string
	Group_Id        string
	Name            string
	Phone_Number    string
	Pinned          bool
	Type            string
	Description     string
	Image_url       string
	Creator_User_Id string
	Created_At      int
	Updated_At      int
	Muted_Until     int
	Office_Mode     bool
	Messages        Messages
	Member          []Member
}

// Messages info
type Messages struct {
	Count                   int
	Last_Message_Id         string
	Last_Message_Created_At int
	Preview                 Preview
}

// Message preview
type Preview struct {
	Nickname   string
	Text       string
	Image_url  string
	Attachment []Attachment
}

// Message attachment
type Attachment struct {
	Type        string
	Url         string
	Lat         string
	Lng         string
	Name        string
	Token       string
	Placeholder string
	Charmap     [][]int
}

// Member data
type Member struct {
	UserId   string
	Nickname string
}

func main() {

	// Load config
	cfg, err := ini.Load("chatbot.ini")
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		os.Exit(1)
	}

	// Read token and group id from cfg
	token := cfg.Section("").Key("token").String()
	groupId := cfg.Section("").Key("group_id").String()
	gpt_token := cfg.Section("").Key("gpt_token").String()
	chatbot_name := cfg.Section("").Key("chatbot_name").String()
	trigger_word := cfg.Section("").Key("trigger_word").String()

	// Set up GPT-3 client
	c := gogpt.NewClient(gpt_token)
	ctx := context.Background()

	// Get latest message from group
	newestMessage := getNewestMessage(groupId, token)

	// Run as server
	for {

		// If there is a new message, respond
		newMessagePresent, msg := hasNewMessage(groupId, token, newestMessage)

		if newMessagePresent {

			newestMessage = msg

			// Get message text and sender
			msgText := msg.Response.Messages.Preview.Text
			msgSender := msg.Response.Messages.Preview.Nickname

			// Make sure we don't repond to ourselves
			if !strings.Contains(msgSender, chatbot_name) {

				// Look for keyword !marv in message
				if strings.Contains(msgText, trigger_word) {

					// Strip off the keyword
					userPrompt := strings.ReplaceAll(msgText, trigger_word, "")

					// Have GPT-3 respond
					response, err := completeText(c, ctx, userPrompt)
					if err != nil {
						log.Fatal(err)
					}

					// Send response back to group
					sendMessageToGroup(groupId, token, strings.TrimSpace(response))
				}
			}
		} else {
			time.Sleep(time.Second)
		}
	}
}

func completeText(c *gogpt.Client, ctx context.Context, text string) (string, error) {
	req := gogpt.CompletionRequest{
		MaxTokens:        60,
		Temperature:      0.5,
		TopP:             0.3,
		FrequencyPenalty: 0.5,
		PresencePenalty:  0.0,
		Prompt:           "Marv is a chatbot that reluctantly answers questions with sarcastic responses:\n\nYou: " + text + "\nMarv: ",
	}
	resp, err := c.CreateCompletion(ctx, "text-davinci-002", req)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Text, nil
}

// Check if there is a new message in the group
// Compares message ids
func hasNewMessage(groupId string, token string, lastMessage Group) (bool, Group) {
	newestMessage := getNewestMessage(groupId, token)
	return newestMessage.Response.Messages.Last_Message_Id > lastMessage.Response.Messages.Last_Message_Id, newestMessage
}

// Get the latest message from the group
// Makes GET request to group endpoint
// Returns a Group struct
func getNewestMessage(groupId string, token string) Group {

	// Set up request
	requestURL := rootURL + "/groups/" + groupId + "?token=" + token
	response, err := http.Get(requestURL)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	// Parse JSON response into Group struct
	var groups Group
	jsonErr := json.NewDecoder(response.Body).Decode(&groups)

	if jsonErr != nil {
		fmt.Println("Error: ", jsonErr)
		log.Fatal(jsonErr)
	}

	return groups
}

func sendMessageToGroup(groupId string, token string, message string) {

	// Set up request
	requestURL := rootURL + "/groups/" + groupId + "/messages?token=" + token

	// Set up message body
	// Using unix timestamp to avoid guid collision
	guid := time.Now().UnixNano()
	messageBody := fmt.Sprintf(`{"message":{"source_guid":"%d","text":"%s"}}`, guid, message)

	// Send POST request
	requestBody := strings.NewReader(messageBody)
	_, err := http.Post(requestURL, "application/json", requestBody)
	if err != nil {
		log.Fatal(err)
	}
}
