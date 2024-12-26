package main

import (
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var baseURL string

// HTML шаблон для главной страницы
const homePageHTML = `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Сервис сокращения ссылок</title>
    <link rel="stylesheet" href="/styles.css">
</head>
<body>
    <header>
        <h1>Сервис сокращения ссылок</h1>
    </header>
    <main>
        <h2>Введите ссылку, чтобы сократить её</h2>
        <div class="form-container">
            <form action="/shorten" method="get">
                <input type="text" name="url" placeholder="Введите ссылку" required>
                <button type="submit">Сократить</button>
            </form>
        </div>
		<p><a href="/admin">Перейти в Админ Панель</a></p>
    </main>
	<footer>
        <p>© 2024 Сервис сокращения ссылок. Все права защищены.</p>
    </footer>
</body>
</html>
`

// HTML шаблон для страницы с результатом
const resultPageHTML = `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Результат сокращения</title>
    <link rel="stylesheet" href="/styles.css">
</head>
<body>
    <header>
        <h1>Сокращенная ссылка готова!</h1>
    </header>
    <main>
        <div class="form-container">
            <p>Ваша сокращенная ссылка: <a href="{{.}}" target="_blank">{{.}}</a></p>
            <p><a href="/home">Назад</a></p>
        </div>
    </main>
	<footer>
        <p>© 2024 Сервис сокращения ссылок. Все права защищены.</p>
    </footer>
</body>
</html>
`

// HTML шаблон для страницы AdminPage
const adminPageHTML = `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Админ Панель</title>
    <link rel="stylesheet" href="/styles.css">
</head>
<body>
    <header>
        <h1>Админ Панель</h1>
    </header>
    <main>
		<div class="form-container">
        <h2>Существующие ссылки</h2>
		<p><a href="/home">На главную</a></p>
		<div class="table-container">
        <table>
            <thead>
                <tr>
                    <th>ID</th>
                    <th>Оригинальная ссылка</th>
                    <th>Короткая ссылка</th>
                </tr>
            </thead>
            <tbody>
                {{range .}}
                <tr>
                    <td>{{.ID}}</td>
                    <td><a href="{{.OriginalURL}}" target="_blank">{{.OriginalURL}}</a></td>
                    <td><a href="{{.ShortURL}}" target="_blank">{{.ShortURL}}</a></td>
                </tr>
                {{end}}
            </tbody>
        </table>
		</div>
		</div>
    </main>
	<footer>
        <p>© 2024 Сервис сокращения ссылок. Все права защищены.</p>
    </footer>
</body>
</html>
`

// Структура для передачи данных в шаблон AdminPage
type URLRecord struct {
	ID          int
	OriginalURL string
	ShortURL    string
}

// Обработчик для страницы AdminPage
func adminPageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем все записи из базы данных
		rows, err := db.Query("SELECT id, original_url, short_url FROM urls")
		if err != nil {
			http.Error(w, "Ошибка при извлечении данных из базы", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Сохраняем результаты в срез
		var records []URLRecord
		for rows.Next() {
			var record URLRecord
			if err := rows.Scan(&record.ID, &record.OriginalURL, &record.ShortURL); err != nil {
				http.Error(w, "Ошибка при обработке данных", http.StatusInternalServerError)
				return
			}
			record.ShortURL = fmt.Sprintf("%s/%s", baseURL, record.ShortURL) // Формируем полный короткий URL
			records = append(records, record)
		}

		// Проверяем на ошибки при итерации
		if err := rows.Err(); err != nil {
			http.Error(w, "Ошибка при чтении данных из базы", http.StatusInternalServerError)
			return
		}

		// Отображаем страницу AdminPage
		w.Header().Set("Content-Type", "text/html")
		tmpl, _ := template.New("adminPage").Parse(adminPageHTML)
		tmpl.Execute(w, records)
	}
}

func loadBaseURL() {
	// Чтение URL из файла mainurl.txt
	data, err := ioutil.ReadFile("mainurl.txt")
	if err != nil {
		log.Fatal("Error reading mainurl.txt: ", err)
	}

	// Убираем лишние пробелы и символы новой строки
	baseURL = strings.TrimSpace(string(data))
	if baseURL == "" {
		log.Fatal("Base URL is empty in mainurl.txt")
	}
}

func initializeDatabase() *sql.DB {
	// Подключение к базе данных SQLite
	db, err := sql.Open("sqlite3", "shorturls.db")
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	// Создание таблицы, если она не существует
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_url TEXT NOT NULL,
		short_url TEXT NOT NULL UNIQUE,
		key TEXT NOT NULL UNIQUE
	)`) // Таблица с уникальными ключами
	if err != nil {
		log.Fatal("Failed to create table: ", err)
	}

	return db
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Отображение главной страницы
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(homePageHTML))
}

func shortenHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем оригинальный URL
		originalURL := r.URL.Query().Get("url")
		if originalURL == "" {
			http.Error(w, "URL не указан", http.StatusBadRequest)
			return
		}

		// Проверяем, существует ли уже ссылка
		var shortURL string
		row := db.QueryRow("SELECT key FROM urls WHERE original_url = ?", originalURL)
		if err := row.Scan(&shortURL); err == nil {
			// Если ссылка найдена, возвращаем её
			shortenedURL := fmt.Sprintf("%s/%s", baseURL, shortURL)
			w.Header().Set("Content-Type", "text/html")
			tmpl, _ := template.New("result").Parse(resultPageHTML)
			tmpl.Execute(w, shortenedURL)
			return
		}

		// Создаем новый уникальный ключ для короткой ссылки
		for {
			shortURL = uuid.New().String()[:8]
			if err := db.QueryRow("SELECT id FROM urls WHERE key = ?", shortURL).Scan(new(int)); err == sql.ErrNoRows {
				break // Генерируем, пока не найдём уникальный ключ
			}
		}

		// Сохраняем новую пару в базу данных
		_, err := db.Exec("INSERT INTO urls (original_url, short_url, key) VALUES (?, ?, ?)", originalURL, shortURL, shortURL)
		if err != nil {
			http.Error(w, "Ошибка при сохранении ссылки", http.StatusInternalServerError)
			return
		}

		// Формируем полный URL для короткой ссылки
		shortenedURL := fmt.Sprintf("%s/%s", baseURL, shortURL)

		// Отображаем страницу с результатом
		w.Header().Set("Content-Type", "text/html")
		tmpl, _ := template.New("result").Parse(resultPageHTML)
		tmpl.Execute(w, shortenedURL)
	}
}

func redirectHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем короткую ссылку (часть URL после "/")
		shortURL := strings.TrimPrefix(r.URL.Path, "/")

		// Ищем оригинальный URL по короткой ссылке
		var originalURL string
		row := db.QueryRow("SELECT original_url FROM urls WHERE key = ?", shortURL)
		if err := row.Scan(&originalURL); err != nil {
			http.Error(w, "Короткая ссылка не найдена", http.StatusNotFound)
			return
		}

		// Перенаправляем на оригинальный URL
		http.Redirect(w, r, originalURL, http.StatusFound)
	}
}

func main() {
	// Загружаем базовый URL из файла
	loadBaseURL()

	// Инициализация базы данных
	db := initializeDatabase()
	defer db.Close()

	// Обработчики маршрутов
	http.HandleFunc("/home", homeHandler)       // Главная страница
	http.HandleFunc("/shorten", shortenHandler(db)) // Сокращение URL
	http.HandleFunc("/", redirectHandler(db))       // Перенаправление
	http.HandleFunc("/admin", adminPageHandler(db)) // Админ Панель

	// Обслуживание статики (например, styles.css)
	http.Handle("/styles.css", http.FileServer(http.Dir(".")))

	// Запуск HTTP-сервера
	fmt.Printf("Server is running on %s/home\n", baseURL)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
