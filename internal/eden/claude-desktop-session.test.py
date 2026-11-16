import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest


FIXTURE = r'''import json, os, signal, socket, sys, time
from pathlib import Path
name = Path(sys.argv[0]).name
root = Path(os.environ['SESSION_TEST_ROOT'])
mode = os.environ['SESSION_TEST_MODE']
runtime = root / 'runtime'
def event(action, **values):
    with (root / 'events').open('a') as stream:
        stream.write(json.dumps(dict(name=name, action=action, pid=os.getpid(), **values)) + '\n')
def live(service):
    try:
        os.kill(int((root / (service + '.pid')).read_text()), 0)
        return True
    except (FileNotFoundError, ProcessLookupError):
        return False
def stop(*_):
    if name == 'workbench-input-session':
        event('save', dependencies=all(live(service) for service in ['dbus-daemon', 'Xtigervnc', 'gnome-keyring-daemon', 'workbench-desktop-files']))
        time.sleep(.15)
    if name == 'fixture-application':
        event('save', dependencies=all(live(service) for service in ['dbus-daemon', 'ibus-daemon', 'workbench-native-input']))
    event('stop')
    sys.exit(0)
services = ['dbus-daemon', 'Xtigervnc', 'openbox', 'workbench-display-governor', 'gnome-keyring-daemon', 'xsettingsd', 'workbench-desktop-files', 'workbench-input-session', 'ibus-daemon', 'workbench-native-input', 'fixture-application']
if name in services:
    signal.signal(signal.SIGTERM, stop)
    (root / (name + '.pid')).write_text(str(os.getpid()))
    event('start')
    if name == 'dbus-daemon': print('unix:path=' + str(runtime / 'bus'), flush=True)
    if name == 'dbus-daemon' and mode == 'slow-input':
        connection = socket.socket(socket.AF_UNIX)
        connection.bind(str(Path(os.environ['HOME']) / '.cache/workbench-input/session-bus'))
    if name == 'ibus-daemon':
        time.sleep(6)
        connection = socket.socket(socket.AF_UNIX)
        connection.bind(os.environ['IBUS_ADDRESS'].removeprefix('unix:path='))
    if name == 'workbench-native-input':
        (Path(os.environ['HOME']) / '.cache/workbench-input/ready').touch()
    if name == 'fixture-application': (root / 'ready').touch()
    if name == 'Xtigervnc' and mode == 'display-failure': sys.exit(7)
    if name == 'gnome-keyring-daemon':
        key = sys.stdin.buffer.read()
        (root / 'key-received').write_bytes(key)
    if name == 'workbench-desktop-files':
        connection = socket.socket(socket.AF_UNIX)
        connection.bind(str(runtime / 'files.sock'))
    while True:
        if name == 'workbench-input-session' and (root / 'close-app').exists():
            event('normal-exit'); sys.exit(0)
        if name == 'openbox' and (root / 'fail-window-manager').exists():
            event('failure'); sys.exit(7)
        time.sleep(.02)
elif name == 'xdpyinfo':
    elapsed = time.time() - (root / 'Xtigervnc.pid').stat().st_mtime
    sys.exit(0 if mode != 'slow-display' or elapsed >= 11 else 1)
elif name == 'xprop':
    if '_NET_SUPPORTING_WM_CHECK' in sys.argv:
        print('_NET_SUPPORTING_WM_CHECK: 0x1234' if live('openbox') else '')
    else:
        print('_NET_CLIENT_LIST: 0x5678' if live('workbench-input-session') else '')
elif name == 'gdbus':
    if 'org.freedesktop.DBus.NameHasOwner' in sys.argv:
        print('(true,)' if (root / 'key-received').exists() else '(false,)')
    else:
        print('(<true>,)' if mode == 'wrong-key' else '(<false>,)')
elif name == 'systemd-notify':
    event('ready'); (root / 'ready').touch()
elif name == 'tigervncpasswd':
    sys.stdout.buffer.write(sys.stdin.buffer.read())
elif name == 'python3':
    if '-c' in sys.argv: sys.exit(0)
    sys.stdin.read()
    if mode == 'settings-error':
        print('display settings dependency unavailable', file=sys.stderr); sys.exit(1)
    sys.exit(0 if live('xsettingsd') else 75)
'''


