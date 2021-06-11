package dolista

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Update is a Telegram object that the handler receives every time an user interacts with the bot.
type Update struct {
	UpdateId int     `json:"update_id"`
	Message  Message `json:"message"`
}

// Message is a Telegram object that can be found in an update.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

// A Telegram Chat indicates the conversation to which the message belongs.
type Chat struct {
	Id int `json:"id"`
}

// SummaryItem represents one of the items that can be held by a summary
type SummaryItem struct {
	Message string `json:"message"`
}

// Config represents the application config params
type Config struct {
	TelegramToken  string `json:"telegramToken"`
	FirebaseConfig struct {
		Type                    string `json:"type"`
		ProjectID               string `json:"project_id"`
		PrivateKeyID            string `json:"private_key_id"`
		PrivateKey              string `json:"private_key"`
		ClientEmail             string `json:"client_email"`
		ClientID                string `json:"client_id"`
		AuthURI                 string `json:"auth_uri"`
		TokenURI                string `json:"token_uri"`
		AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
		ClientX509CertURL       string `json:"client_x509_cert_url"`
	} `json:"firebaseConfig"`
}

func HandleMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var reqBody Update
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("failed to decode request body: %v\n", err)
		return
	}

	appConfig := os.Getenv("APP_CONFIG")
	bConfig, err := base64.RawStdEncoding.DecodeString(appConfig)
	if err != nil {
		log.Printf("failed decode app config: %v\n", err)
		return
	}

	var config Config
	err = json.Unmarshal(bConfig, &config)
	if err != nil {
		log.Printf("failed to parse app config: %v\n", err)
		return
	}

	firebaseConfig, err := json.Marshal(config.FirebaseConfig)
	if err != nil {
		log.Printf("failed to parse firebase config: %v\n", err)
		return
	}

	opt := option.WithCredentialsJSON(firebaseConfig)
	// conf := &firebase.Config{
	// 	ProjectID:        "dolista-safado",
	// 	ServiceAccountID: "github-actions@dolista-safado.iam.gserviceaccount.com",
	// }
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Printf("failed to create firebase app: %v\n", err)
		return
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		log.Printf("failed to create firestore client: %v\n", err)
		return
	}
	defer client.Close()

	if strings.HasPrefix(reqBody.Message.Text, "/hello") {
		handleHello(reqBody.Message.Chat.Id, reqBody.Message.Text, config.TelegramToken)
		return
	}

	if strings.HasPrefix(reqBody.Message.Text, "/safada") {
		handleSafada(reqBody.Message.Chat.Id, reqBody.Message.Text, config.TelegramToken)
		return
	}

	if strings.HasPrefix(reqBody.Message.Text, "/resumo") {
		handleResumo(ctx, reqBody.Message.Chat.Id, config.TelegramToken, client)
		return
	}

	// Make sure `/r` is checked after `/resumo` or the logic will break.
	if strings.HasPrefix(reqBody.Message.Text, "/r") {
		handleAddResumo(ctx, reqBody.Message.Chat.Id, reqBody.Message.Text, config.TelegramToken, client)
		return
	}
}

func handleHello(chatID int, message, token string) {
	sendMessage(chatID, "hello!", token)
}

func handleSafada(chatID int, message, token string) {
	sendMessage(chatID, "é você!", token)
}

func handleResumo(ctx context.Context, chatID int, token string, client *firestore.Client) {
	iter := client.Collection("summary").Documents(ctx)
	var b strings.Builder
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("failed to iterate: %v\n", err)
			return
		}
		var item SummaryItem
		err = doc.DataTo(&item)
		if err != nil {
			log.Printf("failed parse item: %v\n", err)
			return
		}

		b.Grow(len(item.Message))
		b.WriteString("- " + item.Message + "\n")
	}

	sendMessage(chatID, b.String(), token)
}

func handleAddResumo(ctx context.Context, chatID int, message, token string, client *firestore.Client) {
	split := strings.Split(message, " ")
	newEntry := strings.Join(split[1:], " ")
	_, _, err := client.Collection("summary").Add(ctx, SummaryItem{Message: newEntry})
	if err != nil {
		log.Printf("Falied to add message: %v\n", err)
		sendMessage(chatID, "Ops! O código do Caio não funcionou :)", token)
		return
	}

	successMsg := fmt.Sprintf("Adicionado ao resumo: %v", newEntry)
	sendMessage(chatID, successMsg, token)
}

func sendMessage(chatID int, message, token string) {
	var telegramApi string = "https://api.telegram.org/bot" + token + "/sendMessage"
	response, err := http.PostForm(
		telegramApi,
		url.Values{
			"chat_id": {strconv.Itoa(chatID)},
			"text":    {message},
		})

	if err != nil {
		return // TODO: return friendly bot message for error cases
	}

	defer response.Body.Close()
}
