package BotAPI

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strconv"
	"strings"
	"sync"
)

var (
	users = make(map[int64]*User) // ID пользователя -> его профиль
	ads   []Ad                    // Лента объявлений
	mu    sync.Mutex              // Для конкурентной безопасности
)

type Review struct {
	Text   string
	Author string
	Rating uint8
}

type Product struct {
	Type      string
	Name      string
	Condition string
	Price     uint64
}

type Ad struct {
	OwnerID     int64
	OwnerName   string
	Description string
	Product     Product
	Location    string
}

type User struct {
	ID      int64
	Name    string
	Rating  float64
	Reviews []Review
	Ads     []Ad
}

func (u *User) CreateAd(description string, product Product, location string) {
	ad := Ad{OwnerID: u.ID, OwnerName: u.Name, Description: description, Product: product, Location: location}
	mu.Lock()
	ads = append(ads, ad)
	u.Ads = append(u.Ads, ad)
	mu.Unlock()
}

func (u *User) DeleteAd(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	for i, ad := range u.Ads {
		if ad.Product.Name == name {
			u.Ads = append(u.Ads[:i], u.Ads[i+1:]...)
			for j, globalAd := range ads {
				if globalAd.Product.Name == name && globalAd.OwnerID == u.ID {
					ads = append(ads[:j], ads[j+1:]...)
					return true
				}
			}
		}
	}
	return false
}

func (u *User) LeaveReview(target *User, text string, rating uint8) {
	if rating > 5 {
		rating = 5
	}
	review := Review{Text: text, Author: u.Name, Rating: rating}
	mu.Lock()
	target.Reviews = append(target.Reviews, review)
	sum := 0.0
	for _, r := range target.Reviews {
		sum += float64(r.Rating)
	}
	target.Rating = sum / float64(len(target.Reviews))
	mu.Unlock()
}

func HandleStart(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Привет! Я бот для обмена вещами. Доступные команды:\n/newad – создать объявление\n/myads – мои объявления\n/deletead – удалить объявление\n/profile – мой профиль\n/feed – лента объявлений\n/review – оставить отзыв")
	bot.Send(msg)
}

func HandleProfile(bot *tgbotapi.BotAPI, chatID int64) {
	mu.Lock()
	user, exists := users[chatID]
	mu.Unlock()

	if !exists {
		msg := tgbotapi.NewMessage(chatID, "Вы не зарегистрированы! Используйте /register.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("👤 Профиль:\nИмя: %s\nРейтинг: %.1f ⭐\nОбъявлений: %d",
		user.Name, user.Rating, len(user.Ads)))
	bot.Send(msg)
}

func GetFilteredAds(filterType, filterLocation string) []Ad {
	var filtered []Ad
	mu.Lock()
	defer mu.Unlock()
	for _, ad := range ads {
		if (filterType == "" || ad.Product.Type == filterType) && (filterLocation == "" || ad.Location == filterLocation) {
			filtered = append(filtered, ad)
		}
	}
	return filtered
}

var WaitingForNewAdInput = make(map[int64]bool)

func HandleNewAd(bot *tgbotapi.BotAPI, chatID int64) {
	WaitingForNewAdInput[chatID] = true
	msg := tgbotapi.NewMessage(chatID, "Введите объявление в формате:\nНазвание, Категория, Состояние, Цена, Местоположение")
	bot.Send(msg)
}

func ProcessNewAd(bot *tgbotapi.BotAPI, chatID int64, input string) {
	parts := strings.Split(input, ",")
	if len(parts) < 5 {
		msg := tgbotapi.NewMessage(chatID, "Ошибка! Используйте формат: Название, Категория, Состояние, Цена, Местоположение")
		bot.Send(msg)
		return
	}

	name := strings.TrimSpace(parts[0])
	category := strings.TrimSpace(parts[1])
	condition := strings.TrimSpace(parts[2])
	price, err := strconv.ParseUint(strings.TrimSpace(parts[3]), 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка! Цена должна быть числом.")
		bot.Send(msg)
		return
	}
	location := strings.TrimSpace(parts[4])

	mu.Lock()
	user, exists := users[chatID]
	if !exists {
		user = &User{ID: chatID, Name: fmt.Sprintf("User_%d", chatID)}
		users[chatID] = user
	}
	mu.Unlock()

	product := Product{Type: category, Name: name, Condition: condition, Price: price}
	user.CreateAd(fmt.Sprintf("Объявление: %s в %s", name, location), product, location)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"✅ Объявление добавлено!\n📌 *%s* (%s)\n💬 %s\n💰 %d руб.\n📍 %s",
		name, category, condition, price, location))
	msg.ParseMode = "Markdown"
	bot.Send(msg)

	WaitingForNewAdInput[chatID] = false
}

