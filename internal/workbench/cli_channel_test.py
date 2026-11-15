import concurrent.futures
import importlib.util
import io
import json
import os
from pathlib import Path
import select
import runpy
import stat
import subprocess
import sys
import tempfile
import time
import unittest
import uuid
from unittest.mock import patch

sys.dont_write_bytecode = True
source = Path(__file__).with_name("cli_channel.py").resolve()
spec = importlib.util.spec_from_file_location("channel", source)
channel = importlib.util.module_from_spec(spec)
spec.loader.exec_module(channel)


class ChannelTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        # Match the production config resolver when temporary paths use links.
        self.home = Path(temporary.name).resolve()
        self.env = {**os.environ, "HOME": str(self.home), "CLAUDE_CONFIG_DIR": str(self.home / ".claude"), "TMPDIR": str(self.home)}
        environment = patch.dict(os.environ, self.env)
        environment.start()
        self.addCleanup(environment.stop)
        self.root = self.home / ".local/share/workbench/cli-channels"
        runtime = runpy.run_path(source.parent.parent / "eden/client_runtime.py", init_globals={"_controller_context": channel._controller_context})
        runtime_patch = patch.object(channel, "_runtime", runtime)
        runtime_patch.start()
        self.addCleanup(runtime_patch.stop)
        root_patch = patch.object(channel, "ROOT", self.root)
        root_patch.start()
        self.addCleanup(root_patch.stop)
        self.session = str(uuid.uuid4())
        self.project = str(self.home / "project")

    def history(self, session, workspace, messages):
        path = self.home / ".claude/projects/project" / (session + ".jsonl")
        path.parent.mkdir(parents=True, exist_ok=True)
        rows = [{"cwd": workspace, "uuid": str(uuid.uuid4()), **message} for message in messages]
        path.write_text("\n".join(json.dumps(row) for row in rows) + "\n")
        return path

    def test_channel_body_preserves_markup(self):
        request = str(uuid.uuid4())
        body = "show </channel> literally\n<channel request_id=\"other\">example</channel>"
        self.assertIsNone(channel.channel_message('<channel source="cage" cage_request="' + request + '">\n' + body + '\n</channel>'))
        self.assertEqual(channel.channel_message('<channel source="session" request_id="' + request + '">\n' + body + '\n</channel>'), (request, body))
        self.assertEqual(channel.clip_text("你好abc", 4), "你")

    def test_workspace_boundaries_use_linux_paths(self):
        self.assertTrue(channel.workspace_within("/project/Case", "/project/Case"))
        self.assertTrue(channel.workspace_within("/project/Case/nested", "/project/Case/"))
        self.assertFalse(channel.workspace_within("/project/case", "/project/Case"))
        self.assertFalse(channel.workspace_within("/project/Case-other", "/project/Case"))
        self.assertFalse(channel.workspace_within("/project/Case\\nested", "/project/Case"))

    def test_account_switch_preserves_tools_and_receipt(self):
        tool = {"type":"tool_use","id":"tool-completed","name":"Bash","input":{"command":"already completed"}}
        path = self.history(self.session,self.project,[
            {"type":"user","message":{"content":"perform task"}},
            {"type":"assistant","message":{"content":[tool]}},
            {"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool-completed","content":"done"}]}},
        ])
        connection,switch_id,target = [str(uuid.uuid4()) for _ in range(3)]
        request = {"cli_id":self.session,"config":str(self.home / ".claude"),"connection":connection,"workspace":self.project,"target_cli_id":target,"switch_id":switch_id}
        channel.save(self.root/connection/"state.json",{"cli_id":self.session,"config":request["config"],"workspace":self.project,"busy":False,"requests":{"accepted":"digest"}})
        completed = type("Result",(),{"returncode":0,"stdout":"[]"})()
        original = path.read_bytes()
        with patch.object(channel,"active_sessions",return_value=[]),patch.object(channel.subprocess,"run",return_value=completed):
            result=channel.prepare_account_switch(request)
            self.assertEqual(Path(result["transcript"]).read_bytes(),original)
            self.assertEqual(channel.prepare_account_switch(request),result)
        receipt=channel.read_json(Path(result["transcript"]).with_name("receipt.json"))
        self.assertEqual(receipt["requests"],{"accepted":"digest"})
        self.assertEqual(path.read_bytes(),original)
        payload={"hook_event_name":"PreToolUse","session_id":self.session}
        with patch.object(channel.sys,"stdin",io.StringIO(json.dumps(payload))):
            denied=channel.hook(connection)
        self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"],"deny")
        self.assertTrue(channel.cancel_account_switch(request)["ok"])
        self.assertFalse(Path(result["transcript"]).exists())
        self.assertTrue(path.exists())

    def test_account_switch_refuses_busy_and_unresolved_tools(self):
        path=self.history(self.session,self.project,[{"type":"assistant","message":{"content":[{"type":"tool_use","id":"uncertain","name":"Bash","input":{}}]}}])
        connection,switch_id,target=[str(uuid.uuid4()) for _ in range(3)]
        request={"cli_id":self.session,"config":str(self.home/".claude"),"connection":connection,"workspace":self.project,"target_cli_id":target,"switch_id":switch_id}
        state={"cli_id":self.session,"config":request["config"],"workspace":self.project,"busy":True}
        channel.save(self.root/connection/"state.json",state)
        with patch.object(channel.os,"kill") as kill:
            with self.assertRaises(ValueError): channel.prepare_account_switch(request)
            kill.assert_not_called()
        state["busy"]=False
        channel.save(self.root/connection/"state.json",state)
        completed=type("Result",(),{"returncode":0,"stdout":"[]"})()
        with patch.object(channel,"active_sessions",return_value=[]),patch.object(channel.subprocess,"run",return_value=completed):
            with self.assertRaisesRegex(ValueError,"工具"): channel.prepare_account_switch(request)
        self.assertTrue(path.exists())
        self.assertFalse((self.root.parent/"cli-account-switches"/switch_id/"history.jsonl").exists())

    def test_failure_hook_reports_kind_and_new_turn_clears_it(self):
        payload={"hook_event_name":"StopFailure","session_id":self.session,"error":"rate_limit","error_details":"private upstream diagnostic"}
        with patch.object(channel.sys,"stdin",io.StringIO(json.dumps(payload))):channel.hook(self.session)
        state=channel.read_json(self.root/self.session/"state.json")
        self.assertEqual(state["account_failure"],"rate_limit")
        self.assertNotIn("private upstream",json.dumps(state))
        with patch.object(channel.sys,"stdin",io.StringIO(json.dumps({**payload,"hook_event_name":"UserPromptSubmit"}))):channel.hook(self.session)
        self.assertNotIn("account_failure",channel.read_json(self.root/self.session/"state.json"))

    def test_first_display_delta_after_launch(self):
        with channel.state_file(self.session) as state:
            state["display"] = None
        payload = {"hook_event_name":"MessageDisplay","session_id":self.session,"message_id":"reply","index":0,"delta":"hello"}
        with patch.object(channel.sys,"stdin",io.StringIO(json.dumps(payload))):
            channel.hook(self.session)
        self.assertEqual(channel.read_json(self.root / self.session / "state.json")["display"]["text"],"hello")

    def test_ready_channel_wins_over_a_newer_unready_duplicate(self):
        config = str(channel.config_directory("default"))
        ready, starting = str(uuid.uuid4()), str(uuid.uuid4())
        channel.save(self.root / ready / "state.json", {"cli_id":self.session,"config":config,"pid":os.getpid(),"channel_pid":os.getpid(),"updated_at":1})
        channel.save(self.root / starting / "state.json", {"cli_id":self.session,"config":config,"pid":os.getpid(),"channel_pid":None,"updated_at":2})
        self.assertEqual(channel.connection_for(self.session,Path(config))[0],ready)

    def test_launch_retains_pending_permission_mode(self):
        config = str(channel.config_directory("default"))
        channel.save(channel.handoff_receipt(config,self.session),{"resume_task":True,"permission_mode":"manual"})
        connection = str(uuid.uuid4())
        with patch.object(channel,"active_session",return_value=False), patch.object(channel.os,"execvpe") as execute:
            channel.launch({"id":connection,"resume":self.session,"prefix":["claude"],"config":config})
        self.assertEqual(execute.call_args.args[1][-2:],["--permission-mode","manual"])
        self.assertTrue(channel.read_json(self.root / connection / "state.json")["resume_task"])

    def test_launch_selects_the_configured_session_channel(self):
        connection = str(uuid.uuid4())
        with patch.object(channel.os, "execvpe") as execute:
            channel.launch({"id": connection, "prefix": ["claude"]})
        args = execute.call_args.args[1]
        config = channel.read_json(Path(args[args.index("--mcp-config") + 1]))
        selected = args[args.index("--dangerously-load-development-channels") + 1]
        self.assertEqual(selected, "server:" + connection)
        self.assertEqual(list(config["mcpServers"]), [connection])
        settings = channel.read_json(Path(args[args.index("--settings") + 1]))
        argv = config["mcpServers"][connection]
        self.assertEqual(argv["command"], sys.executable)
        self.assertIn(str(source), argv["args"])
        self.assertEqual(args[args.index("--settings") + 1], str(self.root / connection / "settings.json"))
        self.assertIn(str(source), settings["hooks"]["SessionStart"][0]["hooks"][0]["command"])
        self.assertFalse(set(execute.call_args.args[2]) & channel.client_runtime()["_CONTROL_KEYS"])

    def test_launch_adds_bound_directories_as_literal_arguments(self):
        directory = self.home / "backend ' $literal"
        directory.mkdir()
        connection = str(uuid.uuid4())
        with patch.object(channel.os, "execvpe") as execute:
            channel.launch({"id": connection, "prefix": ["claude"], "additional_directories": [str(directory)]})
        args = execute.call_args.args[1]
        self.assertEqual(args[args.index("--add-dir") + 1], str(directory))
        self.assertFalse(set(execute.call_args.args[2]) & channel.client_runtime()["_CONTROL_KEYS"])
        with patch.object(channel.os, "execvpe") as execute:
            with self.assertRaisesRegex(ValueError, "项目文件夹未连接"):
                channel.launch({"id": str(uuid.uuid4()), "prefix": ["claude"], "additional_directories": [str(directory / "missing")]})
            execute.assert_not_called()

    def test_launch_preserves_linux_ccs_prefix(self):
        connection = str(uuid.uuid4())
        prefix = ["ccs", "project-profile"]
        with patch.object(channel.os, "execvpe") as execute:
            channel.launch({"id": connection, "prefix": prefix})
        self.assertEqual(execute.call_args.args[0], "ccs")
        self.assertEqual(execute.call_args.args[1][:2], prefix)
        self.assertFalse(set(execute.call_args.args[2]) & channel.client_runtime()["_CONTROL_KEYS"])

    def test_managed_account_launch_preserves_config_subject_and_statusline(self):
        account_id, subject = "a" * 24, "b" * 64
        helper = self.home / "bin/workbench-claude-account"
        config = self.home / ".local/share/workbench/claude-accounts" / account_id / "config"
        prefix = ["env", "WORKBENCH_CLAUDE_SUBJECT=" + subject, str(helper), "run", account_id]
        profile = {"config": str(config)}
        for configured in (False, True):
            with self.subTest(configured=configured):
                connection = str(uuid.uuid4())
                account = {"profile": lambda value: profile, "has_status_line": lambda value: configured}
                with patch.object(channel.runpy, "run_path", return_value=account), patch.dict(channel._controller_context), patch.object(channel.os, "execvpe") as execute:
                    channel.launch({"id": connection, "prefix": prefix})
                    self.assertEqual(channel._controller_context["WORKBENCH_CLAUDE_SUBJECT"], subject)
                    expected_statusline = channel.client_runtime()["command_text"](str(helper), ["statusline", account_id])
                self.assertEqual(os.environ["CLAUDE_CONFIG_DIR"], str(config))
                self.assertEqual(execute.call_args.args[1][:5], prefix)
                settings = channel.read_json(self.root / connection / "settings.json")
                self.assertEqual("statusLine" in settings, not configured)
                if not configured:
                    self.assertEqual(settings["statusLine"]["command"], expected_statusline)
                self.assertEqual(channel.read_json(self.root / connection / "state.json")["config"], str(config))

    def test_bound_directory_reaches_isolated_child_and_host_write(self):
        default = Path(self.project)
        default.mkdir()
        extra = self.home / "backend ' $literal"
        extra.mkdir()
        binaries = self.home / "bin"
        binaries.mkdir()
        executable = binaries / "claude"
        executable.write_text("#!" + sys.executable + "\n" +
            "import json,os,sys\nfrom pathlib import Path\n" +
            "directory=Path(sys.argv[sys.argv.index('--add-dir')+1])\n" +
            "(directory/'from-child.txt').write_text('shared')\n" +
            "print(json.dumps({'cwd':os.getcwd(),'args':sys.argv[1:],'env':dict(os.environ)}))\n")
        executable.chmod(0o700)
        staged = binaries / "cli_channel.py"
        staged.write_bytes(source.read_bytes())
        (binaries / "client_runtime.py").write_bytes((source.parent.parent / "eden/client_runtime.py").read_bytes())
        env = {**self.env, "PATH": str(binaries) + os.pathsep + os.environ.get("PATH", os.defpath), "WORKBENCH_SESSION": "1", "WORKBENCH_USER_SETTING": "preserved"}
        request = {"id": str(uuid.uuid4()), "prefix": ["claude"], "additional_directories": [str(extra)]}
        result = subprocess.run([sys.executable, "-I", str(staged), "launch", json.dumps(request)],
            cwd=default, env=env, capture_output=True, text=True, timeout=10, check=True)
        received = json.loads(result.stdout)
        self.assertEqual(received["cwd"], str(default))
        self.assertEqual(received["args"][received["args"].index("--add-dir")+1], str(extra))
        self.assertFalse(set(received["env"]) & channel.client_runtime()["_CONTROL_KEYS"])
        self.assertEqual(received["env"]["WORKBENCH_USER_SETTING"], "preserved")
        self.assertEqual((extra / "from-child.txt").read_text(), "shared")

    def test_handoff_preserves_executing_tools(self):
        request = {"cli_id":self.session, "config":str(self.home / ".claude"), "workspace":self.project}
        for status in ("busy", None):
            rows = [{"sessionId":self.session, "pid":1234, "cwd":self.project, "status":status, "kind":"interactive"}]
            with patch.object(channel, "active_sessions", return_value=rows), patch.object(channel.os, "kill") as kill:
                self.assertTrue(channel.handoff_session(request)["running"])
                kill.assert_not_called()

    def test_handoff_rechecks_idle_identity_and_waits_for_exit(self):
        request = {"cli_id":self.session, "config":str(self.home / ".claude"), "workspace":self.project}
        row = {"sessionId":self.session, "pid":1234, "cwd":self.project, "status":"idle", "kind":"interactive"}
        with patch.object(channel, "active_sessions", return_value=[row]), patch.object(channel, "live", return_value=False), patch.object(channel.os, "kill") as kill:
            self.assertFalse(channel.handoff_session(request)["running"])
            kill.assert_called_once_with(1234, channel.signal.SIGTERM)
        with patch.object(channel, "active_sessions", side_effect=[[row], [{**row, "status":"busy"}]]), patch.object(channel.os, "kill") as kill:
            self.assertTrue(channel.handoff_session(request)["running"])
            kill.assert_not_called()

    def test_handoff_waiting_task_requires_fresh_approval(self):
        request = {"cli_id":self.session, "config":str(self.home / ".claude"), "workspace":self.project}
        row = {"sessionId":self.session, "pid":1234, "cwd":self.project, "status":"waiting", "kind":"interactive"}
        with patch.object(channel, "active_sessions", return_value=[row]), patch.object(channel, "live", return_value=False), patch.object(channel.os, "kill") as kill:
            self.assertFalse(channel.handoff_session(request)["running"])
            kill.assert_called_once_with(1234, channel.signal.SIGTERM)
        self.assertTrue(channel.read_json(channel.handoff_receipt(request["config"], self.session))["resume_task"])
        with patch.object(channel, "active_sessions", return_value=[{**row, "cwd":self.project + "-other"}]), patch.object(channel.os, "kill") as kill:
            with self.assertRaises(ValueError): channel.handoff_session(request)
            kill.assert_not_called()

    @patch.object(channel, "running_sessions", return_value=[])
    def test_catalog_activity_requires_live_busy_process(self, _registry):
        self.history(self.session, self.project, [{"type":"user", "message":{"content":"hello"}}])
        connection = str(uuid.uuid4())
        with channel.state_file(connection) as state:
            state.update(cli_id=self.session, config=str(self.home / ".claude"), workspace=self.project,
                         pid=os.getpid(), busy=True)
        request = {"accounts":["default"], "workspaces":{self.project:self.project}}
        item = channel.discover(request)[0]
        self.assertTrue(item["active"])
        self.assertTrue(item["running"])
        self.assertTrue(channel.view(self.home / ".claude", self.session)["active"])
        with channel.state_file(connection) as state:
            state["busy"] = False
        item = channel.discover(request)[0]
        self.assertFalse(item["active"], "idle CLI was shown as active")
        self.assertTrue(item["running"], "idle owned process was shown as stopped")
        with channel.state_file(connection) as state:
            state.update(busy=True, closed=True)
        item = channel.discover(request)[0]
        self.assertFalse(item["active"], "closed CLI retained activity")
        self.assertFalse(item["running"])

    def test_discovery_reports_external_idle_process_and_clears_after_exit(self):
        self.history(self.session, self.project, [{"type":"user", "message":{"content":"hello"}}])
        other = str(uuid.uuid4())
        self.history(other, self.project, [{"type":"user", "message":{"content":"other"}}])
        request = {"accounts":["default"], "workspaces":{self.project:self.project}}
        registry = [{"sessionId": self.session, "pid": os.getpid(), "status":"idle"}]
        completed = subprocess.CompletedProcess([], 0, json.dumps(registry), "")
        with patch.object(channel.subprocess, "run", return_value=completed) as run:
            found = {item["cli_id"]: item for item in channel.discover(request)}
            self.assertTrue(found[self.session]["running"])
            self.assertFalse(found[self.session]["active"])
            self.assertFalse(found[other]["running"])
            self.assertEqual(run.call_count, 1, "registry must be read once per config, not per history")
            self.assertEqual(run.call_args.kwargs["env"]["CLAUDE_CONFIG_DIR"], str(self.home / ".claude"))
            completed.stdout = "[]"
            self.assertFalse(any(item["running"] for item in channel.discover(request)))

    def test_discovery_identifies_desktop_from_live_registration_only(self):
        self.history(self.session, self.project, [{"type":"user", "entrypoint":"claude-desktop", "message":{"content":"hello"}}])
        request = {"accounts":["default"], "workspaces":{self.project:self.project}}
        row = {"sessionId":self.session, "pid":os.getpid(), "startedAt":123, "status":"idle"}
        registry = self.home / ".claude/sessions" / (str(os.getpid()) + ".json")
        channel.save(registry, {**row, "entrypoint":"claude-desktop"})
        with patch.object(channel, "running_sessions", return_value=[row]):
            for entrypoint in ("claude-desktop", "claude-desktop-3p", "local-agent", "local_agent", "remote_desktop", "cli", "claude-vscode", "sdk-ts", "", "future-client"):
                with self.subTest(entrypoint=entrypoint):
                    channel.save(registry, {**row, "entrypoint":entrypoint})
                    self.assertEqual(channel.discover(request)[0]["claude_desktop"], entrypoint in ("claude-desktop", "claude-desktop-3p", "local-agent", "local_agent", "remote_desktop"))
            for invalid in ({**row, "startedAt":122}, {**row, "sessionId":str(uuid.uuid4())}, {**row, "pid":1}):
                channel.save(registry, {**invalid, "entrypoint":"claude-desktop"})
                self.assertFalse(channel.discover(request)[0]["claude_desktop"], "stale registration claimed Desktop ownership")
            registry.write_text("invalid json")
            self.assertFalse(channel.discover(request)[0]["claude_desktop"])
            registry.unlink()
            self.assertFalse(channel.discover(request)[0]["claude_desktop"], "transcript origin was used as live ownership")
            row["entrypoint"] = "claude-desktop"
            self.assertTrue(channel.discover(request)[0]["claude_desktop"], "explicit CLI source was ignored")
        with patch.object(channel, "running_sessions", return_value=[]):
            self.assertFalse(channel.discover(request)[0]["claude_desktop"])

    def test_purge_removes_only_target_transcripts_and_retries(self):
        target = self.history(self.session, self.project, [{"type":"user", "message":{"content":"delete this"}}])
        other = self.history(str(uuid.uuid4()), self.project, [{"type":"user", "message":{"content":"keep this"}}])
        child = target.parent / self.session / "subagents" / "agent-fixture.jsonl"
        child.parent.mkdir(parents=True)
        child.write_text("child history")
        project_file = self.home / "project" / "main.py"
        project_file.parent.mkdir()
        project_file.write_text("project content")
        request = {"cli_id":self.session, "account":"default", "workspace":self.project, "expect_history":True}
        with patch.object(channel, "active_session", return_value=True):
            with self.assertRaises(ValueError): channel.purge_session(request)
        self.assertTrue(target.exists())
        with patch.object(channel, "active_session", return_value=False):
            with self.assertRaises(ValueError): channel.purge_session({**request, "workspace":self.project + "-other"})
            self.assertTrue(channel.purge_session(request)["ok"])
            self.assertTrue(channel.purge_session(request)["ok"])
        self.assertFalse(target.exists())
        self.assertFalse(child.parent.parent.exists())
        self.assertTrue(other.exists())
        self.assertEqual(project_file.read_text(), "project content")

    def test_purge_never_accesses_legacy_mobile_state(self):
        roots = [self.root.parent / name for name in ("mobile-cli-identities", "mobile-cli-sessions")]
        read_json, unlink = channel.read_json, Path.unlink
        def check_path(path):
            self.assertFalse(any(path.is_relative_to(root) for root in roots), "purge accessed legacy mobile state")
        def read_current(path, *args, **kwargs):
            check_path(path)
            return read_json(path, *args, **kwargs)
        def unlink_current(path, *args, **kwargs):
            check_path(path)
            return unlink(path, *args, **kwargs)
        for existing in (False, True):
            with self.subTest(existing=existing):
                session = str(uuid.uuid4())
                history = self.history(session, self.project, [{"type":"user", "message":{"content":"delete current history"}}])
                legacy = [root / session for root in roots]
                if existing:
                    for path in legacy:
                        path.parent.mkdir(parents=True)
                        path.write_bytes(b"preserve legacy state\n")
                before = {path: (path.read_bytes(), path.stat().st_mtime_ns) for path in legacy if path.exists()}
                request = {"cli_id":session, "account":"default", "workspace":self.project, "expect_history":True}
                with patch.object(channel, "active_session", return_value=False), patch.object(channel, "read_json", side_effect=read_current), patch.object(Path, "unlink", unlink_current):
                    self.assertTrue(channel.purge_session(request)["ok"])
                    self.assertTrue(channel.purge_session(request)["ok"])
                self.assertFalse(history.exists())
                self.assertEqual({path: (path.read_bytes(), path.stat().st_mtime_ns) for path in legacy if path.exists()}, before)
                self.assertEqual([root.exists() for root in roots], [existing, existing])

    def test_launch_reads_policy_without_migrating_or_rewriting_it(self):
        policy = self.home / "managed-settings.json"
        original = {"permissions": {"defaultMode": "bypassPermissions"}, "tui": "fullscreen"}
        policy.write_text(json.dumps(original))
        def path(value):
            return policy if str(value) == "/etc/claude-code/managed-settings.json" else Path(value)
        with patch.object(channel, "Path", wraps=Path, side_effect=path), patch.object(channel.os, "execvpe"), patch.object(channel.subprocess, "run") as run:
            for value in (original, {**original, "channelsEnabled": False}, {**original, "channelsEnabled": True}):
                policy.write_text(json.dumps(value))
                connection = str(uuid.uuid4())
                channel.launch({"id": connection, "prefix": ["claude"]})
                self.assertEqual(json.loads(policy.read_text()), value)
                self.assertEqual(channel.read_json(self.root / connection / "state.json")["blocked"], value.get("channelsEnabled") is False)
            run.assert_not_called()

    @patch.object(channel, "running_sessions", return_value=[])
    def test_history_projects_and_channel_identity(self, _registry):
        request = str(uuid.uuid4())
        self.history(self.session, self.project + "/nested", [
            {"type": "user", "message": {"content": "desktop question"}},
            {"type": "assistant", "message": {"content": [{"type": "text", "text": "desktop answer"}]}},
            {"type": "assistant", "message": {"model":"<synthetic>", "content": [{"type": "text", "text": "No response requested."}]}},
            {"type": "user", "isMeta": True, "message": {"content": "system metadata"}},
            {"type": "user", "isMeta": True, "message": {"content": '<channel source="session" request_id="' + request + '">\nshared session question\n</channel>'}},
            {"type": "user", "message": {"content": [{"type": "tool_result", "content": "private tool output"}]}},
            {"type": "assistant", "message": {"content": [{"type": "text", "text": "shared session answer"}]}},
        ])
        self.history(str(uuid.uuid4()), self.project + "-other", [{"type": "user", "message": {"content": "outside project"}}])
        found = channel.discover({"accounts": ["default"], "workspaces": [self.project]})
        self.assertEqual([item["cli_id"] for item in found], [self.session])
        view = channel.view(channel.config_directory("default"), self.session)
        self.assertFalse(view["can_send"])
        self.assertEqual(len(view["turns"]), 2)
        self.assertEqual(view["turns"][1]["id"], request)
        self.assertEqual(view["turns"][1]["prompt"], "shared session question")
        self.assertEqual(view["turns"][1]["text"], "shared session answer")
        self.assertNotIn("private tool output", json.dumps(view))

    def test_history_pairs_tool_only_rows_and_keeps_task_notice(self):
        self.history(self.session, self.project, [
            {"type": "user", "message": {"content": "run"}},
            {"type": "assistant", "message": {"content": [
                {"type": "tool_use", "id": "tool-1", "name": "Bash", "input": {"description": "检查结果", "command": "private-command"}},
                {"type": "tool_use", "id": "tool-2", "name": "Read", "input": {"file_path": "result.md"}},
            ]}},
            {"type": "user", "message": {"content": [
                {"type": "tool_result", "tool_use_id": "tool-2", "content": "private-output"},
                {"type": "tool_result", "tool_use_id": "tool-1", "is_error": True},
            ]}},
            {"type": "user", "isMeta": True, "message": {"content": "<task-notification><summary>完成</summary></task-notification>"}},
        ])
        view = channel.view(channel.config_directory("default"), self.session)
        self.assertEqual(len(view["turns"]), 2)
        tools = view["turns"][0]["tools"]
        self.assertEqual([(t["name"], t["status"], t["detail"]) for t in tools],
                         [("Bash", "failed", "检查结果"), ("Read", "completed", "result.md")])
        self.assertTrue(view["turns"][1]["prompt"].startswith("<task-notification>"))
        self.assertNotIn("private-", json.dumps(view))

    def test_concurrent_send_and_stale_approval(self):
        with channel.state_file(self.session) as state:
            state.update(cli_id=self.session, config=str(channel.config_directory("default")), workspace=self.project, pid=os.getpid())
        process = subprocess.Popen([sys.executable, "-I", str(source), "serve", self.session],
                                   env=self.env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        def stop():
            process.stdin.close()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            process.stdout.close()
            process.stderr.close()
        self.addCleanup(stop)
        def event(message):
            process.stdin.write(json.dumps({"jsonrpc": "2.0", **message}) + "\n")
            process.stdin.flush()
        event({"id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05"}})
        handshake = json.loads(process.stdout.readline())["result"]
        self.assertIn("claude/channel/permission", handshake["capabilities"]["experimental"])
        self.assertNotIn("cage", json.dumps(handshake).lower())
        self.assertNotIn("guest", json.dumps(handshake).lower())
        socket = self.root / self.session / "rpc"
        self.assertTrue(socket.is_socket())
        self.assertEqual(stat.S_IMODE(socket.stat().st_mode), 0o600)
        event({"method": "notifications/initialized"})
        def rpc(**fields):
            result = subprocess.run([sys.executable, "-I", str(source), "rpc", "-"], env=self.env, text=True,
                                    input=json.dumps({"cli_id": self.session, "connection": self.session, **fields}), capture_output=True, check=True)
            return json.loads(result.stdout)
        request = str(uuid.uuid4())
        with concurrent.futures.ThreadPoolExecutor(2) as workers:
            replies = list(workers.map(lambda _: rpc(action="send", request_id=request, text="once"), range(2)))
        self.assertEqual(replies, [{"ok": True}, {"ok": True}])
        sent = json.loads(process.stdout.readline())
        self.assertEqual(sent["params"]["meta"]["request_id"], request)
        self.assertFalse(select.select([process.stdout], [], [], 0.1)[0])
        self.assertIn("error", rpc(action="send", request_id=request, text="changed"))
        event({"method": "notifications/claude/channel/permission_request", "params": {
            "request_id": "abcde", "tool_name": "Bash", "input_preview": '{"command":"echo test"}'}})
        path = self.root / self.session / "state.json"
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline and not channel.read_json(path).get("approvals"):
            time.sleep(0.01)
        approval = channel.read_json(path)["approvals"][0]
        self.assertIn("error", rpc(action="approve", approval_id=str(uuid.uuid4()), allow=True))
        self.assertEqual(rpc(action="approve", approval_id=approval["id"], allow=False), {"ok": True})
        self.assertEqual(json.loads(process.stdout.readline())["params"], {"request_id": "abcde", "behavior": "deny"})
        self.assertIn("error", rpc(action="approve", approval_id=approval["id"], allow=True))
        with channel.state_file(self.session) as state:
            state["resume_task"] = True
        self.assertEqual(rpc(action="resume"), {"ok":True})
        self.assertEqual(rpc(action="resume"), {"ok":True})
        resumed = json.loads(process.stdout.readline())
        self.assertEqual(resumed["params"]["meta"], {"resume":"true"})
        self.assertFalse(select.select([process.stdout], [], [], 0.1)[0], "resume was injected twice")


if __name__ == "__main__":
    unittest.main()
