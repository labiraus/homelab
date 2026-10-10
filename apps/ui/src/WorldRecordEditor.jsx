import { useState } from "react";

const kinds = [
  "entity",
  "type",
  "source",
  "timeline",
  "fact",
  "relationship",
  "event",
  "rule",
  "world_settings",
];
const ruleFields = {
  life_bounds: {
    birth: "Birth predicate",
    death: "Death predicate",
    exceptions: "Exception predicates (comma separated, optional)",
  },
  relationship_cardinality: {
    min: "Minimum simultaneous relationships",
    max: "Maximum simultaneous relationships",
  },
  exclusive_facts: {
    predicates: "Exclusive boolean predicates (comma separated)",
  },
};
const valueTypes = ["string", "number", "boolean", "entity"];
const entries = (value) =>
  Object.entries(value ?? {}).map(([name, value]) => ({ name, value }));

function mapFromRows(rows, caption, allowEmpty = false) {
  const result = {};
  for (const row of rows) {
    if (!row.name.trim() || (!allowEmpty && !row.value.trim()))
      throw new Error(`${caption} requires both a name and value.`);
    if (Object.hasOwn(result, row.name))
      throw new Error(`${caption} contains duplicate name ${row.name}.`);
    Object.defineProperty(result, row.name, {
      value: row.value,
      enumerable: true,
    });
  }
  return result;
}

function MapEditor({
  title,
  rows,
  setRows,
  options,
  identifier = false,
  allowEmpty = false,
}) {
  return (
    <fieldset>
      <legend>{title}</legend>
      {rows.map((row, index) => (
        <div key={index}>
          <label>
            {title} name {index + 1}
            <input
              required
              value={row.name}
              pattern={identifier ? "[a-zA-Z0-9_-]{1,64}" : undefined}
              onChange={(e) =>
                setRows(
                  rows.map((r, i) =>
                    i === index ? { ...r, name: e.target.value } : r,
                  ),
                )
              }
            />
          </label>
          <label>
            {title} value {index + 1}
            {options ? (
              <select
                required
                value={row.value}
                onChange={(e) =>
                  setRows(
                    rows.map((r, i) =>
                      i === index ? { ...r, value: e.target.value } : r,
                    ),
                  )
                }
              >
                <option value="">Choose value</option>
                {!options.some((o) => o.id === row.value) && row.value && (
                  <option value={row.value}>{row.value}</option>
                )}
                {options.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.label || o.id}
                  </option>
                ))}
              </select>
            ) : (
              <input
                required={!allowEmpty}
                value={row.value}
                onChange={(e) =>
                  setRows(
                    rows.map((r, i) =>
                      i === index ? { ...r, value: e.target.value } : r,
                    ),
                  )
                }
              />
            )}
          </label>
          <button
            type="button"
            onClick={() => setRows(rows.filter((_, i) => i !== index))}
          >
            Remove {title.toLowerCase()} {index + 1}
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => setRows([...rows, { name: "", value: "" }])}
      >
        Add {title.toLowerCase()}
      </button>
    </fieldset>
  );
}

