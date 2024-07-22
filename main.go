package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get the Telegram API token from the environment variables
	token := os.Getenv("TELEGRAM_APITOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_APITOKEN is not set in the environment variables")
	}

	// Initialize the Telegram bot API
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("Error creating Telegram bot: %v", err)
	}

	bot.Debug = true

	// Create a new UpdateConfig struct with an offset of 0
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	// Start polling Telegram for updates
	updates := bot.GetUpdatesChan(updateConfig)

	// Go through each update that we're getting from Telegram
	for update := range updates {
		// We only want to look at messages for now, so discard any other updates
		if update.Message == nil {
			continue
		}

		// Construct a reply! Take the Chat ID and Text from the incoming message and use it to create a new message
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		msg.ReplyToMessageID = update.Message.MessageID

		// Send our message off!
		if _, err := bot.Send(msg); err != nil {
			// Note that panics are a bad way to handle errors. Telegram can have service outages or network errors, you should retry sending messages or more gracefully handle failures
			log.Printf("Error sending message: %v", err)
		}
	}
}
