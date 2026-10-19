import { useEffect, useState } from "react";
export default function WorldSceneState({
  api,
  path,
  session,
  value,
  onChange,
}) {
  const [entities, setEntities] = useState([]),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    const load = async () => {
      const rows = [];
      let page;
      do {
        page = await api(
          `${path}/canon?timeline=${encodeURIComponent(session.timeline)}&revision=${session.pinnedRevision}&kind=entity&limit=500&offset=${rows.length}`,
        );
        rows.push(...page.records);
      } while (rows.length < page.total);
      if (active)
        setEntities([
          ...new Map(
            [
              ...rows,
              ...Object.values(session.state || {}).filter(
                (r) => r.kind === "entity",
              ),
            ].map((r) => [r.id, r]),
          ).values(),
        ]);
    };
    void load().catch((e) => {
      if (active) setError(e.message);
    });
    return () => {
      active = false;
    };
  }, [api, path, session.id, session.pinnedRevision]);
  const options = entities.map((r) => (
    <option key={r.id} value={r.id}>
      {r.label || r.id}
    </option>
  ));
  return (
    <fieldset>
      <legend>Location and inventory</legend>
      <p>
        Saved atomically with this manual turn. Select published or session
        entities.
      </p>
      {error && <p role="alert">{error}</p>}
      <label>
        Current location
        <select
          value={value.location || ""}
          onChange={(e) => onChange({ ...value, location: e.target.value })}
        >
          <option value="">Unspecified</option>
          {options}
        </select>
      </label>
      {(value.inventory || []).map((item, index) => (
        <div key={index}>
          <label>
            Inventory item {index + 1}
            <select
              value={item.entityId}
              onChange={(e) =>
                onChange({
                  ...value,
                  inventory: value.inventory.map((v, i) =>
                    i === index ? { ...v, entityId: e.target.value } : v,
                  ),
                })
              }
            >
              <option value="">Choose entity</option>
              {options}
            </select>
          </label>
          <label>
            Quantity
            <input
              type="number"
              min="1"
              max="1000000"
              value={item.quantity}
              onChange={(e) =>
                onChange({
                  ...value,
                  inventory: value.inventory.map((v, i) =>
                    i === index
                      ? { ...v, quantity: Number(e.target.value) }
                      : v,
                  ),
                })
              }
            />
          </label>
          <button
            type="button"
            onClick={() =>
              onChange({
                ...value,
                inventory: value.inventory.filter((_, i) => i !== index),
              })
            }
          >
            Remove item {index + 1}
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() =>
          onChange({
            ...value,
            inventory: [
              ...(value.inventory || []),
              { entityId: "", quantity: 1 },
            ],
          })
        }
      >
        Add inventory item
      </button>
    </fieldset>
  );
}
