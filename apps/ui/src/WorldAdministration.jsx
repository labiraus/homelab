import { useEffect, useRef, useState } from "react";

export default function WorldAdministration({ api, path, world, onSaved }) {
  const [members, setMembers] = useState([]);
  const [more, setMore] = useState(false);
  const [action, setAction] = useState("set_member");
  const [principal, setPrincipal] = useState("");
  const [role, setRole] = useState("editor");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const lastRequest = useRef(null);
  useEffect(() => {
    let active = true;
    api(`${path}/members?limit=100`)
      .then((rows) => {
        if (active) {
          setMembers(rows);
          setMore(rows.length === 100);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api, path, world.adminVersion]);
  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const request = {
      expectedVersion: world.adminVersion,
      action,
      reason,
      ...(["set_member", "remove_member", "transfer_owner"].includes(action)
        ? { principal }
        : {}),
      ...(action === "set_member" ? { role } : {}),
    };
    const signature = JSON.stringify(request);
    if (lastRequest.current?.signature !== signature)
      lastRequest.current = { signature, requestId: crypto.randomUUID() };
    try {
      await api(`${path}/administration`, {
        ...request,
        requestId: lastRequest.current.requestId,
      });
      await onSaved();
      setReason("");
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <fieldset disabled={busy}>
      <legend>World administration</legend>
      {error && <p role="alert">{error}</p>}
      <p>
        {world.archived
          ? "Archived: history remains readable; new canon and story writes are paused."
          : "Active world"}
      </p>
      <ul>
        {members.map((member) => (
          <li key={member.principal}>
            {member.principal} · {member.role}
          </li>
        ))}
      </ul>
      {more && (
        <button
          onClick={async () => {
            setBusy(true);
            setError("");
            try {
              const rows = await api(
                `${path}/members?limit=100&offset=${members.length}`,
              );
              setMembers([...members, ...rows]);
              setMore(rows.length === 100);
            } catch (e) {
              setError(e.message);
            } finally {
              setBusy(false);
            }
          }}
        >
          Load more members
        </button>
      )}
      <form onSubmit={submit}>
        <label>
          Administration action
          <select value={action} onChange={(e) => setAction(e.target.value)}>
            <option value="set_member">Set member role</option>
            <option value="remove_member">Revoke membership</option>
            <option value="transfer_owner">Transfer ownership</option>
            <option value="archive">Archive world</option>
            <option value="unarchive">Reopen world</option>
          </select>
        </label>
        {["set_member", "remove_member", "transfer_owner"].includes(action) && (
          <label>
            Member email
            <input
              type="email"
              required
              value={principal}
              onChange={(e) => setPrincipal(e.target.value)}
            />
          </label>
        )}
        {action === "set_member" && (
          <label>
            Role
            <select value={role} onChange={(e) => setRole(e.target.value)}>
              {["editor", "reviewer", "reader", "admin"].map((r) => (
                <option key={r}>{r}</option>
              ))}
            </select>
          </label>
        )}
        {action === "transfer_owner" && (
          <p>
            Choose an existing member. Your role becomes admin when ownership
            transfers.
          </p>
        )}
        <label>
          Administration reason
          <input
            required
            maxLength={2000}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </label>
        <button>Apply administration change</button>
      </form>
      <p>
        Names, default timelines and world settings are canonical records: edit
        the world settings record and obtain independent review.
      </p>
    </fieldset>
  );
}
