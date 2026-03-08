package dap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	godap "github.com/google/go-dap"
)

// writeAndRead is a helper that encodes a DAP message via WriteProtocolMessage
// and decodes it back via ReadProtocolMessage, returning the decoded message.
func writeAndRead(t *testing.T, msg godap.Message) godap.Message {
	t.Helper()
	var buf bytes.Buffer
	if err := godap.WriteProtocolMessage(&buf, msg); err != nil {
		t.Fatalf("WriteProtocolMessage failed: %v", err)
	}
	reader := bufio.NewReader(&buf)
	decoded, err := godap.ReadProtocolMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolMessage failed: %v", err)
	}
	return decoded
}

func TestRoundTripInitializeRequest(t *testing.T) {
	req := &godap.InitializeRequest{}
	req.Seq = 1
	req.Type = "request"
	req.Command = "initialize"
	req.Arguments = godap.InitializeRequestArguments{
		ClientID:        "lldb-debug-mcp",
		ClientName:      "LLDB Debug MCP",
		AdapterID:       "lldb-dap",
		LinesStartAt1:   true,
		ColumnsStartAt1: true,
		PathFormat:      "path",
	}

	decoded := writeAndRead(t, req)

	got, ok := decoded.(*godap.InitializeRequest)
	if !ok {
		t.Fatalf("expected *InitializeRequest, got %T", decoded)
	}
	if got.Seq != 1 {
		t.Errorf("Seq: got %d, want 1", got.Seq)
	}
	if got.Command != "initialize" {
		t.Errorf("Command: got %q, want %q", got.Command, "initialize")
	}
	if got.Arguments.ClientID != "lldb-debug-mcp" {
		t.Errorf("ClientID: got %q, want %q", got.Arguments.ClientID, "lldb-debug-mcp")
	}
	if got.Arguments.AdapterID != "lldb-dap" {
		t.Errorf("AdapterID: got %q, want %q", got.Arguments.AdapterID, "lldb-dap")
	}
	if !got.Arguments.LinesStartAt1 {
		t.Error("LinesStartAt1: got false, want true")
	}
	if !got.Arguments.ColumnsStartAt1 {
		t.Error("ColumnsStartAt1: got false, want true")
	}
	if got.Arguments.PathFormat != "path" {
		t.Errorf("PathFormat: got %q, want %q", got.Arguments.PathFormat, "path")
	}
}

