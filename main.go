package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Структура заметки
type Note struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Хранилище в памяти с мьютексом для потокобезопасности
type Storage struct {
	mu     sync.RWMutex
	notes  map[int]Note
	nextID int
}

var db = Storage{
	notes:  make(map[int]Note),
	nextID: 1,
}

// Для подсчета посещений за последний час
var (
	muVisits sync.Mutex
	visits   []time.Time
)

func trackVisit() {
	muVisits.Lock()
	defer muVisits.Unlock()
	now := time.Now()
	// Очищаем старые посещения (старше 1 часа)
	var recent []time.Time
	for _, v := range visits {
		if now.Sub(v) < time.Hour {
			recent = append(recent, v)
		}
	}
	visits = append(recent, now)
}

func getVisitsCount() int {
	muVisits.Lock()
	defer muVisits.Unlock()
	now := time.Now()
	count := 0
	for _, v := range visits {
		if now.Sub(v) < time.Hour {
			count++
		}
	}
	return count
}

// HTML-шаблон интерфейса
var tmpl = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Менеджер задач</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; background: #f4f7f6; color: #333; }
        .card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 20px; }
        .metrics { display: flex; gap: 15px; flex-wrap: wrap; }
        .metric-box { background: #eef2f5; padding: 10px 15px; border-radius: 6px; flex: 1; min-width: 150px; }
        input, textarea { width: 100%; padding: 8px; margin: 5px 0 10px 0; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box; }
        button { background: #007bff; color: white; border: none; padding: 10px 15px; border-radius: 4px; cursor: pointer; }
        button:hover { background: #0056b3; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #a71d2a; }
        .note-item { border-bottom: 1px solid #eee; padding: 10px 0; }
    </style>
</head>
<body>
    <h1>Менеджер задач</h1>

    <!-- Блок метрик -->
    <div class="card">
        <h3>📊 Метрики и мониторинг</h3>
        <div class="metrics">
            <div class="metric-box">
                <strong>Health Check:</strong> <span style="color: green;">● OK</span>
            </div>
            <div class="metric-box">
                <strong>Посещений за час:</strong> {{.Visits}}
            </div>
            <div class="metric-box">
                <strong>RAM (Память):</strong> {{.MemAlloc}} MB
            </div>
            <div class="metric-box">
                <strong>CPU Ядер:</strong> {{.NumCPU}}
            </div>
        </div>
        <p style="margin-top: 15px; font-size: 12px; color: #666;">
            API метрики доступны в формате JSON: <a href="/metrics" target="_blank">/metrics</a> | 
            Health: <a href="/health" target="_blank">/health</a>
        </p>
    </div>

    <!-- Форма добавления заметки -->
    <div class="card">
        <h3>✍️ Добавить новую заметку</h3>
        <form action="/add" method="POST">
            <label>Заголовок:</label>
            <input type="text" name="title" required>
            <label>Текст заметки:</label>
            <textarea name="content" rows="3" required></textarea>
            <button type="submit">Сохранить</button>
        </form>
    </div>

    <!-- Список заметок (Редактирование / Удаление) -->
    <div class="card">
        <h3>📋 Список заметок</h3>
        {{if .Notes}}
            {{range .Notes}}
            <div class="note-item">
                <form action="/edit" method="POST" style="display:inline;">
                    <input type="hidden" name="id" value="{{.ID}}">
                    <input type="text" name="title" value="{{.Title}}" style="width: 30%; display:inline-block;">
                    <input type="text" name="content" value="{{.Content}}" style="width: 45%; display:inline-block;">
                    <button type="submit">Изменить</button>
                </form>
                <form action="/delete" method="POST" style="display:inline;">
                    <input type="hidden" name="id" value="{{.ID}}">
                    <button type="submit" class="btn-danger">Удалить</button>
                </form>
            </div>
            {{end}}
        {{else}}
            <p>Заметок пока нет. Создайте первую выше!</p>
        {{end}}
    </div>
</body>
</html>
`))

// Главная страница
func homeHandler(w http.ResponseWriter, r *http.Request) {
	trackVisit()

	db.mu.RLock()
	notesList := make([]Note, 0, len(db.notes))
	for _, n := range db.notes {
		notesList = append(notesList, n)
	}
	db.mu.RUnlock()

	// Сбор данных о памяти
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memAllocMB := m.Alloc / 1024 / 1024

	data := struct {
		Visits   int
		MemAlloc uint64
		NumCPU   int
		Notes    []Note
	}{
		Visits:   getVisitsCount(),
		MemAlloc: memAllocMB,
		NumCPU:   runtime.NumCPU(),
		Notes:    notesList,
	}

	tmpl.Execute(w, data)
}

// Добавление заметки
func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		content := r.FormValue("content")

		db.mu.Lock()
		db.notes[db.nextID] = Note{ID: db.nextID, Title: title, Content: content}
		db.nextID++
		db.mu.Unlock()
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Редактирование заметки
func editHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var id int
		fmt.Sscanf(r.FormValue("id"), "%d", &id)
		title := r.FormValue("title")
		content := r.FormValue("content")

		db.mu.Lock()
		if note, exists := db.notes[id]; exists {
			note.Title = title
			note.Content = content
			db.notes[id] = note
		}
		db.mu.Unlock()
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Удаление заметки
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var id int
		fmt.Sscanf(r.FormValue("id"), "%d", &id)

		db.mu.Lock()
		delete(db.notes, id)
		db.mu.Unlock()
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Health Check эндпоинт
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}

// JSON Метрики эндпоинт
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := map[string]interface{}{
		"status":          "UP",
		"visits_per_hour": getVisitsCount(),
		"ram_alloc_mb":    m.Alloc / 1024 / 1024,
		"ram_sys_mb":      m.Sys / 1024 / 1024,
		"num_cpu":         runtime.NumCPU(),
		"goroutines":      runtime.NumGoroutine(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/edit", editHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/metrics", metricsHandler)

	port := ":8080"
	log.Printf("Сервер запущен на http://localhost%s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}