package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func resetDB(t *testing.T) {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	db.mu.Lock()
	db.notes = map[int]Note{}
	db.nextID = 1
	db.mu.Unlock()
	muVisits.Lock()
	visits = nil
	muVisits.Unlock()
}

func post(h http.HandlerFunc, form url.Values) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h(httptest.NewRecorder(), req)
}

func TestNotesSurviveRestart(t *testing.T) {
	resetDB(t)
	post(addHandler, url.Values{"title": {"a"}, "content": {"b"}})
	post(addHandler, url.Values{"title": {"c"}, "content": {"d"}})
	post(editHandler, url.Values{"id": {"1"}, "title": {"a2"}, "content": {"b2"}})
	post(deleteHandler, url.Values{"id": {"2"}})

	// «перезапуск»: стираем память и читаем файл
	db.mu.Lock()
	db.notes = map[int]Note{}
	db.nextID = 1
	db.mu.Unlock()
	if err := db.load(dataFile()); err != nil {
		t.Fatal(err)
	}
	if db.count() != 1 || db.notes[1].Title != "a2" {
		t.Fatalf("данные не сохранились: %+v", db.notes)
	}
	post(addHandler, url.Values{"title": {"e"}, "content": {"f"}})
	if _, ok := db.notes[3]; !ok { // id не переиспользуется после удаления
		t.Fatalf("ожидался id 3: %+v", db.notes)
	}
}

func TestOnlyRootCountsAsVisit(t *testing.T) {
	resetDB(t)
	homeHandler(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	rec := httptest.NewRecorder()
	homeHandler(rec, httptest.NewRequest("GET", "/favicon.ico", nil))
	if rec.Code != 404 {
		t.Fatalf("/favicon.ico: код %d, ожидали 404", rec.Code)
	}
	if got := getVisitsCount(); got != 1 {
		t.Fatalf("посещений %d, ожидали 1", got)
	}
}

func TestHealthAndMetrics(t *testing.T) {
	resetDB(t)
	rec := httptest.NewRecorder()
	healthHandler(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "healthy") {
		t.Fatalf("/health: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	metricsHandler(rec, httptest.NewRequest("GET", "/metrics", nil))
	for _, k := range []string{`"visits_per_hour"`, `"ram_alloc_mb"`, `"notes"`} {
		if !strings.Contains(rec.Body.String(), k) {
			t.Errorf("в JSON-метриках нет %s: %s", k, rec.Body)
		}
	}
	rec = httptest.NewRecorder()
	prometheusHandler(rec, httptest.NewRequest("GET", "/metrics/prometheus", nil))
	for _, k := range []string{"app_visits_last_hour", "app_ready 1", "go_goroutines", "# TYPE app_notes gauge"} {
		if !strings.Contains(rec.Body.String(), k) {
			t.Errorf("в формате Prometheus нет %q", k)
		}
	}
}

func TestParseCPUMax(t *testing.T) {
	if v, ok := parseCPUMax("50000 100000"); !ok || v != 0.5 {
		t.Fatalf("got %v %v", v, ok)
	}
	if _, ok := parseCPUMax("max 100000"); ok {
		t.Fatal("max = без лимита")
	}
}
