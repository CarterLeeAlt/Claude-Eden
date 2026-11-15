import React, { useCallback, useEffect, useRef, useState } from "react";
import { LoaderCircle } from "lucide-react";
import { api, native, nativeAvailable } from "./api";
import { setClientContext } from "./client-context.mjs";
import logo from "./logo.png";
import "./local-workbench.css";

export function LocalWorkbenchRoot({ children }) {
  const [context, setContext] = useState(null);
  const [error, setError] = useState("");
  const [readError, setReadError] = useState("");
  const [phase, setPhase] = useState("");
  const alive = useRef(false), identity = useRef(""), serial = useRef(0);
  const attempt = useRef(0), pending = useRef(null), automatic = useRef(false);
  const activePhase = useRef("");
  function changePhase(value) { activePhase.current = value; setPhase(value); }
  const refresh = useCallback(async () => {
    const query = ++serial.current;
    const value = await api("/client/context");
    if (!alive.current || query !== serial.current) return null;
    if (!["none", "local"].includes(value.mode)) throw new Error("后台未返回有效的本机工作台状态，请更新 Eden。");
    const next = `${value.mode}:${value.connection_id}`;
    if (identity.current && identity.current !== next) {
      attempt.current++;
      location.reload();
      return null;
    }
    identity.current = next;
    setClientContext(value);
    setContext(value);
    setReadError("");
    return value;
  }, []);
  useEffect(() => {
    alive.current = true;
    let timer;
    const poll = async () => {
      try { await refresh(); }
      catch (failure) { if (alive.current) setReadError(failure.message); }
      if (alive.current) timer = setTimeout(poll, 2000);
    };
    (async () => {
      if (location.hash) {
        await api("/session", { token: location.hash.slice(1) });
        history.replaceState(null, "", location.pathname);
      }
      window.dispatchEvent(new Event("eden-session-ready"));
      await poll();
    })().catch(failure => { if (alive.current) setReadError(failure.message); });
    return () => { alive.current = false; attempt.current++; clearTimeout(timer); };
  }, [refresh]);
  const ready = context?.mode === "local" && (!context.desktop_gateway || context.connection?.state === "connected");
  const connecting = Boolean(phase);
  const message = error || readError || (context?.connection?.state === "failed" ? context.connection.message : "");
  useEffect(() => {
    if (!ready && message && nativeAvailable()) native("workbenchReady").catch(() => {});
  }, [ready, message]);
  async function connect() {
    if (activePhase.current) return;
    const generation = context.connection_id;
    const request = ++attempt.current;
    const current = () => alive.current && request === attempt.current;
    changePhase("preparing"); setError("");
    try {
      const previous = pending.current || (context.candidate && { connection_id: generation, candidate_id: context.candidate.id });
      if (previous) await api("/client/cancel", previous);
      if (!current()) return;
      pending.current = null;
      let candidate = await api("/client/candidate", { connection_id: generation, mode: "local" });
      if (!current()) return;
      pending.current = { connection_id: generation, candidate_id: candidate.id };
      changePhase("connecting");
      while (current()) {
        if (candidate.connection?.state === "connected") {
          changePhase("committing");
          await api("/client/commit", { ...pending.current, local_action: "auto" });
          if (!current()) return;
          pending.current = null;
          await refresh();
          return;
        }
        if (["failed", "disconnected"].includes(candidate.connection?.state)) throw new Error(candidate.connection.message || "本机工作台连接未完成，请重试。");
        await new Promise(resolve => setTimeout(resolve, 350));
        if (!current()) return;
        const value = await refresh();
        if (!current()) return;
        if (!value) continue;
        if (value.connection_id !== generation || value.candidate?.id !== candidate.id) {
          pending.current = null;
          throw new Error("候选连接已变化，请重新连接本机工作台。");
        }
        candidate = value.candidate;
      }
    } catch (failure) { if (alive.current && request === attempt.current) setError(failure.message); }
    finally { if (current()) changePhase(""); }
  }
  async function cancel() {
    if (["preparing", "committing", "cancelling"].includes(activePhase.current)) return;
    const candidate = pending.current || (context.candidate && { connection_id: context.connection_id, candidate_id: context.candidate.id });
    if (!candidate) return;
    attempt.current++;
    changePhase("cancelling");
    try { await api("/client/cancel", candidate); pending.current = null; setError(""); await refresh(); }
    catch (failure) { if (alive.current) setError(failure.message); }
    finally { if (alive.current) changePhase(""); }
  }
  useEffect(() => {
    if (!context || ready || automatic.current) return;
    automatic.current = true;
    if (context.mode === "none") void connect();
  }, [context, ready]);
  return <div className="local-workbench-shell">
    {ready && readError && <div role="alert" className="local-workbench-error">{readError}</div>}
    {ready ? children : <main className="local-workbench-connect">
      <img src={logo} alt="" />
      <h1>{connecting || !context && !message ? "正在打开本机工作台" : "连接本机工作台"}</h1>
      <p>Windows 上的 Eden 管理本地 Debian 13 环境。</p>
      {(connecting || !context && !message) && <p role="status"><LoaderCircle className="spin" size={18} />{phase === "cancelling" ? "正在取消连接…" : "正在连接 Eden…"}</p>}
      {message && <p role="alert">{message}</p>}
      <div>{context && !connecting && <button className="primary" onClick={connect}>重新连接本机</button>}
        {!context && message && <button onClick={() => location.reload()}>重新加载</button>}
        {pending.current && <button disabled={["preparing", "committing", "cancelling"].includes(phase)} onClick={cancel}>取消此次连接</button>}</div>
    </main>}
  </div>;
}
