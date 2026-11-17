import React, {
  createContext,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import {
  Camera,
  Check,
  ChevronDown,
  Eye,
  EyeOff,
  LockKeyhole,
  LogOut,
  Settings,
} from "lucide-react";
import { api, native, nativeAvailable, writeClipboard } from "../lib/api.ts";
import { getClientContext } from "../lib/client-context.ts";
import logo from "../logo.png";
import "../styles/account.css";
import {
  WorkbenchAccountViewSchema,
  type WorkbenchAccountView,
} from "../lib/contracts.generated.ts";
import { errorMessage } from "../lib/errors.ts";
import type { Confirm, DirtyChange, Notify } from "../lib/types.ts";
const accountResponseSchema = WorkbenchAccountViewSchema.extend({
  recovery_key: WorkbenchAccountViewSchema.shape.username.optional(),
});
type Recovery = {
  key: string;
  binding: string;
  username: string;
  reset: boolean;
};
interface AccountContextValue {
  account: WorkbenchAccountView | null;
  error: string;
  deviceNotice: string;
  setDeviceNotice: (message: string) => void;
  recovery: Recovery | null;
  dismissRecovery: () => void;
  refresh: () => Promise<WorkbenchAccountView>;
  act: (
    action: string,
    input?: Record<string, unknown>,
  ) => Promise<WorkbenchAccountView & { recovery_key?: string }>;
}

const AccountContext = createContext<AccountContextValue | null>(null);
export const useAccount = () => {
  const value = useContext(AccountContext);
  if (!value) throw new Error("账户功能需要 AccountProvider。");
  return value;
};
const useUnlockedAccount = () => {
  const value = useAccount();
  if (!value.account) throw new Error("账户资料尚未读取完成。");
  return { ...value, account: value.account };
};
const localDesktop = () => getClientContext().desktop_gateway;
const trustedLogin = (action: string, password?: string) =>
  api("/client/trusted-login", {
    action,
    password,
    connection_id: getClientContext().connection_id,
  });

export function AccountProvider({ children }: React.PropsWithChildren) {
  const [account, setAccount] = useState<WorkbenchAccountView | null>(null);
  const [error, setError] = useState("");
  const [deviceNotice, setDeviceNotice] = useState("");
  const [recovery, setRecovery] = useState<Recovery | null>(null);
  const sequence = useRef(0);
  const refresh = async () => {
    const current = ++sequence.current;
    try {
      const value = await api("/account");
      if (sequence.current === current) {
        setAccount(value);
        setError("");
      }
      return value;
    } catch (e) {
      if (sequence.current === current) setError(errorMessage(e));
      throw e;
    }
  };
  const act = async (action: string, input: Record<string, unknown> = {}) => {
    const current = ++sequence.current;
    const value = await api(
      `/account/${action}`,
      input,
      undefined,
      {},
      { schema: accountResponseSchema },
    );
    const { recovery_key: recoveryKey, ...view } = value;
    if (recoveryKey) {
      // A status read started during this write must not replace its result or
      // discard the only response containing the newly generated secret.
      ++sequence.current;
      setRecovery({
        key: recoveryKey,
        binding: view.device_binding,
        username: view.username,
        reset: action === "recovery-reset",
      });
      setAccount(view);
    } else if (action !== "logout" && sequence.current === current)
      setAccount(view);
    return value;
  };
  useEffect(() => {
    if (
      recovery &&
      account &&
      (recovery.binding !== account.device_binding ||
        (!recovery.reset && !account.unlocked))
    )
      setRecovery(null);
  }, [account?.device_binding, account?.unlocked, recovery]);
  useEffect(() => {
    let live = true,
      timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      try {
        await refresh();
      } catch {
        /* Keep the last state while disconnected. */
      }
      if (live) timer = setTimeout(poll, document.hidden ? 10000 : 2500);
    };
    const start = async () => {
      try {
        if (location.hash) {
          await api("/session", { token: location.hash.slice(1) });
          history.replaceState(null, "", location.pathname);
        }
        window.dispatchEvent(new Event("eden-session-ready"));
        if (live) await poll();
      } catch (e) {
        if (live) setError(errorMessage(e));
      }
    };
    const locked = () => {
      // A 423 can follow a controller restart or directory change.
      setAccount((previous) =>
        previous ? { ...previous, unlocked: false } : previous,
      );
      refresh().catch(() => {});
    };
    start();
    window.addEventListener("eden-account-locked", locked);
    return () => {
      live = false;
      sequence.current++;
      clearTimeout(timer);
      window.removeEventListener("eden-account-locked", locked);
    };
  }, []);
  return (
    <AccountContext.Provider
      value={{
        account,
        error,
        refresh,
        act,
        deviceNotice,
        setDeviceNotice,
        recovery:
          recovery?.binding === account?.device_binding ? recovery : null,
        dismissRecovery: () => setRecovery(null),
      }}
    >
      {children}
    </AccountContext.Provider>
  );
}