func TestRoundTripLaunchRequest(t *testing.T) {
	args := LLDBDAPLaunchArgs{
		Program:     "/path/to/exe",
		Args:        []string{"--flag", "value"},
		Cwd:         "/working/dir",
		Env:         map[string]string{"FOO": "bar"},
		StopOnEntry: true,
		InitCommands: []string{
			"settings set target.x86-disassembly-flavor intel",
		},
		PreRunCommands:    []string{"breakpoint set -n main"},
		PostRunCommands:   []string{"process status"},
		StopCommands:      []string{"bt"},
		ExitCommands:      []string{"quit"},
		TerminateCommands: []string{"process kill"},
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("json.Marshal(LLDBDAPLaunchArgs) failed: %v", err)
	}

	req := &godap.LaunchRequest{}
	req.Seq = 2
	req.Type = "request"
	req.Command = "launch"
	req.Arguments = argsJSON

	decoded := writeAndRead(t, req)

	got, ok := decoded.(*godap.LaunchRequest)
	if !ok {
		t.Fatalf("expected *LaunchRequest, got %T", decoded)
	}
	if got.Seq != 2 {
		t.Errorf("Seq: got %d, want 2", got.Seq)
	}
	if got.Command != "launch" {
		t.Errorf("Command: got %q, want %q", got.Command, "launch")
	}

	// Unmarshal the Arguments back into LLDBDAPLaunchArgs
	var decoded_args LLDBDAPLaunchArgs
	if err := json.Unmarshal(got.Arguments, &decoded_args); err != nil {
		t.Fatalf("json.Unmarshal(Arguments) failed: %v", err)
	}
	if decoded_args.Program != args.Program {
		t.Errorf("Program: got %q, want %q", decoded_args.Program, args.Program)
	}
	if len(decoded_args.Args) != len(args.Args) {
		t.Fatalf("Args length: got %d, want %d", len(decoded_args.Args), len(args.Args))
	}
	for i, v := range args.Args {
		if decoded_args.Args[i] != v {
			t.Errorf("Args[%d]: got %q, want %q", i, decoded_args.Args[i], v)
		}
	}
	if decoded_args.Cwd != args.Cwd {
		t.Errorf("Cwd: got %q, want %q", decoded_args.Cwd, args.Cwd)
	}
	if decoded_args.Env["FOO"] != "bar" {
		t.Errorf("Env[FOO]: got %q, want %q", decoded_args.Env["FOO"], "bar")
	}
	if !decoded_args.StopOnEntry {
		t.Error("StopOnEntry: got false, want true")
	}
	if len(decoded_args.InitCommands) != 1 || decoded_args.InitCommands[0] != args.InitCommands[0] {
		t.Errorf("InitCommands: got %v, want %v", decoded_args.InitCommands, args.InitCommands)
	}
	if len(decoded_args.PreRunCommands) != 1 || decoded_args.PreRunCommands[0] != args.PreRunCommands[0] {
		t.Errorf("PreRunCommands: got %v, want %v", decoded_args.PreRunCommands, args.PreRunCommands)
	}
	if len(decoded_args.PostRunCommands) != 1 || decoded_args.PostRunCommands[0] != args.PostRunCommands[0] {
		t.Errorf("PostRunCommands: got %v, want %v", decoded_args.PostRunCommands, args.PostRunCommands)
	}
	if len(decoded_args.StopCommands) != 1 || decoded_args.StopCommands[0] != args.StopCommands[0] {
		t.Errorf("StopCommands: got %v, want %v", decoded_args.StopCommands, args.StopCommands)
	}
	if len(decoded_args.ExitCommands) != 1 || decoded_args.ExitCommands[0] != args.ExitCommands[0] {
		t.Errorf("ExitCommands: got %v, want %v", decoded_args.ExitCommands, args.ExitCommands)
	}
	if len(decoded_args.TerminateCommands) != 1 || decoded_args.TerminateCommands[0] != args.TerminateCommands[0] {
		t.Errorf("TerminateCommands: got %v, want %v", decoded_args.TerminateCommands, args.TerminateCommands)
	}
}

func TestRoundTripContinueRequest(t *testing.T) {
	req := &godap.ContinueRequest{}
	req.Seq = 3
	req.Type = "request"
	req.Command = "continue"
	req.Arguments = godap.ContinueArguments{
		ThreadId:     42,
		SingleThread: true,
	}

	decoded := writeAndRead(t, req)

	got, ok := decoded.(*godap.ContinueRequest)
	if !ok {
		t.Fatalf("expected *ContinueRequest, got %T", decoded)
	}
	if got.Seq != 3 {
		t.Errorf("Seq: got %d, want 3", got.Seq)
	}
	if got.Command != "continue" {
		t.Errorf("Command: got %q, want %q", got.Command, "continue")
	}
	if got.Arguments.ThreadId != 42 {
		t.Errorf("ThreadId: got %d, want 42", got.Arguments.ThreadId)
	}
	if !got.Arguments.SingleThread {
		t.Error("SingleThread: got false, want true")
	}
}

func TestRoundTripEvaluateRequest(t *testing.T) {
	req := &godap.EvaluateRequest{}
	req.Seq = 4
	req.Type = "request"
	req.Command = "evaluate"
	req.Arguments = godap.EvaluateArguments{
		Expression: "x + y",
		FrameId:    7,
		Context:    "watch",
	}

	decoded := writeAndRead(t, req)

	got, ok := decoded.(*godap.EvaluateRequest)
	if !ok {
		t.Fatalf("expected *EvaluateRequest, got %T", decoded)
	}
	if got.Seq != 4 {
		t.Errorf("Seq: got %d, want 4", got.Seq)
	}
	if got.Command != "evaluate" {
		t.Errorf("Command: got %q, want %q", got.Command, "evaluate")
	}
	if got.Arguments.Expression != "x + y" {
		t.Errorf("Expression: got %q, want %q", got.Arguments.Expression, "x + y")
	}
	if got.Arguments.FrameId != 7 {
		t.Errorf("FrameId: got %d, want 7", got.Arguments.FrameId)
	}
	if got.Arguments.Context != "watch" {
		t.Errorf("Context: got %q, want %q", got.Arguments.Context, "watch")
	}
}