@unittest.skipUnless(sys.platform == 'linux', 'the guest session requires Linux process and FIFO semantics')
class DesktopSessionTests(unittest.TestCase):
    def run_session(self, mode, action):
        with tempfile.TemporaryDirectory(prefix='desktop-lifecycle-') as directory:
            root = Path(directory)
            runtime, home, binary = root / 'runtime', root / 'home', root / 'bin'
            for folder in [runtime, home, binary]:
                folder.mkdir()
            commands = ['dbus-daemon', 'dbus-update-activation-environment', 'xauth', 'Xtigervnc',
                        'xdpyinfo', 'openbox', 'gnome-keyring-daemon', 'gdbus', 'tigervncpasswd',
                        'xprop', 'systemd-notify', 'workbench-display-governor', 'xsettingsd',
                        'workbench-desktop-files', 'workbench-input-session', 'python3']
            for name in commands:
                target = binary / name
                target.write_text('#!' + sys.executable + '\n' + FIXTURE)
                target.chmod(0o700)
            script = Path(__file__).with_name('claude-desktop-session.sh').read_text()
            script = script.replace('/run/workbench-desktop', str(runtime))
            script = script.replace('/etc/workbench', str(root / 'config'))
            script = script.replace('/usr/local/bin/', str(binary) + '/')
            script = script.replace('/usr/bin/python3', str(binary / 'python3'))
            session = root / 'session.sh'
            session.write_text(script)
            environment = dict(os.environ, HOME=str(home), PATH=str(binary) + ':' + os.environ['PATH'],
                               SESSION_TEST_ROOT=str(root), SESSION_TEST_MODE=mode, NOTIFY_SOCKET=str(root / 'notify'))
            with (root / 'output').open('w+') as output:
                child = subprocess.Popen(['bash', str(session)], env=environment, stdout=output,
                                         stderr=subprocess.STDOUT, start_new_session=True)
                try:
                    deadline = time.monotonic() + 35
                    key_sent = False
                    if action == 'cancel-start':
                        while not (root / 'Xtigervnc.pid').exists() and child.poll() is None:
                            self.assertLess(time.monotonic(), deadline)
                            time.sleep(.02)
                        child.send_signal(signal.SIGTERM)
                    else:
                        while not (root / 'ready').exists() and child.poll() is None:
                            self.assertLess(time.monotonic(), deadline, 'session readiness stalled')
                            if not key_sent and (runtime / 'unlock').exists():
                                try:
                                    descriptor = os.open(runtime / 'unlock', os.O_WRONLY | os.O_NONBLOCK)
                                except OSError as error:
                                    if error.errno != 6:
                                        raise
                                else:
                                    try:
                                        os.write(descriptor, b'session-fixture-key')
                                    finally:
                                        os.close(descriptor)
                                    key_sent = True
                            time.sleep(.02)
                        if mode not in ['display-failure', 'wrong-key', 'settings-error']:
                            self.assertTrue((root / 'ready').exists(), 'session exited before readiness')
                            if action == 'stop': child.send_signal(signal.SIGTERM)
                            elif action == 'close-app': (root / 'close-app').touch()
                            elif action == 'dependency-failure': (root / 'fail-window-manager').touch()
                    code = child.wait(timeout=10)
                    output.seek(0)
                    text = output.read()
                    records = [json.loads(line) for line in (root / 'events').read_text().splitlines()]
                    starts = [item['name'] for item in records if item['action'] == 'start']
                    stops = [item['name'] for item in records if item['action'] == 'stop']
                    failed = ['Xtigervnc'] if mode == 'display-failure' else ['openbox'] if action == 'dependency-failure' else []
                    ended = failed + (['workbench-input-session'] if action == 'close-app' else [])
                    dependency_order = ['dbus-daemon', 'Xtigervnc', 'openbox', 'workbench-display-governor',
                                        'gnome-keyring-daemon', 'xsettingsd', 'workbench-desktop-files', 'workbench-input-session']
                    self.assertEqual(stops, [name for name in reversed(dependency_order) if name in starts and name not in ended], text)
                    for record in records:
                        if record['action'] == 'save': self.assertTrue(record['dependencies'], text)
                        if record['action'] == 'start':
                            with self.assertRaises(ProcessLookupError): os.kill(record['pid'], 0)
                    if action == 'cancel-start': self.assertEqual(code, 143, text)
                    elif mode in ['display-failure', 'wrong-key', 'settings-error'] or action == 'dependency-failure': self.assertNotEqual(code, 0, text)
                    elif action == 'stop': self.assertEqual(code, 143, text)
                    else: self.assertEqual(code, 0, text)
                    if mode == 'wrong-key':
                        self.assertNotIn('workbench-input-session', starts)
                        self.assertIn('无法解锁已有密钥环', text)
                    if mode == 'settings-error':
                        self.assertNotIn('workbench-input-session', starts)
                        self.assertIn('display settings probe failed', text)
                finally:
                    try:
                        os.killpg(child.pid, signal.SIGTERM)
                    except ProcessLookupError:
                        pass
                    try:
                        child.wait(timeout=3)
                    finally:
                        try:
                            os.killpg(child.pid, signal.SIGKILL)
                        except ProcessLookupError:
                            pass
                        child.wait()

    def test_slow_display_remains_startable(self): self.run_session('slow-display', 'stop')
    def test_cancel_during_display_start(self): self.run_session('slow-display', 'cancel-start')
    def test_normal_app_exit_closes_dependencies(self): self.run_session('normal', 'close-app')
    def test_dependency_failure_closes_session(self): self.run_session('normal', 'dependency-failure')
    def test_display_failure_is_not_reported_ready(self): self.run_session('display-failure', 'stop')
    def test_wrong_key_preserves_keyring_and_closes_session(self): self.run_session('wrong-key', 'stop')
    def test_settings_probe_failure_closes_session(self): self.run_session('settings-error', 'stop')


