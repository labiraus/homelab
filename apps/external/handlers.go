package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
	"time"
)

func userCountHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(userCountLabel, "call")
	defer func() {
		p := recover()
		if p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(userCountLabel, "error")
		}
		prometheusutil.OpDuration(userCountLabel, time.Since(startTime))
	}()

	count, err := fetchUserCount(r.Context())
	if err != nil {
		err = fmt.Errorf("error fetching user count: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not fetch user count"})
		return
	}

	response := HelloResponse{
		Data: fmt.Sprintf("There are %d users in the database.", count),
	}

	w.Header().Set("Content-Type", "application/json")
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		err = fmt.Errorf("error marshalling json response: %w", encodeErr)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