export default function WorldRecordEditor({
  records,
  timeline,
  record,
  onStage,
  onNew,
}) {
  const [draft, setDraft] = useState(() =>
    record ? structuredClone(record) : { id: "", kind: "entity", label: "" },
  );
  const [fields, setFields] = useState(() => entries(record?.fields));
  const [relations, setRelations] = useState(() => entries(record?.relations));
  const [attributes, setAttributes] = useState(() =>
    entries(record?.attributes).filter(
      (r) =>
        record?.kind !== "rule" ||
        ![
          "check",
          "birth",
          "death",
          "exceptions",
          "min",
          "max",
          "predicates",
        ].includes(r.name),
    ),
  );
  const [error, setError] = useState("");
  const set = (key, value) => setDraft({ ...draft, [key]: value });
  const scoped = ["fact", "relationship", "event"].includes(draft.kind);
  const byId = new Map(records.map((r) => [r.id, r]));
  const subjectType = byId.get(byId.get(draft.subject)?.type);
  const predicates =
    draft.kind === "fact" ? subjectType?.fields : subjectType?.relations;
  const types = records.filter((r) => r.kind === "type" && r.id !== draft.id);
  if (draft.kind === "type" && draft.id)
    types.push({ id: draft.id, label: draft.label || draft.id });
  function choose(caption, key, kind, required = true) {
    const options = records.filter((r) => r.kind === kind);
    return (
      <label>
        {caption}
        <select
          required={required}
          value={draft[key] ?? ""}
          onChange={(e) => set(key, e.target.value)}
        >
          <option value="">Choose {kind}</option>
          {draft[key] && !options.some((r) => r.id === draft[key]) && (
            <option value={draft[key]}>{draft[key]}</option>
          )}
          {options.map((r) => (
            <option key={r.id} value={r.id}>
              {r.label || r.id}
            </option>
          ))}
        </select>
      </label>
    );
  }
  function submit(e) {
    e.preventDefault();
    setError("");
    try {
      const candidate = {
        ...draft,
        attributes: mapFromRows(attributes, "Attributes", true),
      };
      if (scoped || draft.kind === "rule") {
        if (scoped) candidate.timeline = draft.timeline || timeline;
        for (const key of ["from", "until"]) {
          if (draft[key] === "" || draft[key] === undefined)
            delete candidate[key];
          else {
            const value = Number(draft[key]);
            if (!Number.isSafeInteger(value))
              throw new Error(
                "Fictional time must be an exact safe integer in the editor.",
              );
            candidate[key] = value;
          }
        }
      }
      if (draft.kind === "fact") {
        const type =
          subjectType?.fields?.[draft.predicate] ?? draft.valueType ?? "string";
        candidate.valueType = type;
        candidate.value =
          type === "number"
            ? Number(draft.value)
            : type === "boolean"
              ? String(draft.value) === "true"
              : draft.value;
        if (
          type === "number" &&
          (draft.value === "" ||
            draft.value === undefined ||
            !Number.isFinite(candidate.value))
        )
          throw new Error("A finite numeric fact value is required.");
      }
      if (draft.kind === "type") {
        candidate.fields = mapFromRows(fields, "Fields");
        candidate.relations = mapFromRows(relations, "Relationships");
      }
      if (draft.kind === "rule" && draft.attributes?.check) {
        candidate.attributes.check = draft.attributes.check;
        for (const key of Object.keys(
          ruleFields[draft.attributes.check] || {},
        )) {
          if (
            draft.attributes[key] !== undefined &&
            draft.attributes[key] !== ""
          )
            candidate.attributes[key] = draft.attributes[key];
        }
      }
      onStage(candidate);
    } catch (e) {
      setError(e.message);
    }
  }
  const factType =
    subjectType?.fields?.[draft.predicate] ?? draft.valueType ?? "string";
  return (
    <form onSubmit={submit} aria-label="Record editor">
      {error && <p role="alert">{error}</p>}
      {record && (
        <p>
          Editing {record.label || record.id}. All existing fields and
          attributes are loaded.
        </p>
      )}
      <label>
        Record kind
        <select
          value={draft.kind}
          disabled={!!record}
          onChange={(e) => {
            setDraft({ id: "", kind: e.target.value, label: "" });
            setFields([]);
            setRelations([]);
            setAttributes([]);
            setError("");
          }}
        >
          {kinds.map((kind) => (
            <option key={kind}>{kind}</option>
          ))}
        </select>
      </label>
      <label>
        Stable ID
        <input
          required
          readOnly={!!record}
          pattern="[a-zA-Z0-9_-]{1,64}"
          value={draft.id}
          onChange={(e) => set("id", e.target.value)}
        />
      </label>
      <label>
        Label
        <input
          required={[
            "entity",
            "source",
            "timeline",
            "event",
            "rule",
            "world_settings",
          ].includes(draft.kind)}
          maxLength={500}
          value={draft.label ?? ""}
          onChange={(e) => set("label", e.target.value)}
        />
      </label>
      {draft.kind === "entity" && choose("Entity type", "type", "type")}
      {draft.kind === "world_settings" &&
        choose("Default timeline", "timeline", "timeline")}
      {scoped && (
        <>
          {choose("Subject", "subject", "entity", draft.kind !== "event")}
          {choose("Source", "source", "source")}
          <label>
            Record timeline
            <input
              required
              value={draft.timeline ?? timeline}
              onChange={(e) => set("timeline", e.target.value)}
            />
          </label>
          <label>
            Valid from
            <input
              type="number"
              step="1"
              required={draft.kind === "event"}
              value={draft.from ?? ""}
              onChange={(e) => set("from", e.target.value)}
            />
          </label>
          <label>
            Valid until
            <input
              type="number"
              step="1"
              value={draft.until ?? ""}
              onChange={(e) => set("until", e.target.value)}
            />
          </label>
        </>
      )}
      {["fact", "relationship"].includes(draft.kind) && (
        <label>
          Predicate
          <select
            required
            value={draft.predicate ?? ""}
            onChange={(e) =>
              setDraft({ ...draft, predicate: e.target.value, value: "" })
            }
          >
            <option value="">Choose declared predicate</option>
            {draft.predicate &&
              !Object.hasOwn(predicates ?? {}, draft.predicate) && (
                <option value={draft.predicate}>{draft.predicate}</option>
              )}
            {Object.keys(predicates ?? {}).map((p) => (
              <option key={p}>{p}</option>
            ))}
          </select>
        </label>
      )}
      {draft.kind === "relationship" && choose("Target", "target", "entity")}
      {draft.kind === "fact" && (
        <>
          <p>Declared value type: {factType}</p>
          {factType === "entity" ? (
            choose("Value", "value", "entity")
          ) : (
            <label>
              Value
              {factType === "boolean" ? (
                <select
                  required
                  value={String(draft.value ?? "")}
                  onChange={(e) => set("value", e.target.value)}
                >
                  <option value="">Choose boolean</option>
                  <option value="true">true</option>
                  <option value="false">false</option>
                </select>
              ) : (
                <input
                  required
                  type={factType === "number" ? "number" : "text"}
                  step={factType === "number" ? "any" : undefined}
                  value={draft.value ?? ""}
                  onChange={(e) => set("value", e.target.value)}
                />
              )}
            </label>
          )}
        </>
      )}
      {draft.kind === "type" && (
        <>
          <MapEditor
            title="Fields"
            identifier
            rows={fields}
            setRows={setFields}
            options={valueTypes.map((id) => ({ id }))}
          />
          <MapEditor
            title="Relationships"
            identifier
            rows={relations}
            setRows={setRelations}
            options={types}
          />
        </>
      )}
      {draft.kind === "rule" && (
        <fieldset>
          <legend>Deterministic rule</legend>
          <label>
            Rule check
            <select
              value={draft.attributes?.check ?? ""}
              onChange={(e) =>
                set("attributes", {
                  ...draft.attributes,
                  check: e.target.value,
                })
              }
            >
              <option value="">Natural-language guidance only</option>
              <option value="life_bounds">Life bounds</option>
              <option value="relationship_cardinality">
                Relationship cardinality
              </option>
              <option value="exclusive_facts">Exclusive facts</option>
            </select>
          </label>
          {draft.attributes?.check && (
            <>
              {choose(
                "Rule subject type",
                "type",
                "type",
                draft.attributes.check !== "life_bounds",
              )}
              {choose(
                "Rule timeline",
                "timeline",
                "timeline",
                draft.attributes.check !== "life_bounds",
              )}
              {draft.attributes.check === "relationship_cardinality" && (
                <label>
                  Relationship predicate
                  <select
                    required
                    value={draft.predicate || ""}
                    onChange={(e) => set("predicate", e.target.value)}
                  >
                    <option value="">Choose declared relationship</option>
                    {Object.keys(byId.get(draft.type)?.relations || {}).map(
                      (p) => (
                        <option key={p}>{p}</option>
                      ),
                    )}
                  </select>
                </label>
              )}
              {Object.entries(ruleFields[draft.attributes.check] || {}).map(
                ([key, caption]) => (
                  <label key={key}>
                    {caption}
                    <input
                      required={["birth", "death", "predicates"].includes(key)}
                      value={draft.attributes[key] || ""}
                      onChange={(e) =>
                        set("attributes", {
                          ...draft.attributes,
                          [key]: e.target.value,
                        })
                      }
                    />
                  </label>
                ),
              )}
              {["from", "until"].map((key) => (
                <label key={key}>
                  {key === "from" ? "Rule valid from" : "Rule valid until"}
                  <input
                    type="number"
                    step="1"
                    value={draft[key] ?? ""}
                    onChange={(e) => set(key, e.target.value)}
                  />
                </label>
              ))}
            </>
          )}
        </fieldset>
      )}
      <MapEditor
        title="Attributes"
        allowEmpty
        rows={attributes}
        setRows={setAttributes}
      />
      <button>Stage record</button>
      <button type="button" onClick={onNew}>
        Start a new record
      </button>
      <p>
        Staging proposes a complete record replacement. Publishing requires
        independent review.
      </p>
    </form>
  );
}
