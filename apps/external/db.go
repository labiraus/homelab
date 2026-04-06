package main

import (
	"context"
	"fmt"
	"strings"

	"pkg/postgresutil"
)

var fetchUserCount = getUserCount
var validateIdentityEmail = getIdentityByEmail

func getUserCount(ctx context.Context) (int, error) {
	if postgresutil.QueryRow == nil {
		return 0, fmt.Errorf("postgres is not initialized")
	}

	var count int
	if err := postgresutil.QueryRow(ctx, "CALL auth.get_user_count(NULL)").Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func getIdentityByEmail(ctx context.Context, email string) (bool, string, error) {
	if postgresutil.QueryRow == nil {
		return false, "postgres is not initialized", fmt.Errorf("postgres is not initialized")
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	if normalizedEmail == "" {
		return false, "authenticated identity did not include an email address", nil
	}

	var exists bool
	if err := postgresutil.QueryRow(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM auth.users WHERE email = $1)",
		normalizedEmail,
	).Scan(&exists); err != nil {
		return false, "identity lookup failed", err
	}

	if !exists {
		return false, "email is not recognized", nil
	}

	return true, "", nil
}
