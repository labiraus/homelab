package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"go-common/api"
	"go-common/base"
	"go-common/prometheusutil"
)

const (
	helloHandlerLabel = "helloHandler"
)

var configValue string

func main() {
	ctx := base.Init("goapi")
	done := api.Init(ctx)
	prometheusutil.Init()
	configValue = base.GetEnv("configValue", "unknown")
	http.HandleFunc("/hello", helloHandler)
	close(base.Ready)
	http.HandleFunc("/go/benchmarking", benchmarking)
	<-done
	slog.Info("finishing")
}

// Endpoint for benchmarking example
func benchmarking(w http.ResponseWriter, r *http.Request) {
	i := 100  //0.1 seconds
	m := 1000 //1 second
	randomMillis := rand.Intn(m-i+1) + i
	time.Sleep(time.Duration(randomMillis) * time.Millisecond)

	_, err := w.Write([]byte("benchmarking"))
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now() // Capture the start time
	prometheusutil.IncrementProcessed(helloHandlerLabel, "call")
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		if err != nil {
			slog.Error(err.Error())
			prometheusutil.IncrementProcessed(helloHandlerLabel, "error")
		}
		prometheusutil.OpDuration(helloHandlerLabel, time.Since(startTime))
	}()

	slog.Info("helloHandler called")

	user, err := GetUser(1)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := HelloResponse{Data: fmt.Sprintf("Hello %v! (called via golang), using %v", user.Username, configValue)}

	data, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
