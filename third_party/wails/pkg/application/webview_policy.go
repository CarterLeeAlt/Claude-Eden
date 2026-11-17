package application

// WindowsWebViewPolicy supplies native WebView2 policy decisions. Policies are
// installed before the first navigation and reapplied to replacement controllers.
// Navigation/new-window/download callbacks run on the UI thread and must return
// promptly. ScriptDialog runs off the UI thread with a native deferral held.
type WindowsWebViewPolicy struct {
	// DocumentStartScript is registered before navigation and on replacement
	// controllers. Initialized runs only after its registration has completed.
	DocumentStartScript  string
	AllowNavigation      func(WebViewNavigation) bool
	BlockNewWindow       func(string) bool
	CancelDownload       func(string) bool
	ScriptDialog         func(WebViewScriptDialog) (WebViewScriptDialogResult, error)
	Initialized          func(recovering bool) error
	InitializationFailed func(error)
	NavigationCompleted  func(WebViewNavigationResult)
	ProcessFailed        func(WebViewProcessFailure)
}

type WebViewNavigation struct {
	URL           string
	UserInitiated bool
	Redirected    bool
}
type WebViewNavigationResult struct {
	URL            string
	Success        bool
	WebErrorStatus uint32
}
type WebViewProcessFailure struct {
	Kind              uint32
	RecoveryAvailable bool
}
type WebViewScriptDialog struct {
	URL         string
	Kind        uint32
	Message     string
	DefaultText string
}
type WebViewScriptDialogResult struct {
	Accept bool
	Text   string
}
