package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type clipboardCacheResult struct {
	Message    string   `json:"message"`
	Files      string   `json:"files"`
	CacheID    string   `json:"cache_id"`
	Paths      []string `json:"paths"`
	Selections []struct {
		CacheID string   `json:"cache_id"`
		Paths   []string `json:"paths"`
	} `json:"selections"`
}

type clipboardSync struct {
	backend    *Backend
	native     nativeClipboard
	lifetime   context.Context
	generation atomic.Uint64
	watch      sync.Once
	mu         sync.Mutex
	copies     map[string][]string
	pending    map[string]bool
	readers    map[string]selectedFiles
	loaded     bool
	collecting bool
	receiving  int
}

func (c *clipboardSync) retainReader(id string, paths []string, expires time.Time) {
	c.mu.Lock()
	c.readers[id] = selectedFiles{paths: paths, expires: expires}
	c.mu.Unlock()
}

func (c *clipboardSync) releaseReader(id string) {
	c.mu.Lock()
	delete(c.readers, id)
	c.mu.Unlock()
}

func (c *clipboardSync) promoteReader(id string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	reader, ok := c.readers[id]
	if !ok || !reader.expires.IsZero() && !reader.expires.After(now) {
		delete(c.readers, id)
		return false
	}
	reader.expires = time.Time{}
	c.readers[id] = reader
	return true
}

func (c *clipboardSync) snapshot() (clipboardSnapshot, error) {
	return application.InvokeSyncWithResultAndError(c.native.read)
}

// Snapshot acquisition and its temporary lease share the collector's release
// lock. No cache can be removed between reading CF_HDROP and pinning its paths.
func (c *clipboardSync) snapshotReader() (clipboardSnapshot, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot, err := c.snapshot()
	if err != nil || snapshot.paths == nil {
		return snapshot, "", err
	}
	id := uuid.NewString()
	c.readers[id] = selectedFiles{paths: snapshot.paths}
	return snapshot, id, nil
}

func (c *clipboardSync) revision() uint32 {
	c.watch.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-c.lifetime.Done():
					return
				case <-ticker.C:
					c.collect()
				}
			}
		}()
	})
	return c.native.revision()
}

func (c *clipboardSync) cache(ctx context.Context, operation, id string) (clipboardCacheResult, error) {
	request, _ := json.Marshal(struct {
		Action  string `json:"action"`
		CacheID string `json:"cache_id"`
	}{operation, id})
	var result clipboardCacheResult
	code, err := c.backend.JSON(ctx, 10*time.Second, 30<<20, &result, "clipboard-files", "--json", "cache", string(request))
	if err == nil && code != 0 {
		err = errors.New("clipboard_cache_operation_failed")
	}
	return result, err
}

func (c *clipboardSync) transfer(ctx context.Context, connection string, args ...string) (clipboardCacheResult, error) {
	var result clipboardCacheResult
	command := append([]string{"clipboard-files", "--json", "--connection", connectionID(connection)}, args...)
	code, err := c.backend.JSON(ctx, 30*time.Minute, 30<<20, &result, command...)
	if err == nil && code != 0 {
		if result.Message == "" {
			result.Message = "剪贴板文件传输未完成"
		}
		err = errors.New(result.Message)
	}
	return result, err
}

