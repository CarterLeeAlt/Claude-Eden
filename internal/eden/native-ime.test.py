import ast
from pathlib import Path
from types import SimpleNamespace
import unittest


class MainLoop:
    def __init__(self):
        self.timers = {}
        self.idles = []
        self.serial = 0

    def timeout_add(self, delay, callback, *args):
        self.serial += 1
        self.timers[self.serial] = (delay, callback, args)
        return self.serial

    def source_remove(self, identity):
        self.timers.pop(identity)

    def idle_add(self, callback, *args):
        self.idles.append((callback, args))

    def drain(self):
        while self.idles:
            callback, args = self.idles.pop(0)
            callback(*args)


class ProbeTests(unittest.TestCase):
    def setUp(self):
        self.loop = MainLoop()
        self.starts = []
        tree = ast.parse(Path(__file__).with_name('native-ime.py').read_text())
        engine = next(node for node in tree.body if isinstance(node, ast.ClassDef) and node.name == 'Engine')
        self.namespace = {
            'IBus': SimpleNamespace(Engine=object, Text=SimpleNamespace(new_from_string=lambda value: value), ModifierType=SimpleNamespace(RELEASE_MASK=1 << 30)),
            'GLib': self.loop, 'active': None, 'probe_key': 0xffd5,
            'subprocess': SimpleNamespace(DEVNULL=-3, Popen=lambda *args, **kwargs: self.starts.append(args)),
        }
        exec(compile(ast.Module(body=[engine], type_ignores=[]), 'native-ime.py', 'exec'), self.namespace)
        self.engine = self.namespace['Engine']()
        self.engine.context = 'field-a'
        self.engine.update_preedit_text = lambda *args: None
        self.namespace['active'] = self.engine
        self.results = []

    def request(self):
        self.engine.probe(lambda: self.results.append((self.engine.reachable, self.engine.rect)))

    def expire(self):
        identity = self.engine.probe_timer
        delay, callback, args = self.loop.timers.pop(identity)
        self.assertEqual(delay, 45)
        callback(*args)

    def test_probe_acknowledgement_completes_shared_requests_after_queued_caret(self):
        self.request()
        self.request()
        self.assertEqual(len(self.starts), 1)
        self.assertEqual(len(self.loop.timers), 1)
        self.engine.do_process_key_event(0xffd5, 0, 0)
        self.assertEqual(self.results, [])
        self.engine.do_set_cursor_location(20, 30, 1, 18)
        self.loop.drain()
        self.assertEqual(self.results, [(True, [20, 30, 1, 18])] * 2)
        self.assertEqual(self.loop.timers, {})

    def test_unanswered_probe_remains_unavailable(self):
        self.request()
        self.expire()
        self.assertEqual(self.results, [(False, None)])

    def test_unchanged_caret_completes_on_acknowledgement(self):
        self.engine.rect = [10, 20, 1, 18]
        self.request()
        self.engine.do_process_key_event(0xffd5, 0, 0)
        self.loop.drain()
        self.assertEqual(self.results, [(True, [10, 20, 1, 18])])
        self.assertEqual(self.loop.timers, {})

    def test_focus_loss_completes_pending_requests_without_a_stale_target(self):
        self.request()
        self.engine.do_process_key_event(0xffd5, 0, 0)
        self.engine.do_focus_out()
        self.assertEqual(self.results, [(False, None)])
        self.assertEqual(self.loop.timers, {})

    def test_changed_context_does_not_reuse_the_previous_field_probe(self):
        self.request()
        self.engine.context = 'field-b'
        self.engine.do_process_key_event(0xffd5, 0, 0)
        self.engine.do_set_cursor_location(2, 3, 1, 10)
        self.loop.drain()
        self.assertEqual(self.results, [(False, [2, 3, 1, 10])])

    def test_old_idle_callback_cannot_finish_a_new_probe(self):
        self.request()
        self.engine.do_process_key_event(0xffd5, 0, 0)
        self.engine.do_set_cursor_location(2, 3, 1, 10)
        self.engine.do_process_key_event(0xffd5, 0, 1)
        self.engine.do_set_cursor_location(2, 4, 1, 10)
        callback, args = self.loop.idles.pop(0)
        callback(*args)
        self.request()
        self.loop.drain()
        self.assertEqual(len(self.results), 1)
        self.assertTrue(self.engine.probing)
        self.expire()
        self.assertEqual(self.results[-1][0], False)

    def test_previous_key_release_cannot_acknowledge_a_new_probe(self):
        self.request()
        self.engine.do_process_key_event(0xffd5, 0, 0)
        self.loop.drain()
        self.assertEqual(len(self.results), 1)
        self.request()
        self.assertTrue(self.engine.do_process_key_event(0xffd5, 0, 1 << 30))
        self.loop.drain()
        self.assertEqual(len(self.results), 1)
        self.expire()
        self.assertFalse(self.results[-1][0])


if __name__ == '__main__':
    unittest.main()
