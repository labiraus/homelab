package worldbuilder

import (
	"context"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Account struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Superuser   bool   `json:"superuser"`
}

func (s *Store) Account(ctx context.Context, actor string) (Account, error) {
	a := Account{Email: actor}
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=$1)`, actor).Scan(&a.Superuser)
	return a, err
}

func (s *Store) accounts(ctx context.Context, actor string, limit, offset int64) ([]Account, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = requireSuperuser(ctx, tx, actor); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT u.email,u.display_name,s.principal IS NOT NULL FROM auth.users u LEFT JOIN worldbuilder.superuser s ON s.principal=u.email ORDER BY u.email LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Account{}
	for rows.Next() {
		var a Account
		if err = rows.Scan(&a.Email, &a.DisplayName, &a.Superuser); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}

func requireSuperuser(ctx context.Context, tx pgx.Tx, actor string) error {
	var authority *string
	err := tx.QueryRow(ctx, `SELECT worldbuilder.lock_superuser($1)::text`, actor).Scan(&authority)
	if err == nil && authority == nil {
		return forbidden
	}
	return err
}

func (s *Store) registerAccount(ctx context.Context, actor, email, name string) (Account, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 || name == "" || len(name) > 200 {
		return Account{}, bad("valid email and display name (1–200 characters) required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Account{}, err
	}
	defer tx.Rollback(ctx)
	if err = requireSuperuser(ctx, tx, actor); err != nil {
		return Account{}, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO auth.users(email,display_name) VALUES($1,$2) ON CONFLICT(email) DO NOTHING`, email, name)
	if err != nil {
		return Account{}, err
	}
	if tag.RowsAffected() == 1 {
		if _, err = tx.Exec(ctx, `INSERT INTO worldbuilder.account_audit(actor,action,principal,detail) VALUES($1,'user.register',$2,jsonb_build_object('displayName',$3::text))`, actor, email, name); err != nil {
			return Account{}, err
		}
	}
	a := Account{Email: email}
	if err = tx.QueryRow(ctx, `SELECT u.display_name,s.principal IS NOT NULL FROM auth.users u LEFT JOIN worldbuilder.superuser s ON s.principal=u.email WHERE u.email=$1`, email).Scan(&a.DisplayName, &a.Superuser); err != nil {
		return Account{}, err
	}
	return a, tx.Commit(ctx)
}
