package desktop

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

type desktopInstance struct {
	handle   windows.Handle
	listener net.Listener
}

// The SID is part of both the identity and the ACL. A second profile for the
// same user owns another window; another user cannot send restore requests.
func acquireInstance(ctx context.Context, root string, smoke bool, restore func()) (*desktopInstance, bool, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, false, err
	}
	sid := user.User.Sid.String()
	key := instanceKey(sid, root)
	security, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, false, err
	}
	attributes := windows.SecurityAttributes{Length: uint32(24), SecurityDescriptor: security}
	// Use the real structure size on both supported compiler architectures.
	attributes.Length = uint32(securityAttributesSize())
	handle, err := windows.CreateMutex(&attributes, false, windows.StringToUTF16Ptr(`Local\Eden-`+key))
	pipe := `\\.\pipe\Eden-` + key
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			windows.CloseHandle(handle)
		}
		if smoke {
			return nil, true, nil
		}
		deadline, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		connection, err := winio.DialPipeContext(deadline, pipe)
		if err != nil {
			return nil, true, errors.New("Claude Eden 已在运行，但窗口暂时无法唤起。请从任务栏打开 Claude Eden。")
		}
		defer connection.Close()
		_ = connection.SetWriteDeadline(time.Now().Add(time.Second))
		_, err = connection.Write([]byte{1})
		return nil, true, err
	}
	if err != nil {
		return nil, false, err
	}
	listener, err := winio.ListenPipe(pipe, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + sid + ")", InputBufferSize: 1, OutputBufferSize: 1})
	if err != nil {
		windows.CloseHandle(handle)
		return nil, false, err
	}
	instance := &desktopInstance{handle, listener}
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
			var signal [1]byte
			_, err = io.ReadFull(connection, signal[:])
			_ = connection.Close()
			if err == nil && signal[0] == 1 {
				restore()
			}
		}
	}()
	return instance, false, nil
}

func (instance *desktopInstance) close() {
	if instance == nil {
		return
	}
	_ = instance.listener.Close()
	_ = windows.CloseHandle(instance.handle)
}