@unittest.skipUnless(sys.platform == 'linux', 'the input session requires Linux process and FIFO semantics')
class InputSessionTests(unittest.TestCase):
    def test_slow_input_bus_and_cancelled_start_join_children(self):
        for cancel_start in [False, True]:
            with self.subTest(cancel_start=cancel_start), tempfile.TemporaryDirectory(prefix='input-lifecycle-') as directory:
                root = Path(directory)
                binary, home = root / 'bin', root / 'home'
                binary.mkdir(); home.mkdir()
                for name in ['dbus-daemon', 'ibus-daemon', 'workbench-native-input', 'fixture-application', 'python3']:
                    target = binary / name
                    target.write_text('#!' + sys.executable + '\n' + FIXTURE)
                    target.chmod(0o700)
                portal = root / 'portal.service'
                portal.write_text('[D-BUS Service]\n')
                script = Path(__file__).with_name('native-ime-session.sh').read_text()
                script = script.replace('/usr/local/bin/', str(binary) + '/')
                script = script.replace('/usr/bin/python3', str(binary / 'python3'))
                script = script.replace('/usr/share/dbus-1/services/org.freedesktop.portal.IBus.service', str(portal))
                session = root / 'session.sh'
                session.write_text(script)
                environment = dict(os.environ, HOME=str(home), PATH=str(binary) + ':' + os.environ['PATH'],
                                   SESSION_TEST_ROOT=str(root), SESSION_TEST_MODE='slow-input')
                environment.pop('DBUS_SESSION_BUS_ADDRESS', None)
                with (root / 'output').open('w+') as output:
                    child = subprocess.Popen(['bash', str(session), str(binary / 'fixture-application')], env=environment,
                                             stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
                    try:
                        marker = root / ('ibus-daemon.pid' if cancel_start else 'ready')
                        deadline = time.monotonic() + 20
                        while not marker.exists() and child.poll() is None:
                            self.assertLess(time.monotonic(), deadline)
                            time.sleep(.02)
                        output.flush()
                        self.assertTrue(marker.exists(), (root / 'output').read_text())
                        child.send_signal(signal.SIGTERM)
                        self.assertEqual(child.wait(timeout=10), 143, (root / 'output').read_text())
                        records = [json.loads(line) for line in (root / 'events').read_text().splitlines()]
                        starts = {item['name'] for item in records if item['action'] == 'start'}
                        stops = [item['name'] for item in records if item['action'] == 'stop']
                        self.assertEqual(stops, [name for name in ['fixture-application', 'workbench-native-input', 'ibus-daemon', 'dbus-daemon'] if name in starts])
                        if cancel_start: self.assertNotIn('fixture-application', starts)
                        for record in records:
                            if record['action'] == 'save': self.assertTrue(record['dependencies'])
                            if record['action'] == 'start':
                                with self.assertRaises(ProcessLookupError): os.kill(record['pid'], 0)
                    finally:
                        try: os.killpg(child.pid, signal.SIGTERM)
                        except ProcessLookupError: pass
                        try: child.wait(timeout=3)
                        finally:
                            try: os.killpg(child.pid, signal.SIGKILL)
                            except ProcessLookupError: pass
                            child.wait()


if __name__ == '__main__':
    unittest.main()
