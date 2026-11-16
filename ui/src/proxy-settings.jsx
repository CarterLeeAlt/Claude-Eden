import { InlineSelect } from "./inline-select";
import React, { useEffect, useRef, useState } from "react";
import {
  Plus,
  Search,
  RefreshCw,
  Check,
  Activity,
  Server,
  Trash2,
  Pencil,
  LoaderCircle,
  Square,
  CircleHelp,
} from "lucide-react";
import { api } from "./api";
import { SafetyMessage } from "./safety-message";
import "./proxy.css";

const bytes = (value) => {
  const n = Math.max(0, Number(value) || 0);
  if (n < 1024) return `${n.toFixed(0)} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KiB`;
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MiB`;
  return `${(n / 1024 ** 3).toFixed(2)} GiB`;
};
const protocolNames = { http: "HTTP CONNECT", socks5: "SOCKS5", ssh: "SSH" };
const emptyNode = {
  name: "",
  type: "socks5",
  server: "",
  port: "1080",
  username: "",
  auth_method: "password",
  password: "",
  private_key: "",
  private_key_passphrase: "",
  host_key: "",
};

export function ProxySettings({
  status,
  onApplied,
  onConfirm,
  onDirty,
  initializing = false,
}) {
  const [proxy, setProxy] = useState(null);
  const [tab, setTab] = useState("nodes");
  const [search, setSearch] = useState("");
  const [form, setForm] = useState(null);
  const [draft, setDraft] = useState(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState("");
  const [notice, setNotice] = useState("");
  const [delays, setDelays] = useState({});
  const [connections, setConnections] = useState({ connections: [] });
  const [logs, setLogs] = useState([]);
  const [port, setPort] = useState("17896");
  const [rates, setRates] = useState({ up: 0, down: 0 });
  const [helpOpen, setHelpOpen] = useState(false);
  const alive = useRef(true),
    loading = useRef(false),
    latest = useRef(null),
    measuring = useRef(false),
    portDirty = useRef(false);

  const reload = async () => {
    const value = await api("/proxy");
    if (alive.current) {
      setProxy(value);
      if (!portDirty.current) setPort(String(value.port));
    }
    return value;
  };
  useEffect(() => {
    alive.current = true;
    const tick = async () => {
      if (loading.current) return;
      loading.current = true;
      try {
        await reload();
      } catch (failure) {
        if (alive.current) setError(failure.message);
      } finally {
        loading.current = false;
      }
    };
    tick();
    const timer = setInterval(tick, 4000);
    return () => {
      alive.current = false;
      clearInterval(timer);
    };
  }, []);
  useEffect(() => {
    if (tab !== "connections" && tab !== "diagnostics") return;
    let canceled = false;
    const tick = async () => {
      try {
        if (tab === "diagnostics") {
          const result = await api("/proxy/logs");
          if (!canceled) setLogs(result.lines);
        } else {
          const result = await api("/proxy/connections"),
            now = Date.now();
          if (canceled) return;
          if (latest.current) {
            const seconds = (now - latest.current.time) / 1000;
            if (seconds > 0)
              setRates({
                up: Math.max(
                  0,
                  (result.uploadTotal - latest.current.uploadTotal) / seconds,
                ),
                down: Math.max(
                  0,
                  (result.downloadTotal - latest.current.downloadTotal) /
                    seconds,
                ),
              });
          }
          latest.current = { ...result, time: now };
          setConnections(result);
        }
      } catch (failure) {
        if (!canceled) setError(failure.message);
      }
    };
    tick();
    const timer = setInterval(tick, 3000);
    return () => {
      canceled = true;
      clearInterval(timer);
      latest.current = null;
    };
  }, [tab]);

  const busy = Boolean(pending || status?.job?.running);
  const selectedID = draft ?? proxy?.selected ?? "";
  const selectionChanged = selectedID !== (proxy?.selected || "");
  useEffect(() => {
    onDirty?.(selectionChanged || Boolean(form));
    return () => onDirty?.(false);
  }, [onDirty, selectionChanged, Boolean(form)]);

  const mutate = async (action, input = {}) => {
    setError("");
    setNotice("");
    setPending(action);
    try {
      const job = await api(`/proxy/${action}`, input);
      let current;
      do {
        await new Promise((resolve) => setTimeout(resolve, 500));
        if (!alive.current) return;
        current = (await api("/status")).job;
        if (current.id !== job.id)
          throw new Error("操作已被其他任务替代，请查看当前状态");
      } while (current.running);
      portDirty.current = false;
      await reload();
      await onApplied();
      if (current.error) throw new Error(current.error);
      if (alive.current) {
        setForm(null);
        if (action === "select" || action === "delete") setDraft(null);
        setNotice(current.message || "已完成");
        setDelays({});
      }
    } catch (failure) {
      if (alive.current) setError(failure.message);
    } finally {
      if (alive.current) setPending("");
    }
  };
  const applyNode = async () => {
    if (
      (!proxy.active || Number(port) !== proxy.port) &&
      status.vm_running &&
      !(await onConfirm({
        title: "启用代理并重启环境？",
        body: "首次启用或修改代理端口需要重启，当前终端会结束。后续切换节点保留终端进程，并重新建立网络连接。",
        confirm: "启用并重启",
      }))
    )
      return;
    await mutate("select", { node_id: selectedID, port: Number(port) });
  };
  const measure = async (ids) => {
    if (measuring.current) return;
    measuring.current = true;
    setPending("delay");
    setError("");
    try {
      let index = 0;
      await Promise.all(
        Array.from({ length: Math.min(4, ids.length) }, async () => {
          while (index < ids.length && alive.current) {
            const id = ids[index++];
            try {
              const result = await api("/proxy/delay", { node_id: id });
              if (alive.current)
                setDelays((old) => ({ ...old, [id]: `${result.delay} ms` }));
            } catch {
              if (alive.current)
                setDelays((old) => ({ ...old, [id]: "不可用 / 未载入" }));
            }
          }
        }),
      );
    } finally {
      measuring.current = false;
      if (alive.current) setPending("");
    }
  };
  const openEditor = (node) => {
    setError("");
    setNotice("");
    setForm(
      node
        ? {
            ...emptyNode,
            ...node,
            port: String(node.port),
            auth_method: node.auth_method || "password",
            password: "",
            private_key: "",
            private_key_passphrase: "",
            host_key: (node.host_key || []).join("\n"),
          }
        : { ...emptyNode },
    );
  };
  const ssh = form?.type === "ssh";
  const privateKey = ssh && form?.auth_method === "private-key";
  const formValid = Boolean(
    form &&
    form.name.trim() &&
    form.server.trim() &&
    Number.isInteger(Number(form.port)) &&
    Number(form.port) > 0 &&
    Number(form.port) <= 65535 &&
    (!ssh ||
      (form.username.trim() &&
        (privateKey
          ? form.private_key.includes("PRIVATE KEY") ||
            (!form.private_key && form.has_private_key)
          : form.password || form.has_password))),
  );
  const submit = (event) => {
    event.preventDefault();
    if (busy || !formValid) return;
    const node = {
      name: form.name.trim(),
      type: form.type,
      server: form.server.trim(),
      port: Number(form.port),
      username: form.username.trim(),
    };
    if (ssh) {
      node.auth_method = form.auth_method;
      node.host_key = form.host_key
        .split(/\r?\n/)
        .map((key) => key.trim())
        .filter(Boolean);
    }
    if (privateKey) {
      node.private_key =
        form.private_key || (form.has_private_key ? undefined : "");
      node.private_key_passphrase =
        form.private_key_passphrase ||
        (form.has_private_key_passphrase && !form.private_key ? undefined : "");
    } else {
      node.password = !node.username
        ? ""
        : form.password || (form.has_password ? undefined : "");
    }
    mutate(form.id ? "edit" : "add", {
      ...(form.id ? { node_id: form.id } : {}),
      node,
    });
  };
  const changeAuthentication = (changes) =>
    setForm((old) => {
      if (changes.type === old.type || changes.auth_method === old.auth_method)
        return old;
      return {
        ...old,
        ...changes,
        password: "",
        private_key: "",
        private_key_passphrase: "",
        has_password: false,
        has_private_key: false,
        has_private_key_passphrase: false,
      };
    });

  if (!proxy)
    return (
      <div className="proxy-loading" role={error ? "alert" : "status"}>
        {error || "正在读取代理配置…"}
      </div>
    );
  const allNodes = proxy.nodes;
  const selected = allNodes.find((node) => node.id === proxy.selected);
  const proposed = allNodes.find((node) => node.id === selectedID);
  const nodes = allNodes.filter((node) =>
    `${node.name} ${node.type} ${node.server}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const validPort =
    Number.isInteger(Number(port)) &&
    Number(port) >= 1024 &&
    Number(port) <= 65535;

  return (
    <div className="proxy-manager">
      <div className="section-title">
        <div className="proxy-heading">
          <h3>内置代理</h3>
          <button
            type="button"
            className="icon-button proxy-help-toggle"
            aria-label="代理使用说明"
            title={helpOpen ? "收起代理使用说明" : "查看代理使用说明"}
            aria-expanded={helpOpen}
            aria-controls="proxy-help"
            onClick={() => setHelpOpen((value) => !value)}
          >
            <CircleHelp size={17} aria-hidden="true" />
          </button>
        </div>
        <p>手动添加 HTTP CONNECT、SOCKS5 或 SSH 节点，选择一个供客机使用。</p>
      </div>
      {helpOpen && (
        <section
          id="proxy-help"
          className="proxy-help"
          aria-labelledby="proxy-help-title"
        >
          <h4 id="proxy-help-title">代理使用说明</h4>
          <p>
            Claude Eden 自带 Mihomo
            核心。准备自己的服务器地址、端口和认证信息，即可添加节点。
          </p>
          <ol className="proxy-help-steps">
            <li>
              <strong>添加节点</strong>
              <p>
                点击“添加节点”，填写连接信息并保存。SSH
                可使用密码或完整私钥内容。
              </p>
            </li>
            <li>
              <strong>选择并应用</strong>
              <p>
                选择一个节点，再点击“应用代理”。保存节点与应用节点是两个步骤，切换会关闭旧网络连接并重新检查出口。
              </p>
            </li>
            <li>
              <strong>验证联网</strong>
              <p>
                {initializing
                  ? "点击“开始安装”，等待客机安装与检查完成。"
                  : "启动环境并等待网络检查通过。"}
                “测速”显示节点延迟，实际联网以工作台检查状态为准。
              </p>
            </li>
          </ol>
          <p className="proxy-help-notes">
            代理故障时客机绝不直连。节点不可用时检查服务器与认证信息，或选择另一个节点并重新验证出口。
          </p>
        </section>
      )}
      <div className="proxy-summary">
        <Server size={22} />
        <div>
          <strong>
            {selected ? `当前节点 · ${selected.name}` : "尚未应用代理节点"}
          </strong>
          <span>
            {proxy.active
              ? proxy.running
                ? "Claude Eden 自带核心运行中"
                : "核心未就绪"
              : "请选择节点后应用，未配置时无法联网"}{" "}
            · 独立端口 {proxy.port}
          </span>
        </div>
        <span className={`proxy-badge ${proxy.network_open ? "online" : ""}`}>
          {proxy.state_label}
        </span>
      </div>
      {proxy.active && status.location && (
        <p className="field-help">
          最近检测出口：{status.location.ip} · {status.location.country}{" "}
          {status.location.city}。联网是否放行以工作台检查状态为准。
        </p>
      )}
      {!proxy.core_available && (
        <div role="alert" className="error-message">
          安装包缺少随附代理核心，请重新构建或安装完整 Claude Eden 应用。
        </div>
      )}
      <p className="field-help">
        客机公网 IPv4 TCP 和 DNS
        经选定节点转发；代理故障只会请求失败或断网。公网 UDP、IPv6、ICMP
        持续阻断。
      </p>
      <div role="tablist" aria-label="代理管理" className="proxy-tabs">
        {[
          ["nodes", "节点"],
          ["connections", "连接"],
          ["diagnostics", "诊断"],
        ].map(([id, label]) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            disabled={Boolean(form) && tab !== id}
            onClick={() => setTab(id)}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "nodes" && (
        <>
          <div className="proxy-toolbar">
            <label className="proxy-search">
              <Search size={15} />
              <input
                aria-label="搜索节点"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="搜索节点、服务器或协议"
              />
            </label>
            <button
              type="button"
              disabled={busy || Boolean(form)}
              onClick={() => openEditor(null)}
            >
              <Plus size={14} />
              添加节点
            </button>
            <button
              type="button"
              disabled={busy || !proxy.running || !nodes.length}
              title="检测 HTTP 往返延迟，通常不含建连开销，不代表网页首开耗时"
              onClick={() => measure(nodes.map((node) => node.id))}
            >
              <Activity size={14} />
              测速
            </button>
          </div>
          {form && (
            <form
              className="proxy-node-editor"
              aria-label={form.id ? "编辑代理节点" : "添加代理节点"}
              onSubmit={submit}
            >
              <h4>{form.id ? "编辑节点" : "添加节点"}</h4>
              {form.id && form.id === proxy.selected && (
                <p className="field-help">
                  保存当前节点的修改会重建网络连接并重新检查出口。
                </p>
              )}
              <div className="field-row">
                <label>
                  节点名称
                  <input
                    autoFocus
                    value={form.name}
                    maxLength={100}
                    disabled={busy}
                    onChange={(event) =>
                      setForm({ ...form, name: event.target.value })
                    }
                  />
                </label>
                <label>
                  协议
                  <InlineSelect
                    inline
                    aria-label="协议"
                    value={form.type}
                    disabled={busy}
                    onValueChange={(type) =>
                      changeAuthentication({
                        type,
                        host_key: "",
                        port:
                          form.port === (ssh ? "22" : "1080")
                            ? type === "ssh"
                              ? "22"
                              : "1080"
                            : form.port,
                      })
                    }
                  >
                    {Object.entries(protocolNames).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </InlineSelect>
                </label>
              </div>
              <div className="field-row">
                <label>
                  服务器
                  <input
                    value={form.server}
                    maxLength={253}
                    disabled={busy}
                    onChange={(event) =>
                      setForm({ ...form, server: event.target.value })
                    }
                    placeholder="域名或 IP"
                  />
                </label>
                <label>
                  端口
                  <input
                    type="number"
                    min="1"
                    max="65535"
                    value={form.port}
                    disabled={busy}
                    onChange={(event) =>
                      setForm({ ...form, port: event.target.value })
                    }
                  />
                </label>
              </div>
              {ssh && (
                <label>
                  认证方式
                  <InlineSelect
                    inline
                    aria-label="认证方式"
                    value={form.auth_method}
                    disabled={busy}
                    onValueChange={(auth_method) =>
                      changeAuthentication({ auth_method })
                    }
                  >
                    <option value="password">密码</option>
                    <option value="private-key">私钥</option>
                  </InlineSelect>
                </label>
              )}
              <div className="field-row">
                <label>
                  {ssh ? "用户名（必填）" : "用户名"}
                  <input
                    value={form.username}
                    autoComplete="off"
                    maxLength={255}
                    disabled={busy}
                    onChange={(event) =>
                      setForm({ ...form, username: event.target.value })
                    }
                  />
                </label>
                {!privateKey && (
                  <label>
                    {ssh ? "密码（必填）" : "密码"}
                    <input
                      type="password"
                      value={form.password}
                      autoComplete="new-password"
                      disabled={busy}
                      placeholder={
                        form.has_password ? "留空保留已保存密码" : ""
                      }
                      onChange={(event) =>
                        setForm({ ...form, password: event.target.value })
                      }
                    />
                  </label>
                )}
              </div>
              {privateKey && (
                <>
                  <label>
                    私钥内容（必填）
                    <textarea
                      aria-label="私钥内容（必填）"
                      value={form.private_key}
                      rows={7}
                      spellCheck={false}
                      autoComplete="off"
                      autoCapitalize="none"
                      disabled={busy}
                      placeholder={
                        form.has_private_key
                          ? "留空保留已保存私钥"
                          : "-----BEGIN OPENSSH PRIVATE KEY-----"
                      }
                      onChange={(event) =>
                        setForm({ ...form, private_key: event.target.value })
                      }
                    />
                  </label>
                  <p className="field-help">
                    粘贴完整 PEM / OpenSSH 私钥内容。
                  </p>
                  <label>
                    私钥口令（可选）
                    <input
                      type="password"
                      value={form.private_key_passphrase}
                      autoComplete="new-password"
                      disabled={busy}
                      placeholder={
                        form.has_private_key_passphrase && !form.private_key
                          ? "留空保留已保存口令"
                          : ""
                      }
                      onChange={(event) =>
                        setForm({
                          ...form,
                          private_key_passphrase: event.target.value,
                        })
                      }
                    />
                  </label>
                </>
              )}
              {ssh && (
                <>
                  <label>
                    服务器公钥（可选，建议填写）
                    <textarea
                      aria-label="服务器公钥（可选，建议填写）"
                      value={form.host_key}
                      rows={2}
                      spellCheck={false}
                      autoCapitalize="none"
                      disabled={busy}
                      onChange={(event) =>
                        setForm({ ...form, host_key: event.target.value })
                      }
                      placeholder="ssh-ed25519 AAAA…"
                    />
                  </label>
                  <p className="field-help">
                    每行一条可信服务器公钥；留空时不校验服务器身份。SSH 仅转发
                    TCP。
                  </p>
                </>
              )}
              <div className="proxy-form-actions">
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    setForm(null);
                    setError("");
                  }}
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="primary"
                  disabled={busy || !formValid}
                >
                  {pending === "add" || pending === "edit" ? (
                    <LoaderCircle size={14} className="spin" />
                  ) : (
                    <Check size={14} />
                  )}
                  保存并验证
                </button>
              </div>
            </form>
          )}
          {!allNodes.length ? (
            <div className="proxy-empty">
              <Server size={26} />
              <strong>添加第一个代理节点</strong>
              <p>填写 HTTP CONNECT、SOCKS5 或 SSH 的连接信息。</p>
            </div>
          ) : (
            <>
              <div className="proxy-node-list">
                {nodes.map((node) => (
                  <div
                    className={`proxy-node ${node.id === selectedID ? "selected" : ""}`}
                    key={node.id}
                  >
                    <button
                      type="button"
                      className="proxy-node-choice"
                      disabled={busy || Boolean(form) || !proxy.core_available}
                      title="选择后点击应用代理生效"
                      onClick={() => setDraft(node.id)}
                      aria-pressed={node.id === selectedID}
                      aria-label={`选择节点 ${node.name}`}
                    >
                      <span className="proxy-radio">
                        {node.id === selectedID && <Check size={12} />}
                      </span>
                      <span>
                        <strong>{node.name}</strong>
                        <small>
                          {protocolNames[node.type]} · {node.server}:{node.port}
                        </small>
                      </span>
                    </button>
                    <div className="proxy-node-actions">
                      {node.id === proxy.selected && (
                        <span className="proxy-node-role">当前节点</span>
                      )}
                      <span className="proxy-delay">
                        {delays[node.id] || "未测速"}
                      </span>
                      <button
                        type="button"
                        aria-label={`测速 ${node.name}`}
                        title="检测 HTTP 往返延迟"
                        disabled={busy || !proxy.running}
                        onClick={() => measure([node.id])}
                      >
                        <Activity size={14} />
                      </button>
                      <button
                        type="button"
                        aria-label={`编辑 ${node.name}`}
                        title={`编辑 ${node.name}`}
                        disabled={busy || Boolean(form)}
                        onClick={() => openEditor(node)}
                      >
                        <Pencil size={14} />
                      </button>
                      <button
                        type="button"
                        aria-label={`删除 ${node.name}`}
                        title={`删除 ${node.name}`}
                        disabled={busy || Boolean(form)}
                        onClick={async () => {
                          if (
                            await onConfirm({
                              title: `删除节点“${node.name}”？`,
                              body:
                                node.id === proxy.selected
                                  ? "删除后关闭客机外网，选择并应用其他节点后才能恢复联网。"
                                  : "将删除这个节点及保存的认证信息。",
                              confirm: "删除节点",
                            })
                          )
                            await mutate("delete", { node_id: node.id });
                        }}
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </div>
                ))}
                {!nodes.length && (
                  <p className="proxy-empty">没有符合搜索条件的节点。</p>
                )}
              </div>
              <div className="proxy-selection">
                <span aria-live="polite">
                  {selectionChanged
                    ? `待应用：${proposed?.name || "节点已移除"}`
                    : proxy.active
                      ? "当前配置已应用"
                      : "选择一个节点后应用代理"}
                </span>
                <div>
                  {draft !== null && (
                    <button
                      type="button"
                      disabled={busy || Boolean(form)}
                      onClick={() => setDraft(null)}
                    >
                      撤销选择
                    </button>
                  )}
                  <button
                    type="button"
                    className="primary"
                    disabled={
                      busy ||
                      Boolean(form) ||
                      !proxy.core_available ||
                      !proposed ||
                      !validPort
                    }
                    onClick={applyNode}
                  >
                    {pending === "select" ? (
                      <LoaderCircle size={14} className="spin" />
                    ) : (
                      <Check size={14} />
                    )}
                    {selectionChanged || !proxy.active
                      ? "应用代理"
                      : "重新应用"}
                  </button>
                </div>
              </div>
            </>
          )}
        </>
      )}
      {tab === "connections" && (
        <>
          <div className="proxy-metrics">
            <span>
              上传<strong>{bytes(rates.up)}/s</strong>
              <small>累计 {bytes(connections.uploadTotal)}</small>
            </span>
            <span>
              下载<strong>{bytes(rates.down)}/s</strong>
              <small>累计 {bytes(connections.downloadTotal)}</small>
            </span>
          </div>
          <p className="field-help">
            显示当前核心的连接与流量，包含 Claude Eden
            自身检测请求；重启核心后累计值重置。
          </p>
          <div className="proxy-node-list">
            {connections.connections.map((connection) => (
              <div className="proxy-connection" key={connection.id}>
                <span>
                  {connection.host}:{connection.port}
                  <small>{connection.network}</small>
                </span>
                <small>
                  ↑ {bytes(connection.upload)} · ↓ {bytes(connection.download)}
                </small>
              </div>
            ))}
            {!connections.connections.length && (
              <p className="proxy-empty">暂无活动连接</p>
            )}
          </div>
        </>
      )}
      {tab === "diagnostics" && (
        <>
          <p className="field-help">
            随附核心 {proxy.version || "未运行"}。以下为 Claude Eden
            状态摘要；含凭据风险的核心原始日志不向页面公开。
          </p>
          <div className="proxy-diagnostics" role="log">
            {logs.map((line, index) => (
              <div key={index}>{line}</div>
            ))}
          </div>
          <label>
            独立代理端口
            <input
              type="number"
              min="1024"
              max="65535"
              value={port}
              onChange={(event) => {
                portDirty.current = true;
                setPort(event.target.value);
              }}
            />
          </label>
          <div className="proxy-toolbar">
            <button
              type="button"
              disabled={busy || !selected || !validPort}
              onClick={async () => {
                if (
                  Number(port) !== proxy.port &&
                  status.vm_running &&
                  !(await onConfirm({
                    title: "更换代理端口并重启环境？",
                    body: "端口变化需要重启环境，当前终端会结束。",
                    confirm: "更换并重启",
                  }))
                )
                  return;
                await mutate("restart", { port: Number(port) });
              }}
            >
              <RefreshCw size={14} />
              应用端口并重启核心
            </button>
            <button
              type="button"
              disabled={busy || !proxy.running}
              onClick={() => mutate("stop")}
            >
              <Square size={14} />
              停止核心
            </button>
          </div>
          <p className="field-help">
            Claude Eden 使用独立代理端口。主机其他代理的 TUN
            可能影响核心出站，需要结合实际路由验证。
          </p>
        </>
      )}
      {pending && (
        <p className="proxy-progress" role="status">
          <LoaderCircle size={14} className="spin" />
          {pending === "delay"
            ? "正在检测节点延迟…"
            : status.job?.message || "正在处理…"}
        </p>
      )}
      {error && (
        <div role="alert" className="error-message">
          <SafetyMessage message={error} />
        </div>
      )}
      {notice && (
        <p role="status" className="proxy-notice">
          <Check size={14} />
          {notice}
        </p>
      )}
    </div>
  );
}
