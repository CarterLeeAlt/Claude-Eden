"""Keep controller state outside the environment passed to external programs."""
import json
import os
from pathlib import Path
import runpy
import shlex
import sys

_KEYS = ("WORKBENCH_CLAUDE_SUBJECT",)
_CONTROL_KEYS = frozenset((*_KEYS, "WORKBENCH_CLAUDE_ACCOUNT_ID", "WORKBENCH_CLAUDE_LOGIN",
    "WORKBENCH_SESSION", "WORKBENCH_PROMPT", "WORKBENCH_TERMINAL_COMMAND",
    "WORKBENCH_LANGUAGE", "WORKBENCH_LOCALE", "WORKBENCH_TZ", "WORKBENCH_DISPLAY_SCALE",
    "WORKBENCH_WORKSPACE", "WORKBENCH_USER_HOME", "WORKBENCH_DESKTOP_SESSION",
    "WORKBENCH_NOTIFY_SOCKET", "WORKBENCH_DISPLAY_ACTIVITY", "WORKBENCH_DISPLAY_OUTPUT"))


def command(script, args=()):
    """Pass callback state as arguments, leaving the program environment clean."""
    context = globals().get("_controller_context", os.environ)
    internal = {key: context[key] for key in _KEYS if key in context}
    return [sys.executable, "-I", str(Path(__file__).absolute()), "--callback",
            json.dumps(internal, separators=(",", ":")), str(Path(script).absolute()), *args]


def command_text(script, args=()):
    return shlex.join(command(script, args))


def environment(values):
    return {key: value for key, value in values.items() if key.upper() not in _CONTROL_KEYS}


if __name__ == "__main__":
    if sys.argv[1] == "--callback":
        context = json.loads(sys.argv[2])
        if not isinstance(context, dict) or context.keys() - set(_KEYS) or any(not isinstance(value, str) for value in context.values()):
            raise ValueError("Invalid callback context")
        sys.argv = sys.argv[3:]
        runpy.run_path(sys.argv[0], run_name="__main__", init_globals={"_controller_context": context})
    else:
        os.execvpe(sys.argv[1], sys.argv[1:], environment(os.environ))
