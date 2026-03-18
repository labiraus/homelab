package main

import (
	"context"
	"fmt"

	"pkg/postgresutil"
)

var fetchUserCount = getUserCount

func getUserCount(ctx context.Context) (int, error) {
	if postgresutil.QueryRow == nil {
		return 0, fmt.Errorf("postgres is not initialized")
	}

	var count int
	if err := postgresutil.QueryRow(ctx, "CALL auth.get_user_count()").Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}