func pathsReference(selected, owned []string) bool {
	if len(owned) == 0 {
		return false
	}
	root := strings.ToLower(filepath.Clean(filepath.Dir(owned[0])))
	for _, path := range selected {
		path = strings.ToLower(filepath.Clean(path))
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) || strings.HasPrefix(root, strings.TrimRight(path, `\/`)+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (c *clipboardSync) releaseLocal(ctx context.Context, id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releaseLocalLocked(ctx, id)
}

func (c *clipboardSync) releaseLocalLocked(ctx context.Context, id string) {
	if _, err := c.cache(ctx, "release", id); err == nil {
		delete(c.copies, id)
	}
}

func (c *clipboardSync) collectUnused(ctx context.Context, readSnapshot func() (clipboardSnapshot, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// The clipboard snapshot, lease check and backend deletion are one critical
	// section. Readers and pending receives cannot enter between those steps.
	snapshot, err := readSnapshot()
	if err != nil {
		return
	}
	for _, id := range c.collectCandidatesLocked(snapshot.paths, time.Now()) {
		c.releaseLocalLocked(ctx, id)
	}
}

func (c *clipboardSync) cacheReferencedLocked(id string, selected []string, now time.Time) bool {
	paths, ok := c.copies[id]
	if !ok || c.receiving != 0 || c.pending[id] || pathsReference(selected, paths) {
		return true
	}
	for _, reader := range c.readers {
		if (reader.expires.IsZero() || reader.expires.After(now)) && pathsReference(reader.paths, paths) {
			return true
		}
	}
	return false
}

func (c *clipboardSync) collect() {
	c.mu.Lock()
	if c.collecting {
		c.mu.Unlock()
		return
	}
	c.collecting = true
	loaded := c.loaded
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.collecting = false; c.mu.Unlock() }()
	if !loaded {
		result, err := c.cache(c.lifetime, "list", "")
		if err != nil {
			return
		}
		c.mu.Lock()
		for _, selection := range result.Selections {
			c.copies[selection.CacheID] = selection.Paths
		}
		c.loaded = true
		c.mu.Unlock()
	}
	c.collectUnused(c.lifetime, c.snapshot)
}

func (c *clipboardSync) collectCandidates(selected []string, now time.Time) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.collectCandidatesLocked(selected, now)
}

func (c *clipboardSync) collectCandidatesLocked(selected []string, now time.Time) []string {
	var release []string
	for id, reader := range c.readers {
		if !reader.expires.IsZero() && !reader.expires.After(now) {
			delete(c.readers, id)
		}
	}
	for id := range c.copies {
		if !c.cacheReferencedLocked(id, selected, now) {
			release = append(release, id)
		}
	}
	return release
}

func (c *clipboardSync) read(ctx context.Context, request ConnectionRequest) (ClipboardContent, error) {
	_ = c.revision()
	generation := c.generation.Load()
	snapshot, reader, err := c.snapshotReader()
	if err != nil {
		return ClipboardContent{}, err
	}
	result := ClipboardContent{Revision: snapshot.revision}
	if snapshot.paths != nil {
		defer func() { c.releaseReader(reader); c.collect() }()
		response, err := c.transfer(ctx, request.ConnectionID, append([]string{"send"}, snapshot.paths...)...)
		if err != nil {
			return result, err
		}
		if _, err := uuid.Parse(response.Files); err != nil {
			return result, errors.New("后台返回了无效的剪贴板文件标识")
		}
		result.Files = response.Files
		if generation != c.generation.Load() || snapshot.revision != c.revision() {
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = c.transfer(cleanup, request.ConnectionID, "release", response.Files)
		}
	} else if snapshot.png != nil {
		result.PNG = base64.StdEncoding.EncodeToString(snapshot.png)
	} else {
		result.Text = &snapshot.text
	}
	return result, nil
}

func (c *clipboardSync) receive(ctx context.Context, connection, files string) (clipboardCacheResult, error) {
	c.mu.Lock()
	c.receiving++
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.receiving--; c.mu.Unlock() }()
	response, err := c.transfer(ctx, connection, "receive", files)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.loaded = false
	} else if response.CacheID != "" && len(response.Paths) != 0 {
		c.copies[response.CacheID] = response.Paths
		c.pending[response.CacheID] = true
	}
	return response, err
}

func (c *clipboardSync) write(ctx context.Context, request ClipboardWriteRequest) (ClipboardContent, error) {
	_ = c.revision()
	generation := c.generation.Load()
	var snapshot clipboardSnapshot
	cache := ""
	if request.Files != "" {
		if _, err := uuid.Parse(request.Files); err != nil {
			return ClipboardContent{}, errors.New("剪贴板文件标识无效")
		}
		response, err := c.receive(ctx, request.ConnectionID, request.Files)
		if err != nil {
			return ClipboardContent{}, err
		}
		if len(response.Paths) == 0 {
			return ClipboardContent{}, errors.New("后台没有返回剪贴板文件")
		}
		snapshot.paths, cache = response.Paths, response.CacheID
	} else if request.PNG != "" {
		if len(request.PNG) > ((maxPNG+2)/3)*4 {
			return ClipboardContent{}, errors.New("图片不能超过 20 MB")
		}
		var err error
		snapshot.png, err = base64.StdEncoding.DecodeString(request.PNG)
		if err != nil {
			return ClipboardContent{}, errors.New("PNG 图片编码无效")
		}
	} else if request.Text != nil {
		snapshot.text = *request.Text
	} else {
		return ClipboardContent{}, errors.New("剪贴板格式无效")
	}
	defer func() { c.mu.Lock(); delete(c.pending, cache); c.mu.Unlock(); c.collect() }()
	if generation != c.generation.Load() {
		if cache != "" {
			c.releaseLocal(c.lifetime, cache)
		}
		return ClipboardContent{Revision: c.revision(), Superseded: true}, nil
	}
	type outcome struct {
		revision   uint32
		superseded bool
	}
	value, err := application.InvokeSyncWithResultAndError(func() (outcome, error) {
		if generation != c.generation.Load() {
			return outcome{c.revision(), true}, nil
		}
		revision, superseded, err := c.native.write(snapshot, request.Revision)
		return outcome{revision, superseded}, err
	})
	if err != nil {
		return ClipboardContent{}, err
	}
	if cache != "" {
		if value.superseded {
			c.releaseLocal(c.lifetime, cache)
		} else {
			_, _ = c.cache(c.lifetime, "publish", cache)
		}
	}
	return ClipboardContent{Revision: value.revision, Superseded: value.superseded}, nil
}
