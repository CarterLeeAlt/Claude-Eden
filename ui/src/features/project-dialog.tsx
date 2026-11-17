import React, { useState } from "react";
import {
  FolderMinus,
  FolderOpen,
  LoaderCircle,
  PencilLine,
  Plus,
} from "lucide-react";
import { api, native, nativeAvailable } from "../lib/api.ts";
import { SafetyMessage } from "./safety-message.tsx";
import { Modal } from "./views.tsx";
import { ProjectRecovery } from "./project-recovery.tsx";
import { projectGroups, projectName } from "../lib/workspace-state.ts";
import type { Status } from "../lib/contracts.ts";
import type { Settings } from "../lib/types.ts";
import { WorkbenchError, errorMessage } from "../lib/errors.ts";

export function ProjectDialog({
  status,
  initialPath = "",
  activate = true,
  onClose,
  onOpened,
  onRemoved,
  onSettings,
}: {
  status: Status;
  current?: Settings;
  initialPath?: string;
  activate?: boolean;
  onClose: () => void;
  onOpened: (path: string) => unknown;
  onRemoved?: (removed: string, workspace: string) => Promise<unknown>;
  onSettings?: () => void;
}) {
  const groups = projectGroups(status);
  const projects = groups.map((group) => group.workspace);
  const initialGroup = groups.find((group) =>
    group.folders.includes(initialPath),
  );
  const [adding, setAdding] = useState(
    Boolean(initialPath && !initialGroup) || projects.length === 0,
  );
  const [path, setPath] = useState(
    initialGroup?.workspace ||
      initialPath ||
      groups.find((group) => group.folders.includes(status.workspace))
        ?.workspace ||
      projects[0] ||
      "",
  );
  const [editing, setEditing] = useState("");
  const [name, setName] = useState("");
  const [folders, setFolders] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [recovery, setRecovery] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [removeTarget, setRemoveTarget] = useState("");
  const [removed, setRemoved] = useState<string[]>([]);
  const [browsing, setBrowsing] = useState(false);
  const projectList = projects.filter((project) => !removed.includes(project));
  const displayName = (value: string) => projectName(value, groups);
  const selectedGroup = groups.find((group) => group.workspace === path);
  const draftFolders = [
    ...new Set([...folders, ...(path.trim() ? [path.trim()] : [])]),
  ];
  const appendFolder = (value: string) => {
    const folder = value.trim();
    if (!folder) return;
    setFolders((previous) => [...new Set([...previous, folder])]);
    setName((previous) => previous || projectName(folder));
    setPath("");
    setError("");
  };
  const edit = () => {
    if (!selectedGroup) return;
    setEditing(selectedGroup.workspace);
    setName(selectedGroup.name);
    setFolders(selectedGroup.folders);
    setPath("");
    setAdding(true);
    setError("");
  };
  const busy = submitting || Boolean(status.job?.running);
  const remove = async () => {
    if (busy || !removeTarget) return;
    setSubmitting(true);
    setError("");
    try {
      const result = await api("/projects/remove", { workspace: removeTarget });
      setRemoved((previous) => [...previous, removeTarget]);
      setPath(result.workspace || "");
      if (!result.workspace) setAdding(true);
      setRemoveTarget("");
      await onRemoved?.(removeTarget, result.workspace);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSubmitting(false);
    }
  };
  const choose = async () => {
    setBrowsing(true);
    try {
      const selected = await native("chooseFolder");
      if (selected) {
        appendFolder(selected);
      }
    } catch {
      setError("未能选择文件夹，请直接填写路径。");
    } finally {
      setBrowsing(false);
    }
  };
  const add = () => {
    setAdding(true);
    setEditing("");
    setName("");
    setFolders([]);
    setPath("");
    setError("");
    if (nativeAvailable()) void choose();
  };
  const performOpen = async () => {
    let workspace = path.trim();
    if (adding) {
      const saved = await api("/projects/save", {
        workspace: editing,
        name: name.trim() || projectName(draftFolders[0]),
        folders: draftFolders,
      });
      workspace = saved.workspace;
      setEditing(saved.workspace);
    }
    const result = await api("/projects/open", {
      workspace,
      add: false,
      activate,
    });
    await onOpened(result.workspace);
  };
  const open = async (event?: React.SyntheticEvent) => {
    event?.preventDefault();
    if (
      busy ||
      removeTarget ||
      !status.ready ||
      !(adding ? draftFolders.length : path.trim())
    )
      return;
    setSubmitting(true);
    setError("");
    setRecovery("");
    try {
      await performOpen();
    } catch (e) {
      setError(errorMessage(e));
      setRecovery(e instanceof WorkbenchError ? e.recovery || "" : "");
    } finally {
      setSubmitting(false);
    }
  };
  return (
    <Modal
      title={
        adding
          ? editing
            ? "编辑项目"
            : "创建项目"
          : activate
            ? "打开项目"
            : "选择工作项目"
      }
      onClose={busy ? undefined : onClose}
      footer={
        <>
          <span className="save-note">
            {busy
              ? submitting
                ? removeTarget
                  ? "正在移除项目…"
                  : "正在连接项目…"
                : status.job?.message
              : adding
                ? editing
                  ? "会话历史保留"
                  : "首个文件夹为默认目录"
                : "使用已授权的 Claude Eden 项目"}
          </span>
          <button disabled={busy} onClick={onClose}>
            取消
          </button>
          <button
            className="primary"
            form="open-project"
            type="submit"
            disabled={
              !(adding ? draftFolders.length : path.trim()) ||
              busy ||
              Boolean(removeTarget) ||
              !status.ready
            }
          >
            {busy ? (
              <LoaderCircle className="spin" size={15} />
            ) : (
              <FolderOpen size={15} />
            )}
            {adding
              ? editing
                ? "保存项目"
                : "创建项目"
              : activate
                ? "打开项目"
                : "使用此项目"}
          </button>
        </>
      }
    >
      <form
        id="open-project"
        className="modal-body project-form"
        onSubmit={open}
      >
        <p>
          {adding
            ? "为项目命名，并添加需要一起使用的文件夹。"
            : activate
              ? "选择要打开的项目，或添加新项目。"
              : "从 Claude Eden 项目列表选择工作目录。新项目由 Claude Eden 授权并共享。"}
        </p>
        {!adding && (
          <>
            <fieldset
              className="project-picker-list"
              disabled={busy || Boolean(removeTarget)}
            >
              <legend>项目</legend>
              {projectList.map((project) => (
                <div
                  className={`project-picker-row ${path === project ? "selected" : ""}`}
                  key={project}
                >
                  <label className="project-picker-item">
                    <input
                      type="radio"
                      name="project"
                      value={project}
                      checked={path === project}
                      onChange={() => {
                        setPath(project);
                        setError("");
                      }}
                    />
                    <FolderOpen size={18} aria-hidden="true" />
                    <span>
                      <strong>{displayName(project)}</strong>
                      <small>
                        {groups
                          .find((g) => g.workspace === project)
                          ?.folders.join(" · ") || project}
                      </small>
                      {status.project_errors?.some((item) =>
                        (
                          groups.find((group) => group.workspace === project)
                            ?.folders || [project]
                        ).includes(item.path),
                      ) && <small>需要重新连接，选择后重试</small>}
                    </span>
                  </label>
                  {path === project && !removeTarget && (
                    <div className="project-picker-actions">
                      {selectedGroup && (
                        <button
                          type="button"
                          className="icon-button"
                          aria-label="编辑名称与文件夹…"
                          disabled={busy || !status.ready}
                          onClick={edit}
                        >
                          <PencilLine size={16} aria-hidden="true" />
                        </button>
                      )}
                      {activate && (
                        <button
                          type="button"
                          className="icon-button project-picker-remove"
                          aria-label="从 Claude Eden 移除此项目"
                          disabled={busy}
                          onClick={() => {
                            setRemoveTarget(path);
                            setError("");
                            setRecovery("");
                          }}
                        >
                          <FolderMinus size={16} aria-hidden="true" />
                        </button>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </fieldset>
            {removeTarget && (
              <section
                className="project-picker-list"
                aria-label="确认移除项目"
              >
                <p>
                  移除“{displayName(removeTarget)}
                  ”将解除所有绑定文件夹的共享。原文件和会话历史保留，重新添加相同目录后可继续使用历史。
                </p>
                <p className="field-help">
                  请先保存文件，并关闭此项目的终端、运行中会话和编辑器。移除当前项目后选中剩余项目；移除最后一个项目后，可重新添加文件夹。
                </p>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    setRemoveTarget("");
                    setError("");
                  }}
                >
                  取消移除
                </button>
                <button type="button" disabled={busy} onClick={remove}>
                  确认移除项目
                </button>
              </section>
            )}
            <button
              type="button"
              className="quiet-button project-picker-add"
              onClick={add}
              disabled={busy || Boolean(removeTarget) || !status.ready}
            >
              <Plus size={16} />
              添加本机项目…
            </button>
          </>
        )}
        {adding && (
          <>
            {projects.length > 0 && (
              <button
                type="button"
                className="quiet-button project-picker-add"
                disabled={busy}
                onClick={() => {
                  setAdding(false);
                  setEditing("");
                  setFolders([]);
                  setPath(
                    groups.find((group) =>
                      group.folders.includes(status.workspace),
                    )?.workspace || projects[0],
                  );
                  setError("");
                }}
              >
                返回已有项目
              </button>
            )}
            <label htmlFor="project-name">项目名称</label>
            <input
              id="project-name"
              autoFocus
              value={name}
              maxLength={100}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如：课程系统"
              disabled={busy}
            />
            <label>源文件夹</label>
            {folders.length > 0 && (
              <ul className="project-folder-list" aria-label="已绑定文件夹">
                {folders.map((folder, index) => (
                  <li key={folder}>
                    <FolderOpen size={16} aria-hidden="true" />
                    <span>
                      <strong>{projectName(folder)}</strong>
                      <small>{folder}</small>
                    </span>
                    {index === 0 && (
                      <small className="project-folder-default">默认</small>
                    )}
                    <button
                      type="button"
                      disabled={busy || folder === editing}
                      aria-label={`移除文件夹 ${projectName(folder)}`}
                      title={
                        folder === editing
                          ? "默认文件夹用于关联会话历史，需保留"
                          : "解除文件夹绑定"
                      }
                      onClick={() =>
                        setFolders((previous) =>
                          previous.filter((value) => value !== folder),
                        )
                      }
                    >
                      移除
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <label htmlFor="project-path">添加文件夹</label>
            <div className="folder-input">
              <input
                id="project-path"
                value={path}
                onChange={(event) => {
                  setPath(event.target.value);
                  setError("");
                }}
                placeholder="C:/Users/你的名字/Projects/my-project"
                disabled={busy}
                spellCheck="false"
                autoComplete="off"
              />
              {path.trim() && (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => appendFolder(path)}
                >
                  添加
                </button>
              )}
              {nativeAvailable() && (
                <button
                  type="button"
                  onClick={choose}
                  disabled={busy || browsing}
                >
                  {browsing ? "正在读取…" : "选择文件夹…"}
                </button>
              )}
            </div>
          </>
        )}
        {adding && (
          <p className="field-help">
            Claude
            从默认文件夹启动，并可读写其余绑定文件夹。修改绑定后，新建或重新打开会话生效。
            {editing &&
              "默认文件夹用于关联会话历史，需保留。解绑前请关闭项目的终端、运行中会话和编辑器。"}
          </p>
        )}
        <p className="field-help project-picker-note">
          <span>绑定文件夹中的改动会直接写入本机。</span>
          <span>所有项目共用 Claude Eden 环境，已有终端和网页会保持运行。</span>
        </p>
        {!status.ready && (
          <p className="field-help">
            环境就绪后可添加或打开项目。已有项目仍可从列表移除。
          </p>
        )}
        {error && (
          <div className="error-message" role="alert">
            <SafetyMessage message={error} />
            {recovery && (
              <ProjectRecovery
                recovery={recovery}
                disabled={busy}
                onRetry={() => void open()}
                onSettings={onSettings}
              />
            )}
          </div>
        )}
      </form>
    </Modal>
  );
}
