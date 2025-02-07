package main

import (
	"Bot/internal/BotAPI"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"log"
	"os"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	token := os.Getenv("TELEGRAM_API_TOKEN")
	if token == "" {
		log.Fatal("Некорректный токен!")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		switch {
		case text == "/start":
			BotAPI.HandleStart(bot, chatID)
		case text == "/newad":
			BotAPI.HandleNewAd(bot, chatID)
		case text == "/myads":
			BotAPI.HandleMyAds(bot, chatID)
		case text == "/deletead":
			BotAPI.HandleDeleteAd(bot, chatID)
		case text == "/profile":
			BotAPI.HandleProfile(bot, chatID)
		case text == "/feed":
			BotAPI.HandleFeed(bot, chatID)
		case text == "/review":
			BotAPI.HandleReview(bot, chatID)
		case text == "/register":
			BotAPI.HandleRegister(bot, chatID)
		default:
			if BotAPI.WaitingForFeedInput[chatID] {
				BotAPI.ProcessFeedRequest(bot, chatID, text)
				BotAPI.WaitingForFeedInput[chatID] = false
			} else if BotAPI.WaitingForReviewInput[chatID] {
				BotAPI.ProcessReview(bot, chatID, text)
				BotAPI.WaitingForReviewInput[chatID] = false
			} else {
				msg := tgbotapi.NewMessage(chatID, "Неизвестная команда. Используйте /start для списка команд.")
				bot.Send(msg)
			}
			if BotAPI.WaitingForNewAdInput[chatID] {
				BotAPI.ProcessNewAd(bot, chatID, text)
				BotAPI.WaitingForNewAdInput[chatID] = false
				continue
			}
			if BotAPI.WaitingForFeedInput[chatID] {
				BotAPI.ProcessFeedRequest(bot, chatID, text)
				BotAPI.WaitingForFeedInput[chatID] = false
				continue
			}
			if BotAPI.WaitingForRegistration[chatID] {

				BotAPI.ProcessRegistration(bot, chatID, text)
				BotAPI.WaitingForRegistration[chatID] = false
				continue
			}
			if BotAPI.WaitingForDeleteAdInput[chatID] {
				BotAPI.ProcessDeleteAd(bot, chatID, text)
				BotAPI.WaitingForDeleteAdInput[chatID] = false
				continue
			}
		}
	}

}
