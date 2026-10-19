import { useEffect, useRef, useState } from "react";
export default function WorldAdministration({ api, path, world, onSaved }) {
  const [members, setMembers] = useState([]),
    [more, setMore] = useState(false),
    [action, setAction] = useState("set_member"),
    [principal, setPrincipal] = useState(""),
    [role, setRole] = useState("author"),
    [reason, setReason] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [policy, setPolicy] = useState(
    world.publicationPolicy || { mode: "independent", requiredApprovals: 1 },
  );
  const retry = useRef(null);
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
    const body = {
      expectedVersion: world.adminVersion,
      action,
      reason,
      ...(["set_member", "remove_member"].includes(action)
        ? { principal }
        : {}),
      ...(action === "set_member" ? { role } : {}),
      ...(action === "set_policy" ? { policy } : {}),
    };
    const signature = JSON.stringify(body);
    if (retry.current?.signature !== signature)
      retry.current = { signature, id: crypto.randomUUID() };
    try {
      await api(`${path}/administration`, {
        ...body,
        requestId: retry.current.id,
      });
      await onSaved();
      setReason("");
      retry.current = null;
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <fieldset disabled={busy}>
      <legend>World administration</legend>
      <p>
        Authors own this world collectively. At least one Author must remain.
      </p>
      <p>
        {world.archived
          ? "Archived: history remains readable; new canon and story writes are paused."
          : "Active world"}
      </p>
      {error && <p role="alert">{error}</p>}
      <ul>
        {members.map((m) => (
          <li key={m.principal}>
            {m.principal} · {m.role}
          </li>
        ))}
      </ul>
      {more && (
        <button
          onClick={async () => {
            try {
              const rows = await api(
                `${path}/members?limit=100&offset=${members.length}`,
              );
              setMembers([...members, ...rows]);
              setMore(rows.length === 100);
            } catch (e) {
              setError(e.message);
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
            <option value="archive">Archive world</option>
            <option value="unarchive">Reopen world</option>
            {world.superuser && (
              <option value="set_policy">Set publication policy</option>
            )}
          </select>
        </label>
        {["set_member", "remove_member"].includes(action) && (
          <label>
            Member email
            <input
              required
              type="email"
              value={principal}
              onChange={(e) => setPrincipal(e.target.value)}
            />
          </label>
        )}
        {action === "set_member" && (
          <label>
            Role
            <select value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="author">Author</option>
              <option value="reader">Reader</option>
            </select>
          </label>
        )}
        {action === "set_policy" && (
          <>
            <label>
              Publication policy
              <select
                value={policy.mode}
                onChange={(e) => setPolicy({ ...policy, mode: e.target.value })}
              >
                <option value="independent">Independent Author review</option>
                <option value="direct">Direct Author publication</option>
                <option value="unanimous">Unanimous Authors</option>
              </select>
            </label>
            {policy.mode === "independent" && (
              <label>
                Required Author approvals
                <input
                  required
                  type="number"
                  min={1}
                  max={5}
                  value={policy.requiredApprovals}
                  onChange={(e) =>
                    setPolicy({
                      ...policy,
                      requiredApprovals: Number(e.target.value),
                    })
                  }
                />
              </label>
            )}
            <p>
              Changing policy invalidates pending proposal authority. Existing
              published history remains unchanged.
            </p>
          </>
        )}
        <label>
          Administration reason
          <textarea
            required
            maxLength={2000}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </label>
        <button>Apply administration action</button>
      </form>
    </fieldset>
  );
}