func TestLaunchArgsOmitsEmptyFields(t *testing.T) {
	// Only set the required program field; optional fields should be omitted
	// from the JSON output.
	args := LLDBDAPLaunchArgs{
		Program: "/path/to/exe",
	}

	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)

	// The program field must be present.
	if !strings.Contains(jsonStr, `"program"`) {
		t.Error("expected 'program' field in JSON output")
	}

	// Optional fields should not appear when zero-valued.
	for _, field := range []string{
		"args", "cwd", "env", "stopOnEntry",
		"initCommands", "preRunCommands", "postRunCommands",
		"stopCommands", "exitCommands", "terminateCommands",
	} {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("expected field %q to be omitted from JSON, got: %s", field, jsonStr)
		}
	}
}

func TestAttachArgsRoundTrip(t *testing.T) {
	args := LLDBDAPAttachArgs{
		PID:            1234,
		Program:        "/path/to/exe",
		WaitFor:        true,
		StopOnEntry:    true,
		AttachCommands: []string{"process attach --pid 1234"},
		CoreFile:       "/path/to/core",
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("json.Marshal(LLDBDAPAttachArgs) failed: %v", err)
	}

	req := &godap.AttachRequest{}
	req.Seq = 5
	req.Type = "request"
	req.Command = "attach"
	req.Arguments = argsJSON

	decoded := writeAndRead(t, req)

	got, ok := decoded.(*godap.AttachRequest)
	if !ok {
		t.Fatalf("expected *AttachRequest, got %T", decoded)
	}

	var decodedArgs LLDBDAPAttachArgs
	if err := json.Unmarshal(got.Arguments, &decodedArgs); err != nil {
		t.Fatalf("json.Unmarshal(Arguments) failed: %v", err)
	}
	if decodedArgs.PID != args.PID {
		t.Errorf("PID: got %d, want %d", decodedArgs.PID, args.PID)
	}
	if decodedArgs.Program != args.Program {
		t.Errorf("Program: got %q, want %q", decodedArgs.Program, args.Program)
	}
	if !decodedArgs.WaitFor {
		t.Error("WaitFor: got false, want true")
	}
	if !decodedArgs.StopOnEntry {
		t.Error("StopOnEntry: got false, want true")
	}
	if len(decodedArgs.AttachCommands) != 1 || decodedArgs.AttachCommands[0] != args.AttachCommands[0] {
		t.Errorf("AttachCommands: got %v, want %v", decodedArgs.AttachCommands, args.AttachCommands)
	}
	if decodedArgs.CoreFile != args.CoreFile {
		t.Errorf("CoreFile: got %q, want %q", decodedArgs.CoreFile, args.CoreFile)
	}
}

func TestMalformedInputTruncatedMessage(t *testing.T) {
	// A valid header but truncated content body.
	input := "Content-Length: 100\r\n\r\n{\"seq\":1"
	reader := bufio.NewReader(strings.NewReader(input))
	_, err := godap.ReadProtocolMessage(reader)
	if err == nil {
		t.Fatal("expected error for truncated message, got nil")
	}
}

func TestMalformedInputBadJSON(t *testing.T) {
	// Valid header with content length matching the bad JSON body.
	badJSON := "{not valid json!!}"
	header := "Content-Length: " + string(rune('0'+len(badJSON)/10)) + string(rune('0'+len(badJSON)%10)) + "\r\n\r\n"
	input := header + badJSON
	reader := bufio.NewReader(strings.NewReader(input))
	_, err := godap.ReadProtocolMessage(reader)
	if err == nil {
		t.Fatal("expected error for bad JSON, got nil")
	}
}

func TestMalformedInputNoHeader(t *testing.T) {
	// Content without the Content-Length header.
	input := `{"seq":1,"type":"request","command":"initialize"}`
	reader := bufio.NewReader(strings.NewReader(input))
	_, err := godap.ReadProtocolMessage(reader)
	if err == nil {
		t.Fatal("expected error for missing header, got nil")
	}
}

func TestMalformedInputEmptyReader(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	_, err := godap.ReadProtocolMessage(reader)
	if err == nil {
		t.Fatal("expected error for empty reader, got nil")
	}
}
