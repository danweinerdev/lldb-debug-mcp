package dap

// LLDBDAPLaunchArgs holds the lldb-dap-specific arguments for a DAP launch
// request. These fields are serialized into the LaunchRequest.Arguments
// json.RawMessage.
type LLDBDAPLaunchArgs struct {
	Program           string            `json:"program"`
	Args              []string          `json:"args,omitempty"`
	Cwd               string            `json:"cwd,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	StopOnEntry       bool              `json:"stopOnEntry,omitempty"`
	InitCommands      []string          `json:"initCommands,omitempty"`
	PreRunCommands    []string          `json:"preRunCommands,omitempty"`
	PostRunCommands   []string          `json:"postRunCommands,omitempty"`
	StopCommands      []string          `json:"stopCommands,omitempty"`
	ExitCommands      []string          `json:"exitCommands,omitempty"`
	TerminateCommands []string          `json:"terminateCommands,omitempty"`
}

// LLDBDAPAttachArgs holds the lldb-dap-specific arguments for a DAP attach
// request. These fields are serialized into the AttachRequest.Arguments
// json.RawMessage.
type LLDBDAPAttachArgs struct {
	PID            int      `json:"pid,omitempty"`
	Program        string   `json:"program,omitempty"`
	WaitFor        bool     `json:"waitFor,omitempty"`
	StopOnEntry    bool     `json:"stopOnEntry,omitempty"`
	AttachCommands []string `json:"attachCommands,omitempty"`
	CoreFile       string   `json:"coreFile,omitempty"`
}
