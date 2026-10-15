import { useEffect, useState } from "react";

export default function WorldUsers({ api }) {
  const [account, setAccount] = useState(null);
  const [users, setUsers] = useState([]);
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [more, setMore] = useState(false);
  useEffect(() => {
    let active = true;
    api("/account").then(async (row) => {
      if (!active) return;
      setAccount(row);
      if (row.superuser) {
        const rows = await api("/users");
        if (active) { setUsers(rows); setMore(rows.length === 100); }
      }
    }).catch((e) => { if (active) setError(e.message); });
    return () => { active = false; };
  }, [api]);
  async function add(e) {
    e.preventDefault(); setBusy(true); setError("");
    try {
      await api("/users", { email, displayName: name });
      const rows = await api("/users");
      setUsers(rows); setMore(rows.length === 100); setEmail(""); setName("");
    } catch (e) { setError(e.message); }
    finally { setBusy(false); }
  }
  if (!account?.superuser) return error ? <p role="alert">{error}</p> : account
    ? <p>Account administration is available to superusers. World members and roles are managed in World Settings.</p>
    : <p role="status">Loading account access…</p>;
  return <details>
    <summary>Superuser · Manage users</summary>
    <p>You can add users and approve your own proposals. Reviews remain recorded, and each world’s approval count still applies.</p>
    {error && <p role="alert">{error}</p>}
    <form onSubmit={add}>
      <fieldset disabled={busy}>
        <legend>Add a user</legend>
        <label>Google account email<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} maxLength={254} required /></label>
        <label>Display name<input value={name} onChange={(e) => setName(e.target.value)} maxLength={200} required /></label>
        <button type="submit">Add user</button>
      </fieldset>
    </form>
    <p>New users can sign in with Google. Add them to a world using its membership controls. Adding a user does not grant superuser status.</p>
    <ul aria-label="Registered users">{users.map((u) => <li key={u.email}>{u.displayName} · {u.email}{u.superuser ? " · Superuser" : ""}</li>)}</ul>
    {more && <button disabled={busy} onClick={async () => {
      setBusy(true); setError("");
      try { const rows = await api(`/users?offset=${users.length}`); setUsers([...users, ...rows]); setMore(rows.length === 100); }
      catch (e) { setError(e.message); } finally { setBusy(false); }
    }}>Load more users</button>}
  </details>;
}