func HandleMyAds(bot *tgbotapi.BotAPI, chatID int64) {
	mu.Lock()
	user, exists := users[chatID]
	mu.Unlock()

	if !exists || len(user.Ads) == 0 {
		msg := tgbotapi.NewMessage(chatID, "У вас нет объявлений.")
		bot.Send(msg)
		return
	}

	var response strings.Builder
	response.WriteString("📢 Ваши объявления:\n")
	for _, ad := range user.Ads {
		response.WriteString(fmt.Sprintf("\n📌 *%s* (%s)\n💬 %s\n💰 %d руб.\n📍 %s\n",
			ad.Product.Name, ad.Product.Type, ad.Product.Condition, ad.Product.Price, ad.Location))
	}

	msg := tgbotapi.NewMessage(chatID, response.String())
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

var WaitingForDeleteAdInput = make(map[int64]bool)

func HandleDeleteAd(bot *tgbotapi.BotAPI, chatID int64) {
	mu.Lock()
	user, exists := users[chatID]
	mu.Unlock()

	if !exists || len(user.Ads) == 0 {
		msg := tgbotapi.NewMessage(chatID, "У вас нет объявлений для удаления.")
		bot.Send(msg)
		return
	}

	var response strings.Builder
	response.WriteString("📢 Ваши объявления:\n")
	for _, ad := range user.Ads {
		response.WriteString(fmt.Sprintf("🔹 %s (%s)\n", ad.Product.Name, ad.Product.Type))
	}

	msg := tgbotapi.NewMessage(chatID, response.String()+"\n\nВведите точное название объявления для удаления:")
	bot.Send(msg)

	WaitingForDeleteAdInput[chatID] = true
}

func ProcessDeleteAd(bot *tgbotapi.BotAPI, chatID int64, productName string) {
	mu.Lock()
	user, exists := users[chatID]
	mu.Unlock()

	if !exists {
		msg := tgbotapi.NewMessage(chatID, "Вы не зарегистрированы.")
		bot.Send(msg)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	for i, ad := range user.Ads {
		if strings.EqualFold(ad.Product.Name, productName) { // Игнорируем регистр
			user.Ads = append(user.Ads[:i], user.Ads[i+1:]...) // Удаляем объявление
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Объявление '%s' удалено.", productName))
			bot.Send(msg)
			WaitingForDeleteAdInput[chatID] = false
			return
		}
	}

	msg := tgbotapi.NewMessage(chatID, "Объявление не найдено. Проверьте название и попробуйте снова.")
	bot.Send(msg)
}

var WaitingForFeedInput = make(map[int64]bool)

func HandleFeed(bot *tgbotapi.BotAPI, chatID int64) {
	WaitingForFeedInput[chatID] = true
	msg := tgbotapi.NewMessage(chatID, "Введите фильтр в формате: Тип товара, Местоположение (или оставьте пустым для всех объявлений)")
	bot.Send(msg)
}

func ProcessFeedRequest(bot *tgbotapi.BotAPI, chatID int64, filters string) {
	parts := strings.Split(filters, ",")
	filterType, filterLocation := "", ""

	if len(parts) > 0 {
		filterType = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		filterLocation = strings.TrimSpace(parts[1])
	}

	filteredAds := GetFilteredAds(filterType, filterLocation)
	if len(filteredAds) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Объявления не найдены.")
		bot.Send(msg)
		return
	}

	var response strings.Builder
	response.WriteString("🔍 Найденные объявления:\n")
	for _, ad := range filteredAds {
		response.WriteString(fmt.Sprintf("\n📌 *%s* (%s)\n📍 %s\n💰 %d руб.\n💬 %s\n",
			ad.Product.Name, ad.Product.Type, ad.Location, ad.Product.Price, ad.Description))
	}

	msg := tgbotapi.NewMessage(chatID, response.String())
	msg.ParseMode = "Markdown"
	bot.Send(msg)

	WaitingForFeedInput[chatID] = false
}

var WaitingForReviewInput = make(map[int64]bool)

func HandleReview(bot *tgbotapi.BotAPI, chatID int64) {
	WaitingForReviewInput[chatID] = true
	msg := tgbotapi.NewMessage(chatID, "Введите отзыв в формате:\nИмя, Текст, Оценка (1-5)")
	bot.Send(msg)
}

func ProcessReview(bot *tgbotapi.BotAPI, chatID int64, input string) {
	parts := strings.Split(input, ",")
	if len(parts) < 3 {
		msg := tgbotapi.NewMessage(chatID, "Ошибка! Используйте: Имя, Текст, Оценка (1-5)")
		bot.Send(msg)
		return
	}

	//targetName := strings.TrimSpace(parts[0])
	reviewText := strings.TrimSpace(parts[1])
	rating, _ := strconv.Atoi(strings.TrimSpace(parts[2]))

	mu.Lock()
	defer mu.Unlock()

	targetUser, exists := users[chatID]
	if !exists {
		msg := tgbotapi.NewMessage(chatID, "Пользователь не найден.")
		bot.Send(msg)
		return
	}

	targetUser.LeaveReview(users[chatID], reviewText, uint8(rating))

	msg := tgbotapi.NewMessage(chatID, "✅ Отзыв добавлен!")
	bot.Send(msg)

	WaitingForReviewInput[chatID] = false
}

var WaitingForRegistration = make(map[int64]bool)

func HandleRegister(bot *tgbotapi.BotAPI, chatID int64) {
	WaitingForRegistration[chatID] = true
	msg := tgbotapi.NewMessage(chatID, "Введите ваше имя для регистрации:")
	bot.Send(msg)
}

func ProcessRegistration(bot *tgbotapi.BotAPI, chatID int64, name string) {
	mu.Lock()
	defer mu.Unlock()

	// Проверяем, не занято ли имя
	for _, u := range users {
		if u.Name == name {
			msg := tgbotapi.NewMessage(chatID, "Это имя уже занято, выберите другое.")
			bot.Send(msg)
			return
		}
	}

	// Создаём нового пользователя или обновляем существующего
	user, exists := users[chatID]
	if !exists {
		user = &User{ID: chatID, Name: name, Rating: 0}
		users[chatID] = user
	} else {
		user.Name = name
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Вы зарегистрированы как %s!", name))
	bot.Send(msg)

	WaitingForRegistration[chatID] = false
}
