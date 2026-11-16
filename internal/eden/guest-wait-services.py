import json
import argparse
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path


def wait_display(display):
    # systemd owns this ExecStartPost process and cancels it if X exits or stops.
    while True:
        try:
            result = subprocess.run(
                ['/usr/bin/xrandr', '--display', display, '--query'],
                stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True,
                timeout=2,
            )
            if result.returncode == 0:
                return
        except subprocess.TimeoutExpired:
            pass
        time.sleep(0.1)


def check_services(services):
    if not services:
        return
    try:
        result = subprocess.run(
            ['systemctl', 'show', '--property=Id,LoadState,ActiveState,SubState,Result', *services],
            capture_output=True, text=True, timeout=5, check=True,
        )
    except subprocess.TimeoutExpired:
        return
    for block in result.stdout.strip().split('\n\n'):
        state = dict(line.split('=', 1) for line in block.splitlines() if '=' in line)
        if (state.get('LoadState') != 'loaded' or state.get('ActiveState') in ('failed', 'inactive')
                or state.get('SubState') == 'auto-restart'):
            raise SystemExit(f"Workbench service {state.get('Id')} stopped before readiness: "
                              f"state={state.get('ActiveState')}, result={state.get('Result')}. Inspect the guest service logs.")


def wait_audio():
    while True:
        try:
            result = subprocess.run(
                ['pactl', '--server=unix:/run/workbench-audio/native', 'info'],
                stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=2,
            )
            if result.returncode == 0:
                return
        except subprocess.TimeoutExpired:
            pass
        check_services(('workbench-audio.service',))
        time.sleep(0.1)


def wait(config_path='/etc/workbench/browser-control.json', endpoints=None, services=()):
    endpoints = endpoints or (
        ('http://127.0.0.1:18080/healthz', False),
        ('http://127.0.0.1:16080/vnc.html', False),
        ('http://127.0.0.1:18500/health', True),
    )
    # The editor's service user only probes its unauthenticated health endpoint.
    token = json.loads(Path(config_path).read_text())['token'] if any(browser for _, browser in endpoints) else None
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    while True:
        try:
            for address, browser in endpoints:
                headers = {'Authorization': 'Bearer ' + token} if browser else {}
                request = urllib.request.Request(address, headers=headers)
                with opener.open(request, timeout=2) as response:
                    if browser and json.loads(response.read(1024)).get('ready') is not True:
                        break
            else:
                return
        except urllib.error.HTTPError as error:
            if error.code != 503:
                raise SystemExit(f'Workbench readiness request failed: HTTP {error.code} at {address}') from error
        except (OSError, urllib.error.URLError):
            pass
        check_services(services)
        time.sleep(1)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument('--editor', action='store_true')
    mode.add_argument('--display')
    mode.add_argument('--audio', action='store_true')
    mode.add_argument('--check', action='store_true')
    args = parser.parse_args()
    if args.audio:
        wait_audio()
    elif args.display:
        wait_display(args.display)
    else:
        services = ('workbench-editor.service',) if args.editor else (
            'workbench-editor.service', 'workbench-display.service',
            'workbench-window-manager.service', 'workbench-webvnc.service', 'workbench-browser.service',
        )
        if args.check:
            check_services(services)
        else:
            wait(endpoints=(('http://127.0.0.1:18080/healthz', False),) if args.editor else None, services=services)
