import { useEffect, useState } from "react";

export default function WorldUsers({ api }) {
  const [account, setAccount] = useState(null);
  const [users, setUsers] = useState([]);
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [more, setMore] = useState(false);
  const [grantReason, setGrantReason] = useState("");
  useEffect(() => {
    let active = true;
    api("/account")
      .then(async (row) => {
        if (!active) return;
        setAccount(row);
        if (row.superuser) {
          const rows = await api("/users");
          if (active) {
            setUsers(rows);
            setMore(rows.length === 100);
          }
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api]);
  async function add(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/users", { email, displayName: name });
      const rows = await api("/users");
      setUsers(rows);
      setMore(rows.length === 100);
      setEmail("");
      setName("");
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  if (!account?.superuser)
    return error ? (
      <p role="alert">{error}</p>
    ) : account ? (
      <p>
        Account administration is available to Administrators. World members and
        roles are managed in World Settings.
      </p>
    ) : (
      <p role="status">Loading account access…</p>
    );
  return (
    <details>
      <summary>Administrator · Manage users</summary>
      <p>
        You can manage users and record explicit publication overrides. Revision
        checks, validation and immutable established accounts always apply.
      </p>
      {error && <p role="alert">{error}</p>}
      <form onSubmit={add}>
        <fieldset disabled={busy}>
          <legend>Add a user</legend>
          <label>
            Google account email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              maxLength={254}
              required
            />
          </label>
          <label>
            Display name
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={200}
              required
            />
          </label>
          <button type="submit">Add user</button>
        </fieldset>
      </form>
      <p>
        New users can sign in with Google. Add them to a world using its
        membership controls. Adding a user does not grant Administrator status.
      </p>
      <label>
        Administrator role change reason
        <input
          value={grantReason}
          onChange={(e) => setGrantReason(e.target.value)}
          maxLength={2000}
        />
      </label>
      <ul aria-label="Registered users">
        {users.map((u) => (
          <li key={u.email}>
            {u.displayName} · {u.email}
            {u.superuser ? " · Administrator" : ""}
            <button
              disabled={!grantReason || busy}
              onClick={async () => {
                setBusy(true);
                setError("");
                try {
                  await api("/administrators", {
                    principal: u.email,
                    enabled: !u.superuser,
                    reason: grantReason,
                  });
                  setUsers(await api("/users"));
                  setGrantReason("");
                } catch (e) {
                  setError(e.message);
                } finally {
                  setBusy(false);
                }
              }}
            >
              {u.superuser ? "Remove Administrator" : "Make Administrator"}
            </button>
          </li>
        ))}
      </ul>
      {more && (
        <button
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            setError("");
            try {
              const rows = await api(`/users?offset=${users.length}`);
              setUsers([...users, ...rows]);
              setMore(rows.length === 100);
            } catch (e) {
              setError(e.message);
            } finally {
              setBusy(false);
            }
          }}
        >
          Load more users
        </button>
      )}
    </details>
  );
}
