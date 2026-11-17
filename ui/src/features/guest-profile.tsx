import React, { useEffect, useRef, useState } from "react";
import { api } from "../lib/api.ts";
import { InlineSelect } from "../components/ui/inline-select.tsx";
import { SafetyMessage } from "./safety-message.tsx";
import "../styles/guest-profile.css";
import type { GuestProfile, GuestProfileRecord } from "../lib/contracts.ts";
import { errorMessage } from "../lib/errors.ts";

const textFields = [
  ["hostname", "主机名", "留空生成独立的 PC 名称", 15],
  ["username", "工作账户", "developer", 24],
  ["work_home", "工作账户主目录", "留空使用 /home/工作账户名", 69],
  ["browser_username", "浏览器账户", "browser", 24],
  ["browser_home", "浏览器账户主目录", "留空使用 /home/浏览器账户名", 69],
  ["extension_name", "浏览器控制扩展名称", "Workbench browser panes", 64],
] as const;

export function GuestProfileOptions({
  value,
  onChange,
  onReady,
  disabled,
}: {
  value: GuestProfile | null;
  onChange: React.Dispatch<React.SetStateAction<GuestProfile | null>>;
  onReady: (ready: boolean) => void;
  disabled: boolean;
}) {
  const [record, setRecord] = useState<GuestProfileRecord | null>(null);
  const [error, setError] = useState("");
  const [generating, setGenerating] = useState(false);
  const [attempt, retry] = useState(0);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => {
    const request = new AbortController();
    onReady(false);
    setError("");
    api(
      "/environment/guest-profile",
      undefined,
      undefined,
      {},
      { signal: request.signal },
    )
      .then((result) => {
        if (request.signal.aborted) return;
        setRecord(result);
        onChange((current) =>
          result.editable ? current || result.defaults : null,
        );
        onReady(true);
      })
      .catch((error) => {
        if (!request.signal.aborted) setError(errorMessage(error));
      });
    return () => {
      request.abort();
      controller.current?.abort();
    };
  }, [onChange, onReady, attempt]);
  const options = record?.profile || value || record?.defaults;
  const locked = disabled || !record?.editable || generating;
  function set<K extends keyof GuestProfile>(key: K, next: GuestProfile[K]) {
    if (!options) return;
    const updated = { ...options, mode: "manual", [key]: next };
    if (key === "webgl" && next === "disabled")
      updated.graphics_backend = "auto";
    onChange(updated);
  }
  async function randomize() {
    if (!options) return;
    const request = new AbortController();
    controller.current = request;
    setGenerating(true);
    setError("");
    onReady(false);
    try {
      const preview = await api(
        "/environment/guest-profile/preview",
        {
          mode: "random",
          webgl: options.webgl || "native",
          canvas: options.canvas || "native",
          graphics_backend: options.graphics_backend || "auto",
        },
        undefined,
        {},
        { signal: request.signal },
      );
      if (!request.signal.aborted) onChange(preview);
    } catch (error) {
      if (!request.signal.aborted) setError(errorMessage(error));
    } finally {
      if (!request.signal.aborted) {
        setGenerating(false);
        onReady(true);
      }
    }
  }
  const fixed = options?.mode === "default";
  return (
    <section className="guest-profile-options" aria-label="客机创建配置">
      <h2>客机创建配置</h2>
      <p>
        默认使用 developer 工作账户、browser
        浏览器账户及当前设备设置。开始安装时保存，续装和重启复用同一份配置。
      </p>
      {error && (
        <p role="alert">
          <SafetyMessage message={error} />
          {!record && (
            <button onClick={() => retry((value) => value + 1)}>
              重新读取配置
            </button>
          )}
        </p>
      )}
      {!record && !error && <p role="status">正在读取创建配置…</p>}
      {record && !record.editable && (
        <p>
          {record.profile
            ? "创建配置已保存，继续安装将复用以下设置。"
            : "已有环境继续使用原账户、主目录和设备配置。"}
        </p>
      )}
      {record && options && (record.editable || record.profile) && (
        <>
          {record.editable && (
            <div className="guest-profile-actions">
              <button
                type="button"
                disabled={locked}
                aria-pressed={fixed}
                onClick={() => {
                  onChange(record.defaults);
                  setError("");
                }}
              >
                使用默认配置
              </button>
              <button
                type="button"
                disabled={locked}
                aria-pressed={options.mode === "random"}
                onClick={randomize}
              >
                {generating ? "正在生成…" : "随机生成配置"}
              </button>
              <button
                type="button"
                disabled={locked}
                aria-pressed={options.mode === "manual"}
                onClick={() => set("mode", "manual")}
              >
                手动配置
              </button>
            </div>
          )}
          <details open={!fixed || !record.editable}>
            <summary>
              {fixed ? "查看默认账户与设备设置" : "账户与设备设置"}
            </summary>
            <fieldset disabled={locked || fixed}>
              <div className="guest-profile-grid">
                {textFields.map(([key, label, placeholder, maxLength]) => (
                  <label key={key}>
                    {label}
                    <input
                      value={options[key] || ""}
                      maxLength={maxLength}
                      placeholder={placeholder}
                      onChange={(event) => set(key, event.target.value)}
                      autoComplete="off"
                    />
                  </label>
                ))}
                {(
                  [
                    ["display", "虚拟显示"],
                    ["browser", "浏览器初始窗口"],
                  ] as const
                ).map(([key, label]) => (
                  <div key={key} className="guest-profile-dimensions">
                    <span>{label}</span>
                    <div>
                      {(["width", "height"] as const).map((axis, i) => (
                        <label key={axis}>
                          <span>{i ? "高" : "宽"}</span>
                          <input
                            aria-label={`${label}${i ? "高度" : "宽度"}`}
                            type="number"
                            min={i ? 600 : 800}
                            max={i ? 2160 : 3840}
                            value={options[`${key}_${axis}`] || ""}
                            onChange={(event) =>
                              set(
                                `${key}_${axis}`,
                                event.target.value === ""
                                  ? 0
                                  : Number(event.target.value),
                              )
                            }
                          />
                        </label>
                      ))}
                    </div>
                  </div>
                ))}
                <label>
                  音频设备描述
                  <input
                    maxLength={64}
                    value={options.audio_description || ""}
                    onChange={(event) =>
                      onChange({
                        ...options,
                        mode: "manual",
                        audio_model: "",
                        audio_description: event.target.value,
                      })
                    }
                  />
                </label>
                <label>
                  音频设备名
                  <input
                    maxLength={120}
                    value={options.audio_name || ""}
                    onChange={(event) =>
                      onChange({
                        ...options,
                        mode: "manual",
                        audio_model: "",
                        audio_name: event.target.value,
                      })
                    }
                  />
                </label>
                <label>
                  图形渲染后端
                  <InlineSelect
                    inline
                    aria-label="图形渲染后端"
                    value={options.graphics_backend || "auto"}
                    disabled={locked || fixed || options.webgl === "disabled"}
                    onValueChange={(value) => set("graphics_backend", value)}
                  >
                    <option value="auto">自动使用客机可用设备</option>
                    <option value="vulkan">
                      ANGLE / Vulkan（需要客机驱动支持）
                    </option>
                    <option value="opengl">ANGLE / OpenGL</option>
                    <option value="software">SwiftShader 软件渲染</option>
                  </InlineSelect>
                </label>
                <label>
                  WebGL
                  <InlineSelect
                    inline
                    aria-label="WebGL"
                    value={options.webgl || "native"}
                    disabled={locked || fixed}
                    onValueChange={(value) => set("webgl", value)}
                  >
                    <option value="native">使用客机原生图形能力</option>
                    <option value="disabled">关闭网页 3D 图形能力</option>
                  </InlineSelect>
                </label>
                <label>
                  Canvas 图像读取
                  <InlineSelect
                    inline
                    aria-label="Canvas 图像读取"
                    value={options.canvas || "native"}
                    disabled={locked || fixed}
                    onValueChange={(value) => set("canvas", value)}
                  >
                    <option value="native">允许网页读取 Canvas 图像</option>
                    <option value="blocked">阻止网页读取 Canvas 图像</option>
                  </InlineSelect>
                </label>
              </div>
            </fieldset>
            <p>
              随机音频标签从公开型号中选择，表示虚拟音频设备的名称。图形能力以启动后的实际检测为准；关闭
              WebGL 或 Canvas 读取会影响部分网站。浏览器身份跟随实际安装版本。
            </p>
          </details>
        </>
      )}
    </section>
  );
}
