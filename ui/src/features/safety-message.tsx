import { createContext, useContext } from "react";
import { isSafetyCheckMessage } from "../lib/safety-state.ts";

export const SafetyChecklistContext = createContext<(() => void) | null>(null);

export function SafetyMessage({ message }: { message?: string }) {
  const openChecklist = useContext(SafetyChecklistContext);
  if (!openChecklist || !isSafetyCheckMessage(message)) return message;
  return (
    <>
      <span className="safety-message-text">{message}</span>
      <button
        type="button"
        className="text-button safety-checklist-link"
        onClick={openChecklist}
      >
        查看检查清单
      </button>
    </>
  );
}
