package mcpauth

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDenied = errors.New("authorization refused")
var ErrLimited = errors.New("registration limit reached")

type Client struct {
	ID        string   `json:"client_id"`
	Name      string   `json:"client_name"`
	Redirects []string `json:"redirect_uris"`
}
type Authorization struct{ IDHash, ClientID, Email, Redirect, Resource, Challenge, State, CSRFHash string }
type CodeInput struct{ Hash, ClientID, Redirect, Resource, Challenge string }
type Credentials struct{ AccessHash, RefreshHash, Family string }
type Store interface {
	Register(context.Context, Client) error
	Client(context.Context, string) (Client, error)
	SaveAuthorization(context.Context, Authorization) error
	Consent(context.Context, string, string, string, string, bool) (Authorization, error)
	Exchange(context.Context, CodeInput, Credentials) error
	Rotate(context.Context, string, string, string, Credentials) error
	Check(context.Context, string, string) (string, error)
	Revoke(context.Context, string, string) error
}

type Postgres struct{ Pool *pgxpool.Pool }

func refused(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDenied
	}
	return err
}

func (s *Postgres) Register(ctx context.Context, c Client) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize the global admission bound across every external replica.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(603738282901)`); err != nil {
		return err
	}
	// Remove expired transient credentials and never-authorized registrations.
	// Approved client identities remain stable for later reauthorization.
	for _, query := range []string{
		`DELETE FROM mcp_oauth.authorization WHERE expires_at<=now()`,
		`DELETE FROM mcp_oauth.code WHERE expires_at<=now()`,
		`DELETE FROM mcp_oauth.token WHERE refresh_expires_at<=now() AND expires_at<=now()`,
		`DELETE FROM mcp_oauth.client c WHERE NOT approved AND created_at<now()-interval '1 hour' AND NOT EXISTS(SELECT 1 FROM mcp_oauth.authorization a WHERE a.client_id=c.id) AND NOT EXISTS(SELECT 1 FROM mcp_oauth.code a WHERE a.client_id=c.id) AND NOT EXISTS(SELECT 1 FROM mcp_oauth.token a WHERE a.client_id=c.id)`,
	} {
		if _, err = tx.Exec(ctx, query); err != nil {
			return err
		}
	}
	var total, recent int
	if err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE created_at>now()-interval '1 minute') FROM mcp_oauth.client`).Scan(&total, &recent); err != nil {
		return err
	}
	if total >= 1000 || recent >= 30 {
		return ErrLimited
	}
	raw, err := json.Marshal(c.Redirects)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mcp_oauth.client(id,name,redirect_uris) VALUES($1,$2,$3)`, c.ID, c.Name, raw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Postgres) Client(ctx context.Context, id string) (Client, error) {
	var c Client
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT id,name,redirect_uris FROM mcp_oauth.client WHERE id=$1 AND (approved OR created_at>now()-interval '1 hour')`, id).Scan(&c.ID, &c.Name, &raw)
	if err != nil {
		return c, refused(err)
	}
	err = json.Unmarshal(raw, &c.Redirects)
	return c, err
}
func (s *Postgres) SaveAuthorization(ctx context.Context, a Authorization) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO mcp_oauth.authorization(id_hash,client_id,email,redirect_uri,resource,challenge,state,csrf_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now()+interval '10 minutes')`, a.IDHash, a.ClientID, a.Email, a.Redirect, a.Resource, a.Challenge, a.State, a.CSRFHash)
	return err
}
func (s *Postgres) Consent(ctx context.Context, idHash, csrfHash, email, codeHash string, allow bool) (Authorization, error) {
	var a Authorization
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return a, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `DELETE FROM mcp_oauth.authorization WHERE id_hash=$1 AND csrf_hash=$2 AND email=$3 AND expires_at>now() RETURNING client_id,email,redirect_uri,resource,challenge,state`, idHash, csrfHash, email).Scan(&a.ClientID, &a.Email, &a.Redirect, &a.Resource, &a.Challenge, &a.State)
	if err != nil {
		return a, refused(err)
	}
	if allow {
		if _, err = tx.Exec(ctx, `UPDATE mcp_oauth.client SET approved=true WHERE id=$1`, a.ClientID); err != nil {
			return a, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO mcp_oauth.code(code_hash,client_id,email,redirect_uri,resource,challenge,expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+interval '2 minutes')`, codeHash, a.ClientID, a.Email, a.Redirect, a.Resource, a.Challenge)
		if err != nil {
			return a, err
		}
	}
	return a, tx.Commit(ctx)
}
func (s *Postgres) Exchange(ctx context.Context, c CodeInput, t Credentials) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var email string
	// DELETE RETURNING makes the PKCE-bound code single-use even across replicas.
	err = tx.QueryRow(ctx, `DELETE FROM mcp_oauth.code WHERE code_hash=$1 AND client_id=$2 AND redirect_uri=$3 AND resource=$4 AND challenge=$5 AND expires_at>now() RETURNING email`, c.Hash, c.ClientID, c.Redirect, c.Resource, c.Challenge).Scan(&email)
	if err != nil {
		return refused(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO mcp_oauth.token(access_hash,refresh_hash,family,client_id,email,resource,expires_at,refresh_expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+interval '15 minutes',now()+interval '7 days')`, t.AccessHash, t.RefreshHash, t.Family, c.ClientID, email, c.Resource)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Postgres) Rotate(ctx context.Context, hash, client, resource string, t Credentials) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var family string
	err = tx.QueryRow(ctx, `SELECT family FROM mcp_oauth.token WHERE refresh_hash=$1 AND client_id=$2 AND resource=$3`, hash, client, resource).Scan(&family)
	if err != nil {
		return refused(err)
	}
	// Every family mutation takes this lock before any row lock. Re-read after
	// acquiring it: a concurrent rotation or revocation may have committed.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,603738282901))`, family); err != nil {
		return err
	}
	var revoked, expired bool
	err = tx.QueryRow(ctx, `SELECT revoked,refresh_expires_at<=now() FROM mcp_oauth.token WHERE refresh_hash=$1 AND client_id=$2 AND resource=$3 FOR UPDATE`, hash, client, resource).Scan(&revoked, &expired)
	if err != nil {
		return refused(err)
	}
	if revoked {
		if _, err = tx.Exec(ctx, `UPDATE mcp_oauth.token SET revoked=true WHERE family=$1`, family); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		return ErrDenied
	}
	if expired {
		return ErrDenied
	}
	_, err = tx.Exec(ctx, `INSERT INTO mcp_oauth.token(access_hash,refresh_hash,family,client_id,email,resource,expires_at,refresh_expires_at) SELECT $1,$2,family,client_id,email,resource,now()+interval '15 minutes',refresh_expires_at FROM mcp_oauth.token WHERE refresh_hash=$3`, t.AccessHash, t.RefreshHash, hash)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE mcp_oauth.token SET revoked=true WHERE refresh_hash=$1`, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Postgres) Check(ctx context.Context, hash, resource string) (string, error) {
	var email string
	err := s.Pool.QueryRow(ctx, `SELECT t.email FROM mcp_oauth.token t JOIN auth.users u ON u.email=t.email WHERE access_hash=$1 AND resource=$2 AND NOT revoked AND expires_at>now()`, hash, resource).Scan(&email)
	return email, refused(err)
}
func (s *Postgres) Revoke(ctx context.Context, hash, client string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var family string
	err = tx.QueryRow(ctx, `SELECT family FROM mcp_oauth.token WHERE (access_hash=$1 OR refresh_hash=$1) AND client_id=$2`, hash, client).Scan(&family)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,603738282901))`, family); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE mcp_oauth.token SET revoked=true WHERE family=$1`, family); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
