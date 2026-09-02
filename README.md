# 📅 OmGTU Schedule to Google Calendar Sync

Утилита на Go для автоматической выгрузки расписания занятий из API ОмГТУ (`rasp.omgtu.ru`) в ваш Google Календарь на 4 недели вперед.

## ✨ Особенности

- 🎯 **Выборочный импорт подгрупп:** поддержка выбора 1-й или 2-й подгруппы при запуске. Лекции и совместные занятия добавляются всегда.
- 🎨 **Цветовая индикация:** 
  - 🟢 **Зеленый** — Лекции
  - 🟠 **Оранжевый** — Практики
  - 🔵 **Синий** — Лабораторные работы
- 🛡️ **Защита от дубликатов:** повторный запуск не создает дублирующиеся события.
- 🕒 **Учет часового пояса:** автоматическая привязка к омскому времени (`Asia/Omsk`, UTC+6).

---

## 🚀 Быстрый старт

### 1. Требования
- Установленный [Go 1.20+](https://go.dev/)
- Учетная запись Google

### 2. Подготовка Google Calendar API
1. Перейдите в [Google Cloud Console](https://console.cloud.google.com/).
2. Создайте новый проект и включите **Google Calendar API**.
3. В разделе **OAuth consent screen** добавьте свой Google-аккаунт в список *Test users*.
4. В разделе **Credentials** нажмите `Create Credentials` -> `OAuth client ID` (тип приложения: **Desktop app**).
5. Скачайте JSON-файл ключа и переименуйте его в `credentials.json`, поместив в корень проекта.

### 3. Настройка проекта

```bash
# Клонируйте репозиторий
git clone [https://github.com/your-username/omgtu-calendar-sync.git](https://github.com/your-username/omgtu-calendar-sync.git)
cd omgtu-calendar-sync

# Создайте файл .env из шаблона
cp .env.example .env

# Установите зависимости
go mod tidy