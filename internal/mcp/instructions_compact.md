# Pad MCP

Use Pad for project work: issues, plans, status, dependencies, conventions,
playbooks, notes, decisions, and handoffs.

Set or pass a workspace before workspace-scoped actions. Read an item before
updating it, use issue refs, and send only changed fields. Stop on structured
errors; fix the named input instead of retrying unchanged.

Tools are grouped by resource and selected with the required `action` enum.
Parameter descriptions identify action-specific requirements. Prefer bounded
lists and `pad_item` get with `agent=true`; request full bodies only when needed.
Use `fields` for typed remote field objects. Local stdio structured values that
cannot cross CLI `key=value` encoding are refused explicitly.

Load extra guidance only when needed through `pad_meta` action `agent-guide` or
the `pad://agent/guide/{topic}` resource. Bootstrap after selecting a workspace
to obtain schemas, conventions, roles, playbooks, and current work context.
