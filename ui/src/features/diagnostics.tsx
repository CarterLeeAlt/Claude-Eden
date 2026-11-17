import React, { useEffect, useState } from "react";
import { ArrowDownToLine, LoaderCircle } from "lucide-react";
import {
  api,
  native,
  nativeAvailable,
  reportStartupFailure,
} from "../lib/api.ts";
import { reportDiagnostic } from "../lib/diagnostics.ts";
import { runtimeURL } from "../lib/client-context.ts";
import { errorMessage } from "../lib/errors.ts";

export function DiagnosticSettings() {
  const [info, setInfo] = useState<{ bytes: number; files: number } | null>(
    null,
  );
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    let live = true;
    api("/diagnostics")
      .then((value) => {
        if (live) setInfo(value);
      })
      .catch((error) => {
        if (live) setError(errorMessage(error));
      });
    return () => {
      live = false;
    };
  }, []);
  const save = async () => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      if (nativeAvailable()) {
        const result = await native("exportDiagnostics");
        if (!result?.canceled)
          setNotice("诊断包已保存。分享前请检查其中内容。");
      } else {
        // Fetch before downloading so authentication or disk failures are shown.
        const response = await fetch(runtimeURL("/diagnostics/export"));
        if (!response.ok)
          throw new Error((await response.text()).trim() || "诊断包导出失败");
        if (
          !response.headers.get("content-type")?.startsWith("application/zip")
        )
          throw new Error("当前后台不支持诊断导出，请重新打开 Claude Eden");
        const url = URL.createObjectURL(await response.blob());
        const link = document.createElement("a");
        link.href = url;
        link.download = "eden-diagnostics.zip";
        link.click();
        setTimeout(() => URL.revokeObjectURL(url), 30_000);
        setNotice("诊断包已下载。分享前请检查其中内容。");
      }
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="settings-group" aria-label="日志与诊断">
      <h3>日志与诊断</h3>
      <p className="footnote">
        自动记录应用故障并脱敏，保留 14 天。每个后台日志上限 40
        MiB，环境服务预算 10 MiB。诊断包会同时收集桌面连接与当前环境的记录，最大
        96 MiB。采集范围限于故障日志与运行状态。
      </p>
      {info && (
        <p className="footnote">
          本机日志 {(info.bytes / 1024 / 1024).toFixed(1)} MiB · {info.files}{" "}
          个文件
        </p>
      )}
      <button className="secondary-button" disabled={busy} onClick={save}>
        {busy ? (
          <LoaderCircle size={14} className="spin" />
        ) : (
          <ArrowDownToLine size={14} />
        )}
        {busy ? "正在导出…" : "导出诊断包"}
      </button>
      <p className="footnote">
        诊断包包含界面与环境的脱敏日志、版本和收集状态，环境离线时仍可导出本机记录。只保存在你选择的位置。
      </p>
      {notice && <p role="status">{notice}</p>}
      {error && (
        <p role="alert" className="error-message">
          {error}
        </p>
      )}
    </section>
  );
}

export class DiagnosticBoundary extends React.Component<React.PropsWithChildren> {
  state = { failed: false, reportError: "" };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  componentDidCatch(error: Error, info: React.ErrorInfo) {
    reportDiagnostic(
      "react",
      new Error(
        `${error.message}\n${error.stack || ""}\n${info.componentStack || ""}`,
      ),
    );
    if (nativeAvailable()) {
      const detail = Array.from(error.message.replace(/\0/g, "�"))
        .slice(0, 500)
        .join("");
      void reportStartupFailure(`工作台显示异常：${detail}`).catch(
        (failure) => {
          this.setState({ reportError: errorMessage(failure) });
        },
      );
    }
  }
  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <main style={{ padding: 32 }}>
        <h2>工作台显示遇到异常</h2>
        <p>后台任务仍可继续运行。可以导出诊断包，再重新加载界面。</p>
        {this.state.reportError && (
          <p role="alert">未能报告启动失败：{this.state.reportError}</p>
        )}
        <DiagnosticSettings />
        <button className="primary-button" onClick={() => location.reload()}>
          重新加载界面
        </button>
      </main>
    );
  }
}
