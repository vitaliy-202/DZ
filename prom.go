package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Метрики в текстовом формате Prometheus (без внешних библиотек).
// Prometheus опрашивает /metrics/prometheus; тот же набор данных в JSON — на /metrics.

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func head(w io.Writer, name, help, typ string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
}

func simple(w io.Writer, name, help, typ string, v float64) {
	head(w, name, help, typ)
	fmt.Fprintf(w, "%s %s\n", name, num(v))
}

func prometheusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	simple(w, "app_ready", "1, если приложение работает.", "gauge", 1)
	simple(w, "app_uptime_seconds", "Сколько секунд работает приложение.", "gauge", time.Since(startedAt).Seconds())
	simple(w, "app_visits_last_hour", "Посещений главной страницы за последние 60 минут.", "gauge", float64(getVisitsCount()))
	simple(w, "app_notes", "Сколько заметок хранится.", "gauge", float64(db.count()))
	simple(w, "go_goroutines", "Количество горутин.", "gauge", float64(runtime.NumGoroutine()))
	simple(w, "go_memstats_alloc_bytes", "Байт выделено и используется.", "gauge", float64(m.Alloc))
	simple(w, "go_memstats_sys_bytes", "Байт получено от ОС.", "gauge", float64(m.Sys))
	writeProcessMetrics(w)
}

// writeProcessMetrics читает /proc (только Linux). На других ОС просто ничего не выводит.
func writeProcessMetrics(w io.Writer) {
	if raw, err := os.ReadFile("/proc/self/statm"); err == nil {
		if f := strings.Fields(string(raw)); len(f) >= 2 {
			if pages, err := strconv.ParseFloat(f[1], 64); err == nil {
				simple(w, "process_resident_memory_bytes", "Физическая память процесса (RSS).", "gauge", pages*float64(os.Getpagesize()))
			}
		}
	}
	if raw, err := os.ReadFile("/proc/self/stat"); err == nil {
		s := string(raw)
		if i := strings.LastIndex(s, ")"); i >= 0 {
			f := strings.Fields(s[i+1:])
			if len(f) > 12 { // utime — 14-е поле, stime — 15-е (после "pid (name)" это индексы 11 и 12)
				ut, e1 := strconv.ParseFloat(f[11], 64)
				st, e2 := strconv.ParseFloat(f[12], 64)
				if e1 == nil && e2 == nil {
					simple(w, "process_cpu_seconds_total", "Процессорное время процесса (user+system).", "counter", (ut+st)/100)
				}
			}
		}
	}
	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		simple(w, "process_open_fds", "Количество открытых файловых дескрипторов.", "gauge", float64(len(entries)))
	}
	writeCgroupMetrics(w)
}

// ---- ресурсы контейнера (cgroup) ----
// Внутри Docker-контейнера /sys/fs/cgroup — это cgroup самого контейнера, так что
// приложение может честно рассказать, сколько ресурсов ему выдано (лимит) и сколько
// оно использует. Если лимита нет или это не Linux, соответствующие метрики не выводятся.

// readFloatFile читает число из файла; "max" и значения «практически бесконечность» = лимита нет.
func readFloatFile(path string) (float64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	if s == "max" || s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > 1<<60 { // cgroup v1 пишет ~9.2e18, когда лимита нет
		return 0, false
	}
	return v, true
}

// parseCPUMax разбирает cgroup v2 cpu.max: "max 100000" (без лимита) или "50000 100000" (0.5 ядра).
func parseCPUMax(s string) (float64, bool) {
	f := strings.Fields(s)
	if len(f) != 2 || f[0] == "max" {
		return 0, false
	}
	quota, e1 := strconv.ParseFloat(f[0], 64)
	period, e2 := strconv.ParseFloat(f[1], 64)
	if e1 != nil || e2 != nil || quota <= 0 || period <= 0 {
		return 0, false
	}
	return quota / period, true
}

func cgroupCPULimit() (float64, bool) {
	if raw, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil { // v2
		return parseCPUMax(string(raw))
	}
	quota, ok1 := readFloatFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us") // v1: -1 = без лимита
	period, ok2 := readFloatFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if ok1 && ok2 && quota > 0 && period > 0 {
		return quota / period, true
	}
	return 0, false
}

func writeCgroupMetrics(w io.Writer) {
	if v, ok := readFloatFile("/sys/fs/cgroup/memory.current"); ok { // v2
		simple(w, "app_container_memory_usage_bytes", "Память, которую сейчас использует контейнер (cgroup).", "gauge", v)
	} else if v, ok := readFloatFile("/sys/fs/cgroup/memory/memory.usage_in_bytes"); ok { // v1
		simple(w, "app_container_memory_usage_bytes", "Память, которую сейчас использует контейнер (cgroup).", "gauge", v)
	}
	if v, ok := readFloatFile("/sys/fs/cgroup/memory.max"); ok { // v2
		simple(w, "app_container_memory_limit_bytes", "Лимит памяти контейнера (cgroup); нет метрики — лимита нет.", "gauge", v)
	} else if v, ok := readFloatFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok { // v1
		simple(w, "app_container_memory_limit_bytes", "Лимит памяти контейнера (cgroup); нет метрики — лимита нет.", "gauge", v)
	}
	if v, ok := cgroupCPULimit(); ok {
		simple(w, "app_container_cpu_limit_cores", "Лимит CPU контейнера в ядрах; нет метрики — лимита нет.", "gauge", v)
	}
}
