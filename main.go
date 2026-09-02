package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const weeksAhead = 4 // Выгрузка на 4 недели вперед
// Неа
// Цвета Google Calendar:
// "2" - Sage (Зеленый) для Лекций
// "6" - Tangerine (Оранжевый) для Практик
// "9" - Blueberry (Синий) для Лабораторных
// "5" - Banana (Желтый) по умолчанию
func getGoogleColorID(kind string) string {
	k := strings.ToLower(kind)
	switch {
	case strings.Contains(k, "лекц"):
		return "2"
	case strings.Contains(k, "практ"):
		return "6"
	case strings.Contains(k, "лабор"):
		return "9"
	default:
		return "5"
	}
}

type Lesson struct {
	Date        string `json:"date"`
	BeginLesson string `json:"beginLesson"`
	EndLesson   string `json:"endLesson"`
	Discipline  string `json:"discipline"`
	KindOfWork  string `json:"kindOfWork"`
	Lecturer    string `json:"lecturer"`
	Building    string `json:"building"`
	Auditorium  string `json:"auditorium"`
	SubGroup    string `json:"subGroup"`
}

func main() {
	// 1. Загрузка конфигурации из .env
	_ = godotenv.Load()

	defaultGroupID := getEnv("DEFAULT_GROUP_ID", "535")
	calID := getEnv("GOOGLE_CALENDAR_ID", "primary")

	reader := bufio.NewReader(os.Stdin)

	// 2. Интерактивный запрос параметров у пользователя
	fmt.Println("==================================================")
	fmt.Println("  Синхронизатор расписания ОмГТУ -> Google Calendar")
	fmt.Println("==================================================")

	fmt.Printf("Введите ID группы ОмГТУ [нажмите Enter для %s]: ", defaultGroupID)
	groupID, _ := reader.ReadString('\n')
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		groupID = defaultGroupID
	}

	fmt.Print("Выберите вашу подгруппу (1 или 2) [по умолчанию 2]: ")
	mySubGroup, _ := reader.ReadString('\n')
	mySubGroup = strings.TrimSpace(mySubGroup)
	if mySubGroup != "1" && mySubGroup != "2" {
		mySubGroup = "2"
	}

	fmt.Printf("\n⚙️ Выбрано: Группа ID=%s | Подгруппа=%s | Период=%d недели\n\n", groupID, mySubGroup, weeksAhead)

	ctx := context.Background()

	// 3. Авторизация в Google Calendar API
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("❌ Ошибка: Файл credentials.json не найден. См. инструкцию в README.md: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("❌ Ошибка чтения credentials.json: %v", err)
	}

	client := getClient(config)
	gSvc, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("❌ Ошибка инициализации сервиса Календаря: %v", err)
	}
	fmt.Println("✅ Подключение к Google Calendar API выполнено успешно!")

	// 4. Расчет диапазона дат (начиная с понедельника текущей недели)
	now := time.Now()
	offset := int(now.Weekday()) - 1
	if offset < 0 {
		offset = 6
	}
	monday := now.AddDate(0, 0, -offset)
	finishDate := monday.AddDate(0, 0, weeksAhead*7)

	start := monday.Format("2006.01.02")
	finish := finishDate.Format("2006.01.02")

	url := fmt.Sprintf("https://rasp.omgtu.ru/api/schedule/group/%s?start=%s&finish=%s&lng=1", groupID, start, finish)

	fmt.Printf("🚀 Загружаю расписание из ОмГТУ (%s — %s)...\n", start, finish)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	httpCli := &http.Client{}
	resp, err := httpCli.Do(req)
	if err != nil {
		log.Fatalf("❌ Ошибка при запросе к расписанию ОмГТУ: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("❌ Ошибка чтения ответа API: %v", err)
	}

	var lessons []Lesson
	if err := json.Unmarshal(body, &lessons); err != nil {
		log.Fatalf("❌ Ошибка парсинга JSON расписания: %v", err)
	}

	fmt.Printf("📦 Получено предметов из API: %d\n", len(lessons))

	loc, err := time.LoadLocation("Asia/Omsk")
	if err != nil {
		loc = time.FixedZone("OMST", 6*3600)
	}

	count := 0
	for _, l := range lessons {
		sg := strings.ToLower(l.SubGroup)
		kind := strings.ToLower(l.KindOfWork)

		// Фильтрация подгрупп:
		// 1. Лекции всегда идут для всей группы.
		// 2. Если поле subGroup пустое или содержит обе группы — пара берется.
		// 3. Исключаем пары явно чужой подгруппы.
		if !strings.Contains(kind, "лекц") && sg != "" {
			if mySubGroup == "1" && (strings.Contains(sg, "/2") || strings.Contains(sg, "2 подгр")) {
				continue
			}
			if mySubGroup == "2" && (strings.Contains(sg, "/1") || strings.Contains(sg, "1 подгр")) {
				continue
			}
		}

		// Формирование уникального ID (для защиты от дублей)
		uid := fmt.Sprintf("omgtu-%s-%s-%s", l.Date, l.BeginLesson, l.Discipline)
		uid = strings.NewReplacer(" ", "_", ".", "_", ":", "_", "/", "_").Replace(uid)

		layout := "2006.01.02 15:04"
		t1, err1 := time.ParseInLocation(layout, l.Date+" "+l.BeginLesson, loc)
		t2, err2 := time.ParseInLocation(layout, l.Date+" "+l.EndLesson, loc)
		if err1 != nil || err2 != nil {
			continue
		}

		event := &calendar.Event{
			Summary:     fmt.Sprintf("[%s] %s", l.KindOfWork, l.Discipline),
			Location:    fmt.Sprintf("%s, ауд. %s", l.Building, l.Auditorium),
			Description: fmt.Sprintf("Преподаватель: %s\nТип: %s\nПодгруппа: %s", l.Lecturer, l.KindOfWork, l.SubGroup),
			Start:       &calendar.EventDateTime{DateTime: t1.Format(time.RFC3339)},
			End:         &calendar.EventDateTime{DateTime: t2.Format(time.RFC3339)},
			ColorId:     getGoogleColorID(l.KindOfWork),
			ICalUID:     uid + "@omgtu.ru",
		}

		// Используем Import для предотвращения дублирования занятий
		_, err = gSvc.Events.Import(calID, event).Do()
		if err != nil {
			_, err = gSvc.Events.Insert(calID, event).Do()
			if err != nil {
				fmt.Printf("⚠️ Ошибка добавления занятия '%s': %v\n", l.Discipline, err)
				continue
			}
		}

		count++
		fmt.Printf("✅ Добавлено: [%s %s] %s (%s)\n", l.Date, l.BeginLesson, l.Discipline, l.KindOfWork)
	}

	fmt.Printf("\n🎉 Готово! Синхронизировано %d занятий в Google Календарь.\n", count)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("1. Перейдите по ссылке авторизации Google:\n%v\n\n2. Вставьте полученный код авторизации: ", authURL)
	var code string
	fmt.Scan(&code)
	tok, err := config.Exchange(context.TODO(), code)
	if err != nil {
		log.Fatalf("❌ Ошибка авторизации: %v", err)
	}
	return tok
}

func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("❌ Ошибка сохранения token.json: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
