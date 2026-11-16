package workbench

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sylvinus/agent-vm/internal/localsession"
	"github.com/sylvinus/agent-vm/internal/managedproxy"
)

type desktopLocal struct {
	config, profile, executable string
}

func NewDesktopLocalController(configPath, fullExecutable string) DesktopLocalController {
	return &desktopLocal{config: configPath, profile: filepath.Dir(configPath), executable: fullExecutable}
}

func (d *desktopLocal) available() bool {
	info, err := os.Stat(d.executable)
	return filepath.IsAbs(d.executable) && err == nil && info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode()&0111 != 0)
}

func (c *controllerClient) localControl(ctx context.Context, action string, input desktopLocalRequest) (desktopLocalReply, error) {
	var reply desktopLocalReply
	input.Token = c.record.Token
	status, err := c.callContext(ctx, http.MethodPost, "/api/desktop-local/"+action, input, &reply)
	if err != nil {
		return reply, err
	}
	switch status {
	case http.StatusOK:
		if action == "status" && (reply.OwnerID != localControllerID(c.record.Token) || reply.Identity.PID != c.record.PID || reply.Identity.Protocol != controlProtocol || !reply.Identity.matches(c.record.Controller) || !controllerInstanceID.MatchString(reply.InstanceID)) {
			return reply, errors.New("无法确认本机控制服务身份")
		}
		return reply, nil
	case http.StatusLocked:
		return reply, errDesktopLocalLocked
	case http.StatusNotFound:
		return reply, errors.New("当前本机后台版本不支持环境切换，请先在原工作台保存任务并完成后台更新")
	case http.StatusConflict:
		return reply, errors.New("本机仍有操作或状态已变化，请重新确认")
	default:
		return reply, errors.New("本机控制服务未接受此操作")
	}
}

// offlineStatus uses only existing private records. A live lock, orphan VM,
// or proxy without a reachable controller is unknown, never a released state.
func (d *desktopLocal) offlineStatus(ctx context.Context, available bool) DesktopLocalStatus {
	st := DesktopLocalStatus{Available: available, Message: "本机状态待确认，请返回本机检查已有后台"}
	free, err := controllerLockStatus(d.profile)
	if err != nil {
		st.Message = "无法检查本机控制锁，请检查配置目录的访问权限：" + err.Error()
		return st
	}
	if !free {
		st.ServiceRunning = true
		return st
	}
	running, err := controllerVMState(d.profile)
	if err != nil || running {
		st.Running = running
		return st
	}
	if managedproxy.ConfirmStopped(ctx, d.profile) != nil {
		return st
	}
	st.Known, st.Message = true, ""
	return st
}

func (d *desktopLocal) Status(ctx context.Context) (DesktopLocalStatus, error) {
	available := d.available()
	free, lockErr := controllerLockStatus(d.profile)
	if free || lockErr != nil {
		return d.offlineStatus(ctx, available), nil
	}
	c, err := openController(d.profile)
	if errors.Is(err, errControllerUnavailable) {
		return d.offlineStatus(ctx, available), nil
	}
	if err != nil {
		return DesktopLocalStatus{Available: available, Message: "本机后台记录无效，请检查本机配置"}, nil
	}
	defer c.close()
	view, err := c.localControl(ctx, "status", desktopLocalRequest{})
	if err != nil {
		if !processExists(c.record.PID) {
			return d.offlineStatus(ctx, available), nil
		}
		return DesktopLocalStatus{Available: available, ServiceRunning: true, Message: "无法确认已有本机后台的状态或版本"}, nil
	}
	view.Available = available
	return view.DesktopLocalStatus, nil
}

