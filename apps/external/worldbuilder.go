package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
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
	if err != nil || version != 6 || user != "worldbuilder_external" || elevated {
		pool.Close()
		return fmt.Errorf("Worldbuilder requires schema version 6 and nonprivileged worldbuilder_external login")
	}
	go func() { <-ctx.Done(); pool.Close() }()
	store := &worldbuilder.Store{Pool: pool, Reasoner: worldbuilderReasoner(), Discover: worldbuilder.OpenSearchDiscovery(os.Getenv("OPENSEARCH_BASE_URL"), os.Getenv("OPENSEARCH_USERNAME"), os.Getenv("OPENSEARCH_PASSWORD"))}
	if store.Reasoner != nil {
		budget := &worldbuilder.InferenceBudget{Currency: os.Getenv("WORLDBUILDER_AI_CURRENCY"), GatewayRoute: os.Getenv("WORLDBUILDER_AI_GATEWAY_ROUTE")}
		for name, target := range map[string]*int64{"WORLDBUILDER_AI_DAILY_REQUESTS": &budget.DailyRequests, "WORLDBUILDER_AI_CONCURRENT": &budget.Concurrent, "WORLDBUILDER_AI_REQUEST_MICROS": &budget.RequestMicros, "WORLDBUILDER_AI_DAILY_MICROS": &budget.DailyMicros} {
			*target, err = strconv.ParseInt(os.Getenv(name), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid inference budget setting %s", name)
			}
		}
		if err = budget.Validate(); err != nil {
			return err
		}
		config, e := pgxpool.ParseConfig(dsn)
		if e != nil {
			return e
		}
		config.MaxConns = 2
		config.MinConns = 0
		budget.Pool, err = pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			return err
		}
		go func() { <-ctx.Done(); budget.Pool.Close() }()
		store.Budget = budget
	}
	if strings.EqualFold(os.Getenv("WORLDBUILDER_MCP_ENABLED"), "true") {
		key := []byte(os.Getenv("WORLDBUILDER_MCP_KEY"))
		if len(key) < 32 {
			return fmt.Errorf("Worldbuilder MCP requires a dedicated delegation key of at least 32 bytes")
		}
		mux.Handle(worldbuilder.ToolsPath, observeWorldbuilder("worldbuilder_tools", worldbuilder.ToolsHandler(store, key, validateIdentityEmail)))
	}
	if strings.EqualFold(os.Getenv("WORLDBUILDER_JOBS_ENABLED"), "true") {
		token := os.Getenv("WORLDBUILDER_JOB_TOKEN")
		if len(token) < 32 || store.Reasoner == nil || store.Budget == nil {
			return fmt.Errorf("durable generation requires its dedicated worker token and configured inference budget")
		}
		store.JobsEnabled = true
		mux.Handle(worldbuilder.GenerationRunPath, observeWorldbuilder("worldbuilder_worker", worldbuilder.GenerationHandler(store, token, validateIdentityEmail)))
	}
	handler := worldbuilder.Handler(store, func(r *http.Request) (string, bool) {
		s, ok := api.AuthStatusFromContext(r.Context())
		return s.Email, ok && s.Valid
	})
	handler = observeWorldbuilder("worldbuilder", handler)
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
		request.Header.Set("X-Request-ID", input.RequestID)
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
