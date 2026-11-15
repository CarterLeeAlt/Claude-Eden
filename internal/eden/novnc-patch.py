# Patch noVNC's keyboard target and serialize clipboard transfers.
# The resulting rfb.js retains noVNC's MPL-2.0 license header.
from pathlib import Path
import sys

root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path('/usr/share/novnc')
path = root / 'core/rfb.js'
source = path.read_text()
original = path.with_name('rfb.upstream.js')
if 'workbenchPasteText(text)' in source:
    if not original.exists():
        raise SystemExit('Restore the package rfb.js before applying a new clipboard patch')
    source = original.read_text()
else:
    original.write_text(source)
ui = root / 'app/ui.js'
u = ui.read_text() if ui.exists() else ''
u = u.replace('"../core/rfb.js"', '"../core/rfb.js?revision=3"')
page = root / 'vnc.html'
h = (page.read_text() if page.exists() else '').replace('"./app/ui.js"', '"./app/ui.js?revision=3"')
h = h.replace('src="workbench-clipboard.js"', 'src="workbench-clipboard.js?revision=3"')

def replace_once(old, new):
    global source
    if source.count(old) != 1:
        raise SystemExit('Unsupported noVNC version: input/clipboard patch did not apply')
    source = source.replace(old, new)

# Keep noVNC's key tracking and focus lifecycle when the workbench supplies an
# editable host target for IME. Other viewers use the canvas.
replace_once('''    focus(options) {
        this._canvas.focus(options);
    }

    blur() {
        this._canvas.blur();
    }''', '''    workbenchSetKeyboardTarget(target) {
        this._keyboard.ungrab();
        this._workbenchKeyboardTarget = target || this._canvas;
        this._keyboard = new Keyboard(this._workbenchKeyboardTarget);
        this._keyboard.onkeyevent = this._handleKeyEvent.bind(this);
        if (this._rfbConnectionState === 'connected' && !this._viewOnly) {
            this._keyboard.grab();
        }
    }

    focus(options) {
        (this._workbenchKeyboardTarget || this._canvas).focus(options);
    }

    blur() {
        (this._workbenchKeyboardTarget || this._canvas).blur();
    }''')

replace_once('    clipboardPasteFrom(text) {', '''    // Serialize the clipboard notification, data transfer and paste.
    // A server fence confirms it has processed the provided clipboard data.
    _workbenchQueueInput(action) {
        const result = (this._workbenchInputChain || Promise.resolve()).then(action);
        this._workbenchInputChain = result;
        const clear = () => {
            if (this._workbenchInputChain === result) this._workbenchInputChain = null;
        };
        // A failed text transfer also cancels keys/clicks already queued behind
        // it, so an Enter cannot submit a form with the committed text missing.
        result.then(clear, clear);
        return result;
    }

    workbenchRunInput(action) {
        return this._workbenchQueueInput(() => this._workbenchRunInput(action));
    }

    async _workbenchRunInput(action) {
        if (this._rfbConnectionState !== 'connected' || this._viewOnly || !this._supportsFence)
            throw new Error('Input transport is not ready');
        // Flush preceding VNC clicks/keys before a browser text insertion.
        await new Promise((resolve, reject) => {
            const id = 'workbench-input-' + (this._workbenchInputSequence = (this._workbenchInputSequence || 0) + 1);
            const timer = setTimeout(() => {
                this._workbenchInputFence = null;
                reject(new Error('Input fence timed out'));
            }, 4000);
            this._workbenchInputFence = { id, timer, resolve };
            RFB.messages.clientFence(this._sock, (1 << 31) | 1, id);
        });
        if (this._rfbConnectionState !== 'connected') throw new Error('Input disconnected');
        return action((...key) => this._workbenchSendKey(...key));
    }

    workbenchPasteContent(content, pasteImage, valid, pasteText) {
        // Reserve the queue before asynchronous host clipboard/image decoding.
        // Keep a rejection handler attached even if an earlier input fails.
        const pending = Promise.resolve(content).then(value => ({value}), error => ({error}));
        return this._workbenchQueueInput(async () => {
            const {value, error} = await pending;
            if (error) throw error;
            if (!valid()) throw new Error('Input target changed');
            if (value.png || value.files || value.fileSelection) return this._workbenchRunInput(() => pasteImage(value));
            if (pasteText) return this._workbenchRunInput(() => pasteText(value.text || ''));
            return this._workbenchPasteText(value.text || '');
        });
    }

    workbenchPasteText(text) {
        return this._workbenchQueueInput(() => this._workbenchPasteText(text));
    }

    _workbenchPasteText(text) {
        return new Promise((resolve, reject) => {
            if (this._rfbConnectionState !== 'connected' || this._viewOnly ||
                !this._supportsFence ||
                !this._clipboardServerCapabilitiesActions[extendedClipboardActionNotify] ||
                !this._clipboardServerCapabilitiesActions[extendedClipboardActionProvide]) {
                reject(new Error('Clipboard transport is not ready'));
                return;
            }
            const id = 'workbench-paste-' + (this._workbenchPasteSequence = (this._workbenchPasteSequence || 0) + 1);
            const timer = setTimeout(() => {
                if (this._workbenchPasteJob?.id === id) this._workbenchPasteJob = null;
                reject(new Error('Remote application did not accept the paste'));
            }, 4000);
            this._workbenchPasteJob = {id, timer, resolve, phase: 'data'};
            this.clipboardPasteFrom(text);
        });
    }

    clipboardPasteFrom(text) {''')

