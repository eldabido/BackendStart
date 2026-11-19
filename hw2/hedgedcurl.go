package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Ответ.
type res struct {
	response []byte
	err      error
}

// fetch - функция для запроса.
func fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	// Запрос с контекстом, чтобы отменять запрос.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Получаем ссылку и делаем Get запрос с учетом контекста.
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Закрываем тело ответа.
	defer resp.Body.Close()

	var buffer bytes.Buffer
	if err := resp.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil

}

func main() {
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
		return
	}
	// Получаем URLs.
	urls := flag.Args()

	if timeout <= 0 {
		fmt.Fprintln(os.Stderr, "Timeout negative")
		os.Exit(1)
	}

	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "no URLs")
		os.Exit(1)
	}

	// Отправление запросов.
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	// Контекст для обработки первого запроса и отмены остальных.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Канал, через который передаем результат.
	results := make(chan res, len(urls))

	for _, url := range urls {
		// Параллельно запускаем все запросы к сайтам.
		go func(url string) {
			response, err := fetch(ctx, client, url)
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
		if _, err := os.Stdout.Write(result.response); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if timeOut {
		fmt.Fprintln(os.Stderr, "timeout 228")
		os.Exit(228)
		}
	fmt.Fprintln(os.Stderr, "Fail with all urls:", errs)
	os.Exit(1)
}
