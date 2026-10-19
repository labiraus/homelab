/** UI contracts adapt the existing /api/worlds responses; they grant no authority.
 * @typedef {'ask'|'create'|'change'|'play'|'check'|'narrative'} Intent
 * @typedef {'canon_query'|'canon_proposal'|'story_turn'|'narrative_generate'|'consistency_check'} Workflow
 * @typedef {{worldId:string,timeline:string,revision:number,at?:string}} Scope
 * @typedef {{action:'put'|'delete',id?:string,record?:{id:string,kind:string,[key:string]:unknown}}} Operation
 * @typedef {{answer:string,baseRevision:number,workflow:Workflow,citations:string[],operations:Operation[],unknown?:boolean,sessionId?:string,sessionVersion?:number,memorySummary?:string,choices?:Array<{id:string,label:string,direction:string}>}} ReasonResult
 * @typedef {{id:string,role:'user'|'assistant',intent:Intent,scope:Scope,text:string,result?:ReasonResult}} Message
 */
export const sections = {
  workspace: "Workspace",
  explore: "Explore",
  drafts: "Drafts & Proposals",
  stories: "Stories",
  settings: "World Settings",
};
export const workflows = {
  ask: "canon_query",
  create: "canon_proposal",
  change: "canon_proposal",
  play: "story_turn",
  check: "consistency_check",
  narrative: "narrative_generate",
};
export function readWorldRoute(hash = window.location.hash) {
  try {
    const [path, query = ""] = hash.replace(/^#/, "").split("?");
    const [, root, worldId, section] = path.split("/");
    if (root !== "worlds") return {};
    return {
      worldId: decodeURIComponent(worldId || ""),
      section: Object.hasOwn(sections, section) ? section : "workspace",
      params: new URLSearchParams(query),
    };
  } catch {
    return {};
  }
}
export function worldHref(worldId, section = "workspace", params = {}) {
  const query = new URLSearchParams(
    Object.entries(params).filter(
      ([, v]) => v !== undefined && v !== null && v !== "",
    ),
  );
  return `#/worlds/${encodeURIComponent(worldId)}/${section}${query.size ? `?${query}` : ""}`;
}
export function citationScope(citation, worldId) {
  if (typeof citation !== "string") return null;
  const parts = citation.split(":");
  if (
    parts.length !== 4 ||
    parts[0] !== worldId ||
    !/^\d+$/.test(parts[1]) ||
    !Number.isSafeInteger(Number(parts[1])) ||
    !parts[2] ||
    !parts[3]
  )
    return null;
  return {
    worldId,
    revision: Number(parts[1]),
    timeline: parts[2],
    id: parts[3],
  };
}
export function detectIntent(text) {
  if (/^(create|invent|design|draft)\b/i.test(text.trim())) return "create";
  if (
    /^(change|replace|revise|update|remove|delete|suggest a change)\b/i.test(
      text.trim(),
    )
  )
    return "change";
  if (/^(play|start an adventure|continue (the )?story)\b/i.test(text.trim()))
    return "play";
  if (/^(check|find inconsistencies)\b/i.test(text.trim())) return "check";
  if (
    /^(who|what|when|where|why|how|is|are|did|does|tell|show|explain|list|explore)\b|\?\s*$/i.test(
      text.trim(),
    )
  )
    return "ask";
  return null; // Ambiguous assertions require an explicit intent before inference.
}
export function conversationQuestion(text, messages, references) {
  const previous = messages
    .filter((m) => m.role === "user")
    .slice(-3)
    .map((m) => m.text.slice(0, 350));
  const context = references
    .map((r) => `${r.id} (revision ${r.revision}, timeline ${r.timeline})`)
    .join(", ");
  const prefix = [
    previous.length
      ? `Earlier user requests (conversation context, not canon):\n${previous.join("\n")}`
      : "",
    context
      ? `Referenced record IDs (retrieve authorised evidence; do not assume facts): ${context}`
      : "",
  ]
    .filter(Boolean)
    .join("\n");
  const result = prefix ? `${prefix}\nCurrent request:\n${text}` : text;
  if (new TextEncoder().encode(result).length > 4000)
    throw new Error(
      "This request and its context exceed the 4,000-byte limit. Shorten the message or start a new conversation.",
    );
  return result;
}

export function humanLabel(value = "") {
  const labels = {
    world_settings: "World settings",
 established_account: "Established account",
 author: "Author",
 reader: "Reader",
    entity: "Character or place",
    type: "Record category",
    fact: "Fact",
    relationship: "Relationship",
    source: "Source",
    timeline: "Timeline",
    event: "Event",
    rule: "World rule",
    ready_for_review: "Ready for review",
  };
  return (
    labels[value] ||
    value
      .replace(/([a-z])([A-Z])/g, "$1 $2")
      .replaceAll("_", " ")
      .replace(/^./, (c) => c.toUpperCase())
  );
}