func (d *desktopLocal) Connect(ctx context.Context) (_ *localsession.Session, returned error) {
	st, err := d.Status(ctx)
	if err != nil {
		return nil, err
	}
	if !st.Available {
		return nil, errors.New("此安装缺少本机运行组件")
	}
	if !st.Known && st.ServiceRunning && st.ControllerID == "" {
		c, waitErr := waitControllerReady(ctx, d.profile)
		if c != nil {
			c.close()
		}
		if waitErr != nil && !errors.Is(waitErr, errControllerUnavailable) {
			return nil, waitErr
		}
		st, err = d.Status(ctx)
		if err != nil {
			return nil, err
		}
	}
	if !st.Known && st.ServiceRunning && st.ControllerID == "" {
		return nil, errors.New("无法确认已有本机后台的身份和状态，请检查已有服务后重试")
	}
	// Explicit local selection may reattach an orphan workload to its owner.
	// A live or inaccessible lock with no reachable service cannot be taken over.
	if !st.Known && !st.ServiceRunning && !controllerLockFree(d.profile) {
		return nil, errors.New(st.Message)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var launchID string
	var exited <-chan error
	if !st.ServiceRunning {
		launchID = randomToken()
		if exited, err = startControllerExecutable(d.config, d.executable, "EDEN_DESKTOP_START_ID="+launchID); err != nil {
			return nil, err
		}
		defer func() {
			if returned != nil {
				select {
				case <-exited:
					return
				default:
				}
				d.releaseFailedLaunch(launchID)
			}
		}()
	}
	bootstrap := ctx
	for {
		c, err := openController(d.profile)
		if err == nil {
			view, probeErr := c.localControl(bootstrap, "status", desktopLocalRequest{})
			if probeErr == nil {
				defer c.close()
				if st.ControllerID != "" && view.ControllerID != st.ControllerID {
					return nil, errors.New("本机后台已变化，请重新选择")
				}
				life := ctx
				if launchID != "" && view.LaunchID == launchID {
					life = context.WithValue(ctx, localLaunchKey{}, launchID)
				}
				session, err := d.localSession(life, bootstrap, c, view)
				return session, err
			}
			c.close()
			var networkError *url.Error
			if !errors.As(probeErr, &networkError) {
				return nil, probeErr
			}
		} else if !errors.Is(err, errControllerUnavailable) {
			return nil, err
		}
		if launchID == "" && controllerLockFree(d.profile) {
			return nil, errors.New("本机后台已退出，请重新连接")
		}
		select {
		case <-bootstrap.Done():
			return nil, bootstrap.Err()
		case err := <-exited:
			return nil, errors.Join(errors.New("本机后台在连接就绪前退出，请查看 control.log"), err)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

type localLaunchKey struct{}

// ReleaseUnused handles a prepared local view that the user never entered.
// A previous controller, including an idle one, belongs to its original owner.
func (d *desktopLocal) ReleaseUnused(ctx context.Context, session *localsession.Session) error {
	if session == nil || session.Context == nil {
		return nil
	}
	id, _ := session.Context.Value(localLaunchKey{}).(string)
	if id == "" {
		return nil
	}
	c, err := openController(d.profile)
	if errors.Is(err, errControllerUnavailable) {
		return nil
	}
	if err != nil {
		return err
	}
	defer c.close()
	view, err := c.localControl(ctx, "status", desktopLocalRequest{})
	if err != nil {
		return err
	}
	if view.LaunchID != id || !view.Known || view.Busy || view.Running || view.LiveTerminals > 0 {
		return nil
	}
	return d.Release(ctx, view.DesktopLocalStatus, false)
}

// The process started for a cancelled handshake may publish its record after
// cancellation. Only hand off a controller bearing this launcher's own nonce.
func (d *desktopLocal) releaseFailedLaunch(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), localReleaseTimeout)
	defer cancel()
	for ctx.Err() == nil {
		c, err := openController(d.profile)
		if err == nil {
			view, err := c.localControl(ctx, "status", desktopLocalRequest{})
			c.close()
			if err == nil {
				if view.LaunchID == id && view.Known && !view.Busy && !view.Running && view.LiveTerminals == 0 {
					_ = d.Release(ctx, view.DesktopLocalStatus, false)
				}
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func (d *desktopLocal) localSession(life, bootstrap context.Context, c *controllerClient, view desktopLocalReply) (*localsession.Session, error) {
	var launch launchResponse
	status, err := c.callContext(bootstrap, "POST", "/api/launch", map[string]string{"token": c.record.Token}, &launch)
	if err != nil || status != http.StatusOK || launch.Controller != view.Identity {
		return nil, errors.New("本机后台身份已变化，请重新选择")
	}
	u, err := url.Parse(launch.URL)
	if err != nil || u.Scheme+"://"+u.Host != c.origin || u.User != nil || u.Fragment == "" {
		return nil, errors.New("本机连接入口无效")
	}
	status, err = c.callContext(bootstrap, "POST", "/api/session", map[string]string{"token": u.Fragment}, nil)
	if err != nil || status != http.StatusOK {
		return nil, errors.New("本机会话认证失败")
	}
	var m ControllerManifest
	status, err = c.callContext(bootstrap, "GET", "/api/controller/manifest", nil, &m)
	if err != nil || status != http.StatusOK || m.InstanceID != view.InstanceID || m.APIProtocol != controllerAPIProtocol || m.Origin != c.origin || m.Platform != runtime.GOOS {
		return nil, errors.New("本机工作台版本或环境身份不符")
	}
	manifest := localsession.Manifest{APIProtocol: m.APIProtocol, InstanceID: m.InstanceID, Version: m.Version, Platform: m.Platform, Runtime: m.Runtime, Capabilities: m.Capabilities}
	c.client.Jar.SetCookies(u, []*http.Cookie{{
		Name:  cookieName + "_" + u.Port() + "-desktop",
		Value: trustedViewProof(c.record.Token), Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode,
	}})
	return localsession.New(life, c.origin, manifest, c.client.Jar)
}

func (d *desktopLocal) Release(ctx context.Context, expected DesktopLocalStatus, stop bool) error {
	if !expected.Known || !expected.ServiceRunning || expected.ControllerID == "" {
		return errors.New("请先确认本机控制服务身份")
	}
	c, err := openController(d.profile)
	if err != nil {
		return errors.New("本机控制服务已变化，请刷新后重试")
	}
	defer c.close()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	request := desktopLocalRequest{ControllerID: expected.ControllerID, Stop: stop, Running: expected.Running, Terminals: expected.LiveTerminals}
	reply, err := c.localControl(ctx, "release", request)
	if err != nil {
		return err
	}
	if reply.JobRunning {
		jobID := reply.JobID
		if jobID == "" || !stop {
			return errors.New("本机停止作业身份无效")
		}
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(150 * time.Millisecond):
			}
			view, err := c.localControl(ctx, "status", desktopLocalRequest{})
			if err != nil {
				return err
			}
			if view.JobID != jobID || view.JobFailed {
				return errors.New("本机停止作业已变化或失败，请回到本机确认状态")
			}
			if view.JobRunning {
				continue
			}
			if !view.Known || view.Busy || view.Running || view.LiveTerminals != 0 {
				return errors.New("本机工作尚未完全停止")
			}
			_, err = c.localControl(ctx, "release", desktopLocalRequest{ControllerID: view.ControllerID, JobID: jobID})
			if err != nil {
				return err
			}
			break
		}
	}
	if err = c.waitForExit(); err != nil {
		return err
	}
	for processExists(c.record.PID) {
		select {
		case <-ctx.Done():
			return errors.New("本机控制进程仍未退出")
		case <-time.After(75 * time.Millisecond):
		}
	}
	// Confirm that the released controller and its local VM have both stopped.
	if _, err := os.Lstat(filepath.Join(d.profile, "control.json")); !os.IsNotExist(err) || !controllerLockFree(d.profile) {
		return errors.New("本机控制服务状态已变化，请重新确认")
	}
	if running, err := controllerVMState(d.profile); err != nil || running || managedproxy.ConfirmStopped(ctx, d.profile) != nil {
		return errors.New("本机资源状态待确认")
	}
	return nil
}
