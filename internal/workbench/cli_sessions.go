package workbench

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

var cliCCSProfile = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$")

type cliEpochKey struct{}

var errCLILocked = errors.New("Claude Eden 已锁定，请重新解锁后操作")

// Caller holds the account mutex. A request from an earlier unlock cannot gain
// authority from a later unlock of the same account.
func (s *Server) cliAuthorized(ctx context.Context) bool {
	epoch, _ := ctx.Value(cliEpochKey{}).(context.Context)
	return len(s.account.key) == 32 && epoch != nil && epoch == s.account.epoch && epoch.Err() == nil && ctx.Err() == nil
}

func cliLaunchArgs(launcher string, account claudeSelection) ([]string, error) {
	parts := strings.Fields(launcher)
	if len(parts) == 2 && parts[0] == "ccs" && cliCCSProfile.MatchString(parts[1]) {
		if account.ID != "" {
			return nil, errors.New("当前账号池需要直接使用 claude 启动命令")
		}
		return parts, nil
	}
	if len(parts) != 1 || parts[0] != "claude" {
		return nil, errors.New("会话历史支持 claude 或 ccs 配置名，请检查 Claude 启动命令")
	}
	if account.ID == "" && account.GuestID == "" {
		return parts, nil
	}
	boundDefault := account.ID == "" && account.GuestID == "default"
	if (!boundDefault && !validClaudeID(account.ID)) || !validClaudeGuestID(account.GuestID) || len(account.Subject) != 64 {
		return nil, errors.New("Claude 账号身份无效")
	}
	return []string{"env", "WORKBENCH_CLAUDE_SUBJECT=" + account.Subject, "/usr/local/bin/workbench-claude-account", "run", account.GuestID}, nil
}

func sharedCLILauncher(launcher string) bool {
	_, err := cliLaunchArgs(launcher, claudeSelection{})
	return err == nil
}
