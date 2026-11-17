import { useEffect, useRef, useState } from "react";
import { api, uploadFile, native, nativeAvailable } from "../lib/api.ts";
import { Modal } from "../components/ui/modal.tsx";
import { Button, Input } from "../components/ui/controls.tsx";
import type { Upload, KnownPath } from "../lib/contracts.ts";
import type { Notify } from "../lib/types.ts";
import { errorMessage } from "../lib/errors.ts";

export function BrowserUploads({
  requests = [],
  onStatus,
}: {
  requests?: Upload[];
  onStatus: Notify;
}) {
  const [hidden, setHidden] = useState("");
  const request = requests.find(
    (item) =>
      !item.directory &&
      item.id !== hidden &&
      (!item.kind || item.kind === "chooser"),
  );
  const [progress, setProgress] = useState("");
  const [error, setError] = useState("");
  const [percent, setPercent] = useState(0);
  const [cancelPending, setCancelPending] = useState(false);
  const transfer = useRef<AbortController | null>(null);
  const generation = useRef(0);
  const current = useRef(request?.id);
  const canceling = useRef(false);
  current.current = request?.id;
  useEffect(() => {
    setProgress("");
    setError("");
    setCancelPending(false);
    canceling.current = false;
    return () => {
      generation.current++;
      transfer.current?.abort();
    };
  }, [request?.id]);
  if (!request) return null;
  const target = request.target || "browser";
  const action = (name: "finish" | "cancel"): KnownPath =>
    target === "browser" ? `/browser/upload-${name}` : `/input/files/${name}`;
  const body = { id: request.id, ...(target === "browser" ? {} : { target }) };
  const valid = (token: number) =>
    current.current === request.id && generation.current === token;
  const cancel = async () => {
    if (canceling.current) return;
    canceling.current = true;
    const token = ++generation.current;
    transfer.current?.abort();
    setCancelPending(true);
    setProgress("正在取消文件传递…");
    setError("");
    try {
      await api(action("cancel"), body);
      if (valid(token)) setHidden(request.id);
    } catch (error) {
      if (valid(token)) {
        setError(`取消文件传递未完成：${errorMessage(error)}`);
        setProgress("");
      }
    } finally {
      if (valid(token)) {
        canceling.current = false;
        setCancelPending(false);
      }
    }
  };
  const failed = async (error: unknown, token: number) => {
    if (!valid(token)) return;
    const message = errorMessage(error);
    setError(message);
    setProgress("");
    onStatus(`${message}；请重新点击上传按钮`);
    try {
      await api(action("cancel"), body);
    } catch (cleanupError) {
      if (valid(token)) {
        const detail = `${message}；取消文件传递未完成：${errorMessage(cleanupError)}`;
        setError(detail);
        onStatus(detail);
      }
    }
  };
  const select = async (files: File[]) => {
    if (!files.length) return;
    if (
      files.length > (request.multiple ? 100 : 1) ||
      files.some((file) => file.size > 2 * 1024 ** 3)
    ) {
      setError("每个文件不能超过 2 GB；多选最多 100 个文件。");
      return;
    }
    const controller = new AbortController();
    const token = ++generation.current;
    transfer.current = controller;
    setError("");
    try {
      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        setProgress(`正在传递 ${i + 1}/${files.length}：${file.name}`);
        setPercent(0);
        await uploadFile(
          `/input/files/${target}/${request.id}?${new URLSearchParams({ name: file.name })}`,
          file,
          {
            signal: controller.signal,
            onProgress: (loaded, total) => {
              if (valid(token))
                setPercent(
                  total
                    ? Math.min(100, Math.floor((loaded * 100) / total))
                    : 100,
                );
            },
          },
        );
        if (!valid(token)) return;
      }
      setProgress("正在交给应用…");
      await api(
        action("finish"),
        body,
        undefined,
        {},
        { signal: controller.signal },
      );
      if (!valid(token)) return;
      setHidden(request.id);
      onStatus("所选文件已交给应用，请在应用中确认上传结果");
    } catch (error) {
      if (!controller.signal.aborted) await failed(error, token);
    }
  };
  const choose = async () => {
    const token = ++generation.current;
    const controller = new AbortController();
    transfer.current = controller;
    setError("");
    setProgress("正在选择本机文件…");
    try {
      const selection = await native("chooseUploadFiles", {
        multiple: request.multiple,
      });
      if (!valid(token)) return;
      if (selection.canceled) {
        await cancel();
        return;
      }
      setProgress("正在传递所选文件…");
      await native("uploadHostFiles", {
        target,
        id: request.id,
        selection: selection.fileSelection,
      });
      if (!valid(token)) return;
      await api(
        action("finish"),
        body,
        undefined,
        {},
        { signal: controller.signal },
      );
      if (!valid(token)) return;
      setHidden(request.id);
      onStatus("所选文件已交给应用，请在应用中确认上传结果");
    } catch (error) {
      if (!controller.signal.aborted) await failed(error, token);
    }
  };
  return (
    <Modal
      title="上传本机文件"
      onClose={cancelPending ? undefined : () => void cancel()}
      footer={
        <Button disabled={cancelPending} onClick={() => void cancel()}>
          {cancelPending ? "正在取消…" : "取消上传"}
        </Button>
      }
    >
      <div className="modal-body browser-upload-body">
        <p>
          选择要上传的图片、视频或文档。仅传递所选文件，目标网页或应用负责处理附件。
        </p>
        {nativeAvailable() ? (
          <Button
            variant="primary"
            disabled={Boolean(progress)}
            onClick={() => void choose()}
          >
            选择本机文件
          </Button>
        ) : (
          <label className="field">
            选择本机文件
            <Input
              type="file"
              multiple={request.multiple}
              disabled={Boolean(progress)}
              onChange={(event) =>
                void select(Array.from(event.target.files || []))
              }
            />
          </label>
        )}
        {progress && (
          <>
            <p role="status">
              {progress}
              {nativeAvailable() ? "" : ` · ${percent}%`}
            </p>
            <progress
              aria-label="文件传递进度"
              max="100"
              value={nativeAvailable() ? undefined : percent}
            />
          </>
        )}
        {error && <p role="alert">{error}</p>}
      </div>
    </Modal>
  );
}
