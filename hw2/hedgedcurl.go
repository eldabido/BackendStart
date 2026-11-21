package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Ответ.
type res struct {
	response string
	err      error
}

// fetch - функция для запроса.
func fetch(ctx context.Context, client *http.Client, url string, dir string) (string, error) {
	// Проверяем, что URL начинается с http(s)://, если нет, добавляем.
	lowerURL := strings.ToLower(url) // копию строки в нижнем регистре проверяем.

	if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
		url = "http://" + url
	}

	// Запрос с контекстом, чтобы отменять запрос.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	if req.URL.Hostname() == "" {
		return "", fmt.Errorf("missing host in URL: %q", url)
	}

	// Получаем ссылку и делаем Get запрос с учетом контекста.
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	// Закрываем тело ответа.
	defer resp.Body.Close()

	// Формируем ответ без размеров чанков.
	file, err := os.CreateTemp(dir, "response-*")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "%s %s\r\n", resp.Proto, resp.Status); err != nil {
		return "", err
	}

	// Убираем заголовок Transfer-Encoding.
	headers := resp.Header.Clone()
	headers.Del("Transfer-Encoding")

	if err := headers.Write(file); err != nil {
		return "", err
	}
	if _, err := io.WriteString(file, "\r\n"); err != nil {
		return "", err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}

	return file.Name(), nil

}

func run() int {
	var timeout int
	var help bool

	// Регистрируем флаги.
	flag.IntVar(&timeout, "t", 15, "timeout in seconds")
	flag.IntVar(&timeout, "timeout", 15, "timeout in seconds")

	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&help, "help", false, "help")

	flag.Parse()

	// Отдельная логика для help.
	if help {
		fmt.Println("Usage: hedgedcurl [-t SECONDS | --timeout SECONDS] URL...")
		fmt.Println("  -t, --timeout   timeout in seconds (default: 15)")
		fmt.Println("  -h, --help      help")
		return 0
	}
	// Получаем URLs.
	urls := flag.Args()
	maxTimeout := math.MaxInt64 / int64(time.Second)
	if timeout <= 0 || int64(timeout) > maxTimeout {
		fmt.Fprintf(os.Stderr, "Timeout should be between 1 and %d seconds\n", maxTimeout)
		return 1
	}

	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "no URLs")
		return 1
	}

	// Отправление запросов.
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	// Контекст для обработки первого запроса и отмены остальных.
	ctx, cancel := context.WithCancel(context.Background())

	// Создаем временную директорию для хранения ответов.
	dir, err := os.MkdirTemp("", "hedgedcurl-*")
	if err != nil {
		cancel()
		fmt.Fprintln(os.Stderr, "Failed to create temp dir:", err)
		return 1
	}

	// WaitGroup для завершения всех горутин, если не будем ждать, удаление может начаться, хотя в файлы еще пишут.
	var wg sync.WaitGroup
	// Удаляем папку со временными файлами после завершения работы.
	defer func() {
		cancel()
		wg.Wait()
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to remove temp dir:", err)
		}
	}()

	// Канал, через который передаем результат.
	results := make(chan res, len(urls))

	for _, url := range urls {
		wg.Add(1)
		// Параллельно запускаем все запросы к сайтам.
		go func(url string) {
			defer wg.Done()
			response, err := fetch(ctx, client, url, dir)
			results <- res{response: response, err: err}
		}(url)
	}

	var errs error
	var timeOut bool

	for i := 0; i < len(urls); i++ {
		result := <-results
		if result.err != nil {
			errs = result.err
			if os.IsTimeout(result.err) {
				timeOut = true
			}
			continue
		}
		// Как только выполнится самый первый, он отменит другие с помощью cancel().
		cancel()
		file, err := os.Open(result.response)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer file.Close()

		if _, err := io.Copy(os.Stdout, file); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if timeOut {
		fmt.Fprintln(os.Stderr, "timeout 228")
		return 228
	}
	fmt.Fprintln(os.Stderr, "Fail with all urls:", errs)
	return 1
}

func main() {
	os.Exit(run())
}
