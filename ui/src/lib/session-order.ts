import type { Tab, Session, TabMetadata } from "./types.ts";
import { tabDetails } from "./workspace-state.ts";
export const sessionKey = (tab: Tab) =>
  tab.cli_session_id ? `session:${tab.cli_session_id}` : tab.id;

export function projectSessionRows(
  tabs: Tab[],
  sessions: Session[],
  saved: TabMetadata,
  manualOrder: string[] | undefined,
  baseline: Map<string, number>,
) {
  const live = tabs.filter(
    (tab) =>
      !tab.closed ||
      !sessions.some((session) => session.id === tab.cli_session_id),
  );
  const rows = [
    ...sessions.map(
      (session) =>
        live.find((tab) => tab.cli_session_id === session.id) || {
          id: `session:${session.id}`,
          kind: "terminal",
          workspace: session.workspace,
          title: session.title,
          cli_session_id: session.id,
          historySession: session,
          claudeAccountNumber: session.claudeAccountNumber,
        },
    ),
    ...live.filter(
      (tab) => !sessions.some((session) => session.id === tab.cli_session_id),
    ),
  ].map((tab) => {
    const session = sessions.find(
      (session) => session.id === tab.cli_session_id,
    );
    return {
      ...tab,
      claudeAccountNumber: session?.account_id
        ? session.claudeAccountNumber
        : tab.claudeAccountNumber,
      sessionActive: Boolean(session?.active),
      sessionRunningElsewhere: Boolean(tab.historySession && session?.running),
      sessionClaudeDesktop: Boolean(
        tab.historySession && session?.running && session?.claude_desktop,
      ),
      sessionPinned: Boolean(
        session ? session.pinned : tabDetails(saved, tab.id).pinned,
      ),
    };
  });
  for (const row of rows)
    if (!baseline.has(sessionKey(row)))
      baseline.set(sessionKey(row), baseline.size);
  return rows.sort((a, b) => {
    const rank = (row: Tab) =>
      manualOrder?.includes(sessionKey(row))
        ? manualOrder.indexOf(sessionKey(row))
        : (manualOrder?.length ?? 0);
    if (manualOrder)
      return (
        rank(a) - rank(b) ||
        baseline.get(sessionKey(a))! - baseline.get(sessionKey(b))!
      );
    return (
      Number(b.sessionActive) - Number(a.sessionActive) ||
      Number(b.sessionPinned) - Number(a.sessionPinned) ||
      baseline.get(sessionKey(a))! - baseline.get(sessionKey(b))!
    );
  });
}