replace_once('''    sendKey(keysym, code, down) {
        if (this._rfbConnectionState''', '''    sendKey(keysym, code, down) {
        if (this._workbenchInputChain) {
            this._workbenchQueueInput(() => this._workbenchSendKey(keysym, code, down));
            return;
        }
        this._workbenchSendKey(keysym, code, down);
    }

    _workbenchSendKey(keysym, code, down) {
        if (this._rfbConnectionState''')
replace_once('''            this.sendKey(keysym, code, true);
            this.sendKey(keysym, code, false);''', '''            this._workbenchSendKey(keysym, code, true);
            this._workbenchSendKey(keysym, code, false);''')
replace_once('''        const scancode = XtScancode[code];''', '''        // Track modifiers when they actually reach the wire, including keys
        // waiting behind text transfers. Host key state may already be newer.
        if (keysym >= 0xffe1 && keysym <= 0xffee) {
            this._workbenchModifiers ||= new Map();
            if (down) this._workbenchModifiers.set(code, keysym);
            else this._workbenchModifiers.delete(code);
        }
        const scancode = XtScancode[code];''')
replace_once('''    _sendMouse(x, y, mask) {''', '''    _sendMouse(x, y, mask) {
        if (this._workbenchInputChain) {
            this._workbenchQueueInput(() => this._workbenchSendMouse(x, y, mask));
            return;
        }
        this._workbenchSendMouse(x, y, mask);
    }

    _workbenchSendMouse(x, y, mask) {''')
provide = 'RFB.messages.extendedClipboardProvide(this._sock, [extendedClipboardFormatText], [this._clipboardText]);'
replace_once(provide, provide + '''
                        if (this._workbenchPasteJob?.phase === 'data') {
                            this._workbenchPasteJob.phase = 'fence';
                            RFB.messages.clientFence(this._sock, (1 << 31) | 1, this._workbenchPasteJob.id);
                        }''')
replace_once('''        if (!(flags & (1<<31))) {
            return this._fail("Unexpected fence response");
        }''', '''        if (!(flags & (1<<31))) {
            const input = this._workbenchInputFence;
            if (input && payload === input.id) {
                clearTimeout(input.timer);
                this._workbenchInputFence = null;
                input.resolve();
                return true;
            }
            if (payload.startsWith('workbench-input-')) return true;
            const job = this._workbenchPasteJob;
            if (job && job.phase === 'fence' && payload === job.id) {
                // Punctuation can commit while Shift is still physically held.
                // Paste with a plain Ctrl+V, then restore the wire modifiers;
                // queued host key releases will follow in their normal order.
                const modifiers = [...(this._workbenchModifiers || [])];
                for (const [code, sym] of modifiers) this._workbenchSendKey(sym, code, false);
                this._workbenchSendKey(0xffe3, 'ControlLeft', true);
                this._workbenchSendKey(0x76, 'KeyV', true);
                this._workbenchSendKey(0x76, 'KeyV', false);
                this._workbenchSendKey(0xffe3, 'ControlLeft', false);
                for (const [code, sym] of modifiers) this._workbenchSendKey(sym, code, true);
                job.phase = 'keys';
                RFB.messages.clientFence(this._sock, (1 << 31) | 1, job.id + '-keys');
                return true;
            }
            if (job && job.phase === 'keys' && payload === job.id + '-keys') {
                clearTimeout(job.timer);
                this._workbenchPasteJob = null;
                job.resolve();
                return true;
            }
            // A late acknowledgement after a timeout must not disconnect.
            if (payload.startsWith('workbench-paste-')) return true;
            return this._fail("Unexpected fence response");
        }''')
path.write_text(source)
if ui.exists(): ui.write_text(u)
if page.exists(): page.write_text(h)