export function AccountAvatar({
  account,
  large = false,
}: {
  account: { username?: string; avatar?: string } | null;
  large?: boolean;
}) {
  const initial = Array.from(
    account?.username?.trim() || "E",
  )[0].toLocaleUpperCase();
  return (
    <span
      className={`account-avatar ${large ? "large" : ""}`}
      aria-hidden="true"
    >
      {account?.avatar ? <img src={account.avatar} alt="" /> : initial}
    </span>
  );
}

function PasswordField({
  label,
  value,
  onChange,
  autoComplete = "current-password",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  autoComplete?: string;
}) {
  const [visible, setVisible] = useState(false);
  const id = useId();
  return (
    <div className="account-field">
      <label htmlFor={id}>{label}</label>
      <span className="password-input">
        <input
          id={id}
          type={visible ? "text" : "password"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          autoComplete={autoComplete}
          required
          maxLength={1024}
        />
        <button
          type="button"
          className="icon-button"
          aria-label={`${visible ? "隐藏" : "显示"}${label}`}
          onClick={() => setVisible(!visible)}
        >
          {visible ? <EyeOff size={16} /> : <Eye size={16} />}
        </button>
      </span>
    </div>
  );
}

export function AccountGate({ children }: React.PropsWithChildren) {
  const {
    account,
    error: connectionError,
    refresh,
    act,
    setDeviceNotice,
    recovery,
  } = useAccount();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [rememberLogin, setRememberLogin] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [recovering, setRecovering] = useState(false);
  const gateAcknowledged = useRef(false);
  const trustedAttempt = useRef(false);
  const enterTrusted = async (action: string) => {
    setBusy(true);
    setError("");
    try {
      await trustedLogin(action);
      await refresh();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    if (account?.unlocked) {
      trustedAttempt.current = false;
      setRememberLogin(null);
      return;
    }
    if (
      !account ||
      !localDesktop() ||
      trustedAttempt.current ||
      account.require_authentication
    )
      return;
    if (!(account.trusted_local || account.trusted_login)) return;
    trustedAttempt.current = true;
    void enterTrusted("resume");
  }, [
    account?.unlocked,
    account?.exists,
    account?.guest_system,
    account?.trusted_local,
    account?.trusted_login,
    account?.require_authentication,
  ]);
  useEffect(() => {
    if (
      !account ||
      account.unlocked ||
      connectionError ||
      gateAcknowledged.current ||
      !nativeAvailable()
    )
      return;
    // A working lock/setup screen is a valid upgraded UI. Waiting for the
    // user's password would incorrectly leave the native update unconfirmed.
    native("workbenchReady")
      .then((accepted) => {
        if (accepted === true) gateAcknowledged.current = true;
      })
      .catch(() => {});
  }, [Boolean(account), account?.unlocked, connectionError]);
  if (recovery) return <RecoveryKeyScreen />;
  if (account?.unlocked) return children;
  if (recovering)
    return (
      <AccountRecovery
        onBack={() => setRecovering(false)}
        onReset={() => {
          setRecovering(false);
          setPassword("");
          setConfirmation("");
          setRememberLogin(false);
          setError("");
        }}
      />
    );
  const creating = account && !account.exists;
  const rememberThisDevice =
    rememberLogin ?? Boolean(creating || account?.trusted_login);
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (busy) return;
    if (creating && password !== confirmation) {
      setError("两次密码不一致");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await act(
        creating ? "create" : "unlock",
        creating
          ? { username, password, confirm_password: confirmation }
          : { password },
      );
      if (localDesktop()) {
        try {
          await trustedLogin(
            rememberThisDevice ? "enable" : "disable",
            password,
          );
          await refresh();
        } catch (e) {
          setDeviceNotice(
            `已登录，${rememberThisDevice ? "记住我未能开启" : "记住我未能关闭"}：${errorMessage(e)}。可在「设置 → 账户」中重试。`,
          );
        }
      }
      setPassword("");
      setConfirmation("");
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <main className="account-gate">
      <div className="account-brand">
        <img src={logo} alt="" />
        Claude Eden
      </div>
      <section
        className="account-entry"
        aria-label={creating ? "创建本地账户" : "解锁 Claude Eden"}
      >
        {creating ? (
          <img className="account-welcome-logo" src={logo} alt="" />
        ) : (
          account && <AccountAvatar account={account} large />
        )}
        <h1>
          {creating
            ? "注册 Claude Eden 账户"
            : account
              ? `欢迎回来，${account.username}`
              : "正在连接 Claude Eden"}
        </h1>
        <p className="account-entry-intro">
          {creating
            ? "先注册用户名和密码，再安装本地 Debian 13 环境。"
            : account
              ? account.trusted_local
                ? "本机工作区"
                : "输入密码，继续你的工作。"
              : "正在读取本机账户…"}
        </p>
        {account?.background_running && (
          <div className="account-background">
            <i />
            后台环境仍在运行，任务没有停止
          </div>
        )}
        {account?.trusted_local && (
          <div className="account-trust-choice">
            <p>
              {localDesktop()
                ? "使用当前系统用户的本机信任凭据进入工作区。"
                : "请回到受信任的桌面设置账户密码，再从此处登录。"}
            </p>
            {localDesktop() && (
              <button
                className="quiet-button"
                disabled={busy}
                onClick={() => enterTrusted("resume")}
              >
                {busy ? "正在进入…" : "重新进入本机工作区"}
              </button>
            )}
          </div>
        )}
        {account?.trusted_local && error && (
          <p className="account-error" role="alert">
            {error}
          </p>
        )}
        {account && !account.trusted_local && (
          <form onSubmit={submit}>
            <fieldset disabled={busy}>
              {creating && (
                <>
                  <label className="account-field">
                    用户名
                    <input
                      autoFocus
                      autoComplete="username"
                      maxLength={40}
                      required
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      placeholder="你希望怎样被称呼"
                    />
                  </label>
                </>
              )}
              <PasswordField
                label={creating ? "设置密码" : "账户密码"}
                value={password}
                onChange={setPassword}
                autoComplete={creating ? "new-password" : "current-password"}
              />
              {creating && (
                <PasswordField
                  label="确认密码"
                  value={confirmation}
                  onChange={setConfirmation}
                  autoComplete="new-password"
                />
              )}
              {(error || connectionError) && (
                <p className="account-error" role="alert">
                  {error || connectionError}
                </p>
              )}
              {localDesktop() && (
                <label className="account-device-option check-line">
                  <input
                    type="checkbox"
                    checked={rememberThisDevice}
                    onChange={(e) => setRememberLogin(e.target.checked)}
                  />
                  <span>
                    <strong>记住我</strong>
                    <small>
                      信任此电脑，下次用当前系统用户打开时直接进入。手动锁定仍需验证。
                    </small>
                  </span>
                </label>
              )}
              <button
                className="primary account-submit"
                disabled={
                  busy || !password || Boolean(creating && !username.trim())
                }
              >
                {busy
                  ? "正在解锁…"
                  : creating
                    ? "创建本地账户"
                    : "解锁 Claude Eden"}
              </button>
              {!creating && (
                <button
                  type="button"
                  className="text-button"
                  onClick={() => {
                    setPassword("");
                    setError("");
                    setRecovering(true);
                  }}
                >
                  忘记密码？
                </button>
              )}
            </fieldset>
          </form>
        )}
        {!account && connectionError && (
          <>
            <p role="alert" className="account-error">
              {connectionError}
            </p>
            <button onClick={() => refresh().catch(() => {})}>重新连接</button>
          </>
        )}
        <p className="account-local-note">
          <LockKeyhole size={14} />
          账户资料仅保存在这台设备上
        </p>
        {!account?.trusted_local && (
          <p className="account-recovery-note">
            {creating
              ? "注册后会生成恢复密钥。请保存到密码管理器，忘记密码时可用它恢复。"
              : account?.recovery_ready
                ? "忘记密码时，可使用已保存的恢复密钥重置。"
                : "此账户尚未设置恢复密钥。能登录时，请到「设置 → 账户」补设。"}
          </p>
        )}
      </section>
      {nativeAvailable() && (
        <button
          className="text-button account-exit-app"
          onClick={() =>
            native("quitNow").catch((e) => setError(errorMessage(e)))
          }
        >
          {account?.background_running
            ? "退出应用，后台继续运行"
            : "退出 Claude Eden"}
        </button>
      )}
    </main>
  );
}

function RecoveryKeyScreen() {
  const { recovery, dismissRecovery } = useAccount();
  const [saved, setSaved] = useState(false);
  const [notice, setNotice] = useState("");
  if (!recovery) return null;
  return (
    <main className="account-gate">
      <section
        className="account-entry account-recovery"
        aria-label="保存恢复密钥"
      >
        <h1>{recovery.reset ? "密码已重置" : "保存你的恢复密钥"}</h1>
        <p className="account-entry-intro">
          {recovery.reset && "请保存新的恢复密钥。"}账户：{recovery.username}
          。这份密钥可以重置密码并访问原有资料，请存入密码管理器或安全的离线位置，不要分享给他人。
        </p>
        <label className="account-field">
          恢复密钥
          <textarea
            aria-label="恢复密钥"
            readOnly
            value={recovery.key}
            rows={4}
            spellCheck={false}
            autoComplete="off"
            onFocus={(event) => event.target.select()}
          />
        </label>
        <div className="account-recovery-actions">
          <button
            className="quiet-button"
            onClick={async () => {
              try {
                await writeClipboard(recovery.key);
                setNotice("恢复密钥已复制，请粘贴到密码管理器。");
              } catch {
                setNotice("未能复制，请选中上方完整密钥，手动复制并保存。");
              }
            }}
          >
            复制恢复密钥
          </button>
          <button
            className="quiet-button"
            onClick={async () => {
              try {
                if (nativeAvailable()) {
                  if (await native("saveRecoveryKey", { key: recovery.key }))
                    setNotice("恢复密钥文件已保存，请妥善保管。");
                  return;
                }
                const url = URL.createObjectURL(
                  new Blob([recovery.key + "\n"], {
                    type: "text/plain;charset=utf-8",
                  }),
                );
                const link = document.createElement("a");
                link.href = url;
                link.download = "账户恢复密钥.txt";
                link.click();
                setTimeout(() => URL.revokeObjectURL(url), 1000);
                setNotice("请确认恢复密钥文件已保存到安全位置。");
              } catch {
                setNotice("未能保存文件，请重试或复制完整密钥到密码管理器。");
              }
            }}
          >
            下载密钥文件
          </button>
        </div>
        {notice && (
          <p className="field-help" role="status">
            {notice}
          </p>
        )}
        <p className="field-help">
          完整密钥仅在此处显示。关闭或刷新后，可验证账户密码重新生成；重新生成后旧密钥失效。
          {recovery.reset &&
            "原来的资料和 Claude 登录状态已保留，Claude Eden 的记住我已关闭，请用新密码登录后重新设置。"}
        </p>
        <label className="account-device-option check-line">
          <input
            type="checkbox"
            checked={saved}
            onChange={(event) => setSaved(event.target.checked)}
          />
          <span>我已安全保存完整恢复密钥</span>
        </label>
        <button
          className="primary account-submit"
          disabled={!saved}
          onClick={dismissRecovery}
        >
          {recovery.reset ? "返回登录" : "继续使用 Claude Eden"}
        </button>
      </section>
    </main>
  );
}

function AccountRecovery({
  onBack,
  onReset,
}: {
  onBack: () => void;
  onReset: () => void;
}) {
  const { account, act } = useUnlockedAccount();
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <main className="account-gate">
      <section className="account-entry account-recovery" aria-label="恢复账户">
        <h1>使用恢复密钥重置密码</h1>
        <p className="account-entry-intro">
          为 {account.username} 设置新密码，原来的资料和 Claude 登录状态会保留。
        </p>
        {account.recovery_ready ? (
          <form
            onSubmit={async (event) => {
              event.preventDefault();
              if (busy) return;
              if (password !== confirmation) {
                setError("两次密码不一致");
                return;
              }
              setBusy(true);
              setError("");
              try {
                await act("recovery-reset", {
                  recovery_key: code,
                  password,
                  confirm_password: confirmation,
                });
                onReset();
              } catch (e) {
                setError(errorMessage(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            <fieldset disabled={busy}>
              <label className="account-field">
                已保存的恢复密钥
                <textarea
                  aria-label="已保存的恢复密钥"
                  autoFocus
                  required
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  rows={4}
                  maxLength={256}
                  autoComplete="off"
                  spellCheck={false}
                />
              </label>
              <PasswordField
                label="新密码"
                value={password}
                onChange={setPassword}
                autoComplete="new-password"
              />
              <PasswordField
                label="确认新密码"
                value={confirmation}
                onChange={setConfirmation}
                autoComplete="new-password"
              />
              <p className="field-help">
                重置后会换发恢复密钥，旧密钥失效。Claude Eden 的记住我会关闭。
              </p>
              {error && (
                <p className="account-error" role="alert">
                  {error}
                </p>
              )}
              <button
                className="primary account-submit"
                disabled={!code.trim() || !password || !confirmation}
              >
                {busy ? "正在重置…" : "重置密码"}
              </button>
            </fieldset>
          </form>
        ) : (
          <p className="field-help" role="status">
            此账户尚未设置恢复密钥。请使用原密码或已开启的快捷登录进入，再到「设置
            →
            账户」补设。若所有解锁方式均不可用，当前无法恢复加密资料；现有数据会保留。
          </p>
        )}
        <button
          className="text-button account-submit"
          disabled={busy}
          onClick={onBack}
        >
          返回密码登录
        </button>
      </section>
    </main>
  );
}

function AccountRecoverySettings({
  disabled,
  onDirty,
}: {
  disabled?: boolean;
  onDirty: DirtyChange;
}) {
  const { account, act } = useUnlockedAccount();
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    onDirty(Boolean(password));
  }, [password]);
  useEffect(() => () => onDirty(false), []);
  return (
    <section className="account-device-settings" aria-label="账户恢复">
      <form
        onSubmit={async (event) => {
          event.preventDefault();
          if (busy) return;
          setBusy(true);
          setError("");
          try {
            await act("recovery-generate", { password });
            setPassword("");
          } catch (e) {
            setError(errorMessage(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <fieldset disabled={disabled || busy}>
          <h4>恢复密钥</h4>
          <p className="field-help">
            {account.recovery_ready
              ? "已设置。忘记密码时，可在登录页使用保存的恢复密钥。重新生成后旧密钥立即失效。"
              : "尚未设置。请补设并安全保存，忘记密码时可用它恢复原来的资料和登录状态。"}
          </p>
          <PasswordField
            label="生成密钥前验证账户密码"
            value={password}
            onChange={setPassword}
            autoComplete="current-password"
          />
          {error && (
            <p className="account-error" role="alert">
              {error}
            </p>
          )}
          <button className="quiet-button" disabled={!password}>
            {busy
              ? "正在生成…"
              : account.recovery_ready
                ? "重新生成恢复密钥"
                : "生成恢复密钥"}
          </button>
        </fieldset>
      </form>
    </section>
  );
}

export function AccountMenu({
  onSettings,
  onConfirm,
  onStatus,
  busy,
}: {
  onSettings: (section: string) => void;
  onConfirm: Confirm;
  onStatus: Notify;
  busy?: boolean;
}) {
  const { account, act, deviceNotice, setDeviceNotice } = useUnlockedAccount();
  useEffect(() => {
    if (deviceNotice) {
      onStatus(deviceNotice);
      setDeviceNotice("");
    }
  }, [deviceNotice]);
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    root.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    const dismiss = (event: MouseEvent | KeyboardEvent) => {
      if (
        ("key" in event && event.key === "Escape") ||
        (event.type === "click" &&
          event.target instanceof Node &&
          !root.current?.contains(event.target))
      ) {
        if ("key" in event && event.key === "Escape")
          root.current?.querySelector<HTMLElement>(".account-trigger")?.focus();
        setOpen(false);
      }
    };
    window.addEventListener("click", dismiss);
    window.addEventListener("keydown", dismiss);
    return () => {
      window.removeEventListener("click", dismiss);
      window.removeEventListener("keydown", dismiss);
    };
  }, [open]);
  const perform = async (action: string) => {
    setOpen(false);
    if (
      action === "logout" &&
      !(await onConfirm({
        title: "退出本地账户？",
        body: "这会停止 Claude Eden 环境与后台任务，并锁定登录凭据。请先保存文件。",
        confirm: "停止环境并退出账户",
      }))
    )
      return;
    try {
      await act(action);
    } catch (e) {
      onStatus(errorMessage(e));
    }
  };
  return (
    <div className="account-menu-root" ref={root}>
      <button
        className="account-trigger"
        aria-label={`账户菜单，${account.username}`}
        aria-expanded={open}
        aria-haspopup="menu"
        onClick={() => setOpen(!open)}
      >
        <AccountAvatar account={account} />
        <span className="account-trigger-name">
          <strong>{account.username}</strong>
          <small>{account.trusted_local ? "受信任的本机" : "本地账户"}</small>
        </span>
        <ChevronDown size={15} />
      </button>
      {open && (
        <div
          className="account-menu"
          role="menu"
          aria-label="本地账户菜单"
          onKeyDown={(event) => {
            if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
            event.preventDefault();
            const items = [
              ...event.currentTarget.querySelectorAll<HTMLButtonElement>(
                "button:enabled",
              ),
            ];
            const step = event.key === "ArrowDown" ? 1 : -1;
            items[
              (items.indexOf(event.target as HTMLButtonElement) +
                step +
                items.length) %
                items.length
            ].focus();
          }}
        >
          <div className="account-menu-label">你的 Claude Eden</div>
          <button
            role="menuitem"
            onClick={() => {
              setOpen(false);
              onSettings("account");
            }}
          >
            <Settings size={16} />
            设置
          </button>
          {account.trusted_local ? (
            <button
              role="menuitem"
              onClick={() => {
                setOpen(false);
                onSettings("account");
              }}
            >
              <LockKeyhole size={16} /> 设置账户密码
            </button>
          ) : (
            <>
              <div className="account-menu-divider" />
              <button role="menuitem" onClick={() => perform("lock")}>
                <LockKeyhole size={16} />
                锁定 Claude Eden<small>任务继续运行</small>
              </button>
              <button
                role="menuitem"
                disabled={busy}
                onClick={() => perform("logout")}
              >
                <LogOut size={16} />
                退出账户
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

export function AccountSettings({
  busy: environmentBusy,
  onDirty,
}: {
  busy?: boolean;
  onDirty?: DirtyChange;
}) {
  const { account, act, refresh } = useUnlockedAccount();
  const [name, setName] = useState(account.username),
    [avatar, setAvatar] = useState(account.avatar || "");
  const [oldPassword, setOldPassword] = useState(""),
    [password, setPassword] = useState(""),
    [confirmation, setConfirmation] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [message, setMessage] = useState("");
  const [feedbackTarget, setFeedbackTarget] = useState("profile");
  const [recoveryDirty, setRecoveryDirty] = useState(false);
  const file = useRef<HTMLInputElement | null>(null);
  const changed =
    name !== account.username || avatar !== (account.avatar || "");
  useEffect(() => {
    onDirty?.(
      changed ||
        Boolean(oldPassword || password || confirmation) ||
        recoveryDirty,
    );
  }, [changed, oldPassword, password, confirmation, recoveryDirty]);
  useEffect(() => () => onDirty?.(false), []);
  const upload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    setFeedbackTarget("profile");
    const selected = event.target.files?.[0];
    event.target.value = "";
    if (!selected) return;
    if (
      selected.size > 2 * 1024 * 1024 ||
      !["image/png", "image/jpeg"].includes(selected.type)
    ) {
      setError("请选择不超过 2 MB 的 PNG 或 JPEG 图片");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") setAvatar(reader.result);
      setError("");
      setMessage("");
    };
    reader.readAsDataURL(selected);
  };
  const save = async (
    event: React.FormEvent<HTMLFormElement>,
    changingPassword = false,
  ) => {
    event.preventDefault();
    if (busy) return;
    setFeedbackTarget(changingPassword ? "password" : "profile");
    if (changingPassword && password !== confirmation) {
      setError("两次密码不一致");
      return;
    }
    setBusy(true);
    setError("");
    setMessage("");
    try {
      if (changingPassword && account.trusted_local) {
        await trustedLogin("protect", password);
        await refresh();
        setPassword("");
        setConfirmation("");
        setMessage("已设置密码，下次打开 Claude Eden 时使用密码登录。");
        return;
      }
      const value = await act(
        changingPassword ? "password" : "profile",
        changingPassword
          ? {
              old_password: oldPassword,
              password,
              confirm_password: confirmation,
            }
          : { username: name, avatar },
      );
      if (changingPassword) {
        setOldPassword("");
        setPassword("");
        setConfirmation("");
        setMessage("密码已修改，应用登录状态保持不变");
      } else {
        setName(value.username);
        setAvatar(value.avatar || "");
        setMessage("账户资料已保存");
      }
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  const feedback = (
    <>
      {error && (
        <p className="account-error" role="alert">
          {error}
        </p>
      )}
      {message && (
        <p className="account-saved" role="status">
          <Check size={15} />
          {message}
        </p>
      )}
    </>
  );
  return (
    <div className="account-settings">
      <div className="section-title">
        <h3>账户</h3>
        <p>给工作空间一点你的样子。</p>
      </div>
      <form onSubmit={(event) => save(event)}>
        <fieldset disabled={busy || environmentBusy}>
          <div className="account-profile-heading">
            <AccountAvatar account={{ username: name, avatar }} large />
            <div>
              <button
                type="button"
                className="quiet-button"
                onClick={() => file.current?.click()}
              >
                <Camera size={16} />
                更换头像
              </button>
              {avatar && (
                <button
                  type="button"
                  className="text-button"
                  onClick={() => setAvatar("")}
                >
                  恢复默认
                </button>
              )}
              <p className="field-help">
                PNG 或 JPEG，最多 2 MB，自动居中裁切。
              </p>
            </div>
          </div>
          <input
            ref={file}
            type="file"
            accept="image/png,image/jpeg"
            hidden
            onChange={upload}
            aria-label="选择头像图片"
          />
          <label className="account-field">
            用户名
            <input
              value={name}
              maxLength={40}
              autoComplete="username"
              onChange={(e) => setName(e.target.value)}
              required
            />
          </label>
          <button
            className="primary"
            disabled={!changed || !name.trim() || busy}
          >
            <Check size={16} />
            保存资料
          </button>
        </fieldset>
      </form>
      {feedbackTarget === "profile" && feedback}
      <div className="account-section-divider" />
      <form onSubmit={(event) => save(event, true)}>
        <fieldset disabled={busy || environmentBusy}>
          <h4>账户密码</h4>
          <p className="field-help">
            修改密码不会清除 Claude
            的登录状态。忘记密码时，可在登录页使用已保存的恢复密钥重置。
          </p>
          {!account.trusted_local && (
            <PasswordField
              label="当前密码"
              value={oldPassword}
              onChange={setOldPassword}
            />
          )}
          <PasswordField
            label="新密码"
            value={password}
            onChange={setPassword}
            autoComplete="new-password"
          />
          <PasswordField
            label="确认新密码"
            value={confirmation}
            onChange={setConfirmation}
            autoComplete="new-password"
          />
          <button
            className="quiet-button"
            disabled={
              busy ||
              (!account.trusted_local && !oldPassword) ||
              !password ||
              !confirmation
            }
          >
            {account.trusted_local ? "设置密码并关闭免账号登录" : "修改密码"}
          </button>
        </fieldset>
      </form>

      {feedbackTarget === "password" && feedback}
      {!account.trusted_local && (
        <AccountRecoverySettings
          disabled={busy || environmentBusy}
          onDirty={setRecoveryDirty}
        />
      )}
      {localDesktop() && !account.trusted_local && <TrustedDeviceSettings />}
      <p className="account-settings-note">
        <LockKeyhole size={15} />
        一个本地账户，一个持久工作环境。资料与凭据保存在本机。
      </p>
    </div>
  );
}

function TrustedDeviceSettings() {
  const { account, refresh } = useUnlockedAccount();
  const enabled = Boolean(account.trusted_login);
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const update = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    try {
      await trustedLogin(enabled ? "disable" : "enable", password);
      setPassword("");
      await refresh();
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  };
  return (
    <form onSubmit={update}>
      <fieldset disabled={busy}>
        <h4>记住我</h4>
        <p className="field-help">
          {enabled
            ? "已信任此电脑，下次用当前系统用户打开时自动进入。手动锁定仍需验证，退出账户会撤销信任。"
            : "验证一次账户密码，信任此电脑，下次用当前系统用户打开时自动进入。"}
        </p>
        {!enabled && (
          <PasswordField
            label="验证账户密码"
            value={password}
            onChange={setPassword}
            autoComplete="current-password"
          />
        )}
        <button className="quiet-button" disabled={!enabled && !password}>
          {enabled ? "关闭记住我" : "信任此电脑"}
        </button>
        {message && (
          <p className="account-error" role="alert">
            {message}
          </p>
        )}
      </fieldset>
    </form>
  );
}
