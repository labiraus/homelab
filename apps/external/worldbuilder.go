package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"external/worldbuilder"
	"pkg/api"

	"github.com/jackc/pgx/v5/pgxpool"
)

func registerWorldbuilder(ctx context.Context, mux *http.ServeMux) error {
	if !strings.EqualFold(os.Getenv("WORLDBUILDER_ENABLED"), "true") {
		return nil
	}
	dsn := strings.TrimSpace(os.Getenv("WORLDBUILDER_DATABASE_URL"))
	if dsn == "" {
		return fmt.Errorf("Worldbuilder requires its dedicated database connection")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("invalid Worldbuilder database configuration")
	}
	var version int
	var user string
	var elevated bool
	err = pool.QueryRow(ctx, `SELECT current_user, (SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname=current_user),max(version) FROM worldbuilder.schema_version`).Scan(&user, &elevated, &version)
	if err != nil || version != 2 || user != "worldbuilder_external" || elevated {
		pool.Close()
		return fmt.Errorf("Worldbuilder requires schema version 2 and nonprivileged worldbuilder_external login")
	}
	go func() { <-ctx.Done(); pool.Close() }()
	handler := worldbuilder.Handler(&worldbuilder.Store{Pool: pool, Reasoner: worldbuilderReasoner()}, func(r *http.Request) (string, bool) {
		s, ok := api.AuthStatusFromContext(r.Context())
		return s.Email, ok && s.Valid
	})
	mux.Handle("/api/worlds", handler)
	mux.Handle("/api/worlds/", handler)
	return nil
}

func worldbuilderReasoner() worldbuilder.Reasoner {
	token := strings.TrimSpace(os.Getenv("WORLDBUILDER_AI_TOKEN"))
	if token == "" || processorBaseURL() == "" {
		return nil
	}
	client := &http.Client{Timeout: 25 * time.Second}
	return func(ctx context.Context, input worldbuilder.ReasonRequest) (worldbuilder.ReasonResult, error) {
		var result worldbuilder.ReasonResult
		body, err := json.Marshal(input)
		if err != nil {
			return result, err
		}
		request, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(processorBaseURL(), "/")+"/worldbuilder/generate", bytes.NewReader(body))
		if err != nil {
			return result, err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err != nil {
			return result, &worldbuilder.Problem{Status: 502, Code: "inference", Message: "Inference is unavailable"}
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return result, &worldbuilder.Problem{Status: 502, Code: "inference", Message: "Inference failed validation or exceeded its budget"}
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, 100001))
		if err != nil || len(raw) > 100000 {
			return result, fmt.Errorf("invalid inference response")
		}
		err = json.Unmarshal(raw, &result)
		return result, err
	}
}
