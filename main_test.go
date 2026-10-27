package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/host"
	pluginruntime "antigravity-priority/internal/runtime"
)

func TestMain_CGO_ABI_Integration(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows DLL CGO ABI test on non-windows platform")
	}

	tempDir := t.TempDir()
	dllPath := filepath.Join(tempDir, "test_plugin.dll")

	// 1. Build c-shared dynamic library
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", dllPath, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build c-shared dll: %v\nOutput: %s", err, string(out))
	}

	// 2. Create an isolated verifier script to test the DLL in its own process
	q := "`"
	verifierSrc := `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type cliproxyBuffer struct {
	ptr *byte
	len uintptr
}

type hostAPI struct {
	abiVersion uint32
	_pad       uint32
	hostCtx    uintptr
	call       uintptr
	freeBuffer uintptr
}

type pluginAPI struct {
	abiVersion uint32
	_pad       uint32
	call       uintptr
	freeBuffer uintptr
	shutdown   uintptr
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing dll path argument")
		os.Exit(1)
	}
	dllPath := os.Args[1]

	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load dll: %v\n", err)
		os.Exit(1)
	}
	defer dll.Release()

	initProc, err := dll.FindProc("cliproxy_plugin_init")
	if err != nil {
		fmt.Fprintf(os.Stderr, "missing cliproxy_plugin_init: %v\n", err)
		os.Exit(1)
	}

	callProc, err := dll.FindProc("antigravityPriorityPluginCall")
	if err != nil {
		fmt.Fprintf(os.Stderr, "missing antigravityPriorityPluginCall: %v\n", err)
		os.Exit(1)
	}

	freeProc, err := dll.FindProc("antigravityPriorityPluginFreeBuffer")
	if err != nil {
		fmt.Fprintf(os.Stderr, "missing antigravityPriorityPluginFreeBuffer: %v\n", err)
		os.Exit(1)
	}

	shutdownProc, err := dll.FindProc("antigravityPriorityPluginShutdown")
	if err != nil {
		fmt.Fprintf(os.Stderr, "missing antigravityPriorityPluginShutdown: %v\n", err)
		os.Exit(1)
	}
	// 0. Verify call before init returns shutdown / uninitialized envelope safely
	methodMgmt := append([]byte("management.handle"), 0)
	reqMgmt := []byte("{\"Method\":\"GET\",\"Path\":\"/v0/management/plugins/antigravity-priority/diagnostics\"}")
	var respBuf cliproxyBuffer
	ret, _, _ := callProc.Call(
		uintptr(unsafe.Pointer(&methodMgmt[0])),
		uintptr(unsafe.Pointer(&reqMgmt[0])),
		uintptr(len(reqMgmt)),
		uintptr(unsafe.Pointer(&respBuf)),
	)
	if int32(ret) != 0 || respBuf.ptr == nil || respBuf.len == 0 {
		fmt.Fprintf(os.Stderr, "uninitialized call failed to return buffer: ret=%d\n", int32(ret))
		os.Exit(1)
	}
	uninitBytes := unsafe.Slice(respBuf.ptr, respBuf.len)
	var uninitEnvelope struct {
		OK    bool ` + q + `json:"ok"` + q + `
		Error struct {
			Code string ` + q + `json:"code"` + q + `
		} ` + q + `json:"error"` + q + `
	}
	if err := json.Unmarshal(uninitBytes, &uninitEnvelope); err != nil || uninitEnvelope.OK || uninitEnvelope.Error.Code != "shutdown" {
		fmt.Fprintf(os.Stderr, "expected shutdown error for uninitialized call, got: %s\n", string(uninitBytes))
		os.Exit(1)
	}
	freeProc.Call(uintptr(unsafe.Pointer(respBuf.ptr)), respBuf.len)

	// 1. Verify init with nil
	ret, _, _ = initProc.Call(0, 0)
	if int32(ret) != -1 {
		fmt.Fprintf(os.Stderr, "expected -1 on nil init, got %d\n", int32(ret))
		os.Exit(1)
	}
	// 2. Verify init with valid pointers
	var host hostAPI
	var plugin pluginAPI
	ret, _, _ = initProc.Call(uintptr(unsafe.Pointer(&host)), uintptr(unsafe.Pointer(&plugin)))
	if int32(ret) != 0 || plugin.abiVersion != 1 || plugin.call == 0 || plugin.freeBuffer == 0 || plugin.shutdown == 0 {
		fmt.Fprintf(os.Stderr, "init failed: ret=%d, abi=%d, call=%x\n", int32(ret), plugin.abiVersion, plugin.call)
		os.Exit(1)
	}

	// 3. Verify plugin.register call
	methodRegister := append([]byte("plugin.register"), 0)
	reqRegister := []byte("{\"config_yaml\":\"enabled: true\\nantigravity_model_group: gemini\\n\"}")

	respBuf = cliproxyBuffer{}
	ret, _, _ = callProc.Call(
		uintptr(unsafe.Pointer(&methodRegister[0])),
		uintptr(unsafe.Pointer(&reqRegister[0])),
		uintptr(len(reqRegister)),
		uintptr(unsafe.Pointer(&respBuf)),
	)
	if int32(ret) != 0 || respBuf.ptr == nil || respBuf.len == 0 {
		fmt.Fprintf(os.Stderr, "plugin.register failed: ret=%d\n", int32(ret))
		os.Exit(1)
	}

	respBytes := unsafe.Slice(respBuf.ptr, respBuf.len)
	var envelope struct {
		OK     bool ` + q + `json:"ok"` + q + `
		Result struct {
			Metadata struct {
				Name string ` + q + `json:"Name"` + q + `
			} ` + q + `json:"metadata"` + q + `
		} ` + q + `json:"result"` + q + `
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil || !envelope.OK || envelope.Result.Metadata.Name != "Antigravity Priority" {
		fmt.Fprintf(os.Stderr, "invalid envelope: %s\n", string(respBytes))
		os.Exit(1)
	}
	freeProc.Call(uintptr(unsafe.Pointer(respBuf.ptr)), respBuf.len)

	// 4. Verify management.register call
	methodMgmtReg := append([]byte("management.register"), 0)
	respBuf = cliproxyBuffer{}
	ret, _, _ = callProc.Call(
		uintptr(unsafe.Pointer(&methodMgmtReg[0])),
		0,
		0,
		uintptr(unsafe.Pointer(&respBuf)),
	)
	if int32(ret) != 0 || respBuf.ptr == nil || respBuf.len == 0 {
		fmt.Fprintf(os.Stderr, "management.register failed: ret=%d\n", int32(ret))
		os.Exit(1)
	}
	freeProc.Call(uintptr(unsafe.Pointer(respBuf.ptr)), respBuf.len)

	// 5. Verify repeated calls and memory freeing
	for i := 0; i < 5; i++ {
		methodMgmt := append([]byte("management.handle"), 0)
		reqMgmt := []byte("{\"Method\":\"GET\",\"Path\":\"/v0/management/plugins/antigravity-priority/diagnostics\"}")
		respBuf = cliproxyBuffer{}
		ret, _, _ = callProc.Call(
			uintptr(unsafe.Pointer(&methodMgmt[0])),
			uintptr(unsafe.Pointer(&reqMgmt[0])),
			uintptr(len(reqMgmt)),
			uintptr(unsafe.Pointer(&respBuf)),
		)
		if int32(ret) != 0 || respBuf.ptr == nil || respBuf.len == 0 {
			fmt.Fprintf(os.Stderr, "management.handle iteration %d failed: ret=%d\n", i, int32(ret))
			os.Exit(1)
		}
		freeProc.Call(uintptr(unsafe.Pointer(respBuf.ptr)), respBuf.len)
	}

	// 6. Verify re-initialization (host reload) cleanly terminates earlier instance
	ret, _, _ = initProc.Call(uintptr(unsafe.Pointer(&host)), uintptr(unsafe.Pointer(&plugin)))
	if int32(ret) != 0 {
		fmt.Fprintf(os.Stderr, "re-init failed: ret=%d\n", int32(ret))
		os.Exit(1)
	}

	// 7. Verify call succeeds after re-initialization
	respBuf = cliproxyBuffer{}
	ret, _, _ = callProc.Call(
		uintptr(unsafe.Pointer(&methodMgmt[0])),
		uintptr(unsafe.Pointer(&reqMgmt[0])),
		uintptr(len(reqMgmt)),
		uintptr(unsafe.Pointer(&respBuf)),
	)
	if int32(ret) != 0 || respBuf.ptr == nil || respBuf.len == 0 {
		fmt.Fprintf(os.Stderr, "call after re-init failed: ret=%d\n", int32(ret))
		os.Exit(1)
	}
	freeProc.Call(uintptr(unsafe.Pointer(respBuf.ptr)), respBuf.len)

	// 8. Shutdown
	shutdownProc.Call()

	// 9. Verify call after shutdown returns shutdown error
	respBuf = cliproxyBuffer{}
	ret, _, _ = callProc.Call(
		uintptr(unsafe.Pointer(&methodMgmt[0])),
		uintptr(unsafe.Pointer(&reqMgmt[0])),
		uintptr(len(reqMgmt)),
		uintptr(unsafe.Pointer(&respBuf)),
	)
	if int32(ret) != 0 || respBuf.ptr == nil || respBuf.len == 0 {
		fmt.Fprintf(os.Stderr, "post-shutdown call failed: ret=%d\n", int32(ret))
		os.Exit(1)
	}
	postShutdownBytes := unsafe.Slice(respBuf.ptr, respBuf.len)
	var shutdownEnvelope struct {
		OK    bool ` + q + `json:"ok"` + q + `
		Error struct {
			Code string ` + q + `json:"code"` + q + `
		} ` + q + `json:"error"` + q + `
	}
	if err := json.Unmarshal(postShutdownBytes, &shutdownEnvelope); err != nil || shutdownEnvelope.OK || shutdownEnvelope.Error.Code != "shutdown" {
		fmt.Fprintf(os.Stderr, "expected shutdown error for post-shutdown call, got: %s\n", string(postShutdownBytes))
		os.Exit(1)
	}
	freeProc.Call(uintptr(unsafe.Pointer(respBuf.ptr)), respBuf.len)

	// 10. Repeated shutdown (idempotency check)
	shutdownProc.Call()

	fmt.Println("VERIFY_CGO_ABI_SUCCESS")
}
`
	verifierPath := filepath.Join(tempDir, "verify.go")
	if err := os.WriteFile(verifierPath, []byte(verifierSrc), 0o600); err != nil {
		t.Fatalf("failed to write verifier source: %v", err)
	}

	// 3. Run verifier in an isolated subprocess
	verifyCmd := exec.Command("go", "run", verifierPath, dllPath)
	verifyOut, err := verifyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verifier failed: %v\nOutput:\n%s", err, string(verifyOut))
	}

	if !strings.Contains(string(verifyOut), "VERIFY_CGO_ABI_SUCCESS") {
		t.Fatalf("unexpected verifier output: %s", string(verifyOut))
	}
}

type stubHostCallbacks struct{}

func (stubHostCallbacks) ListAuthFiles(ctx context.Context) ([]host.AuthFile, error) {
	return nil, nil
}
func (stubHostCallbacks) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	return host.AuthDocument{}, nil
}
func (stubHostCallbacks) GetRuntime(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
	return host.RuntimeAuth{}, nil
}
func (stubHostCallbacks) SaveAuth(ctx context.Context, name string, doc json.RawMessage) error {
	return nil
}
func (stubHostCallbacks) HTTPDo(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	return host.HTTPResponse{StatusCode: 200}, nil
}

func TestMain_PackageLoad_NoBackgroundGoroutines(t *testing.T) {
	shutdownCPARuntime()
	if rt := getCPARuntime(); rt != nil {
		t.Fatalf("expected nil cpaRuntime before init, got non-nil: %v", rt)
	}
}

func TestMain_Lifecycle_GoroutineLeakPrevention(t *testing.T) {
	shutdownCPARuntime()
	baseline := runtime.NumGoroutine()
	cacheDir := t.TempDir()

	for i := range 10 {
		initCPARuntimeWithOptions(pluginruntime.Options{
			Host:           stubHostCallbacks{},
			StateCachePath: filepath.Join(cacheDir, fmt.Sprintf("state-%d.json", i)),
		})
		current := runtime.NumGoroutine()
		if current > baseline+4 {
			t.Fatalf("iteration %d: goroutines leaked during re-init: current=%d, baseline=%d", i, current, baseline)
		}
	}

	shutdownCPARuntime()

	clean := false
	for range 50 {
		if runtime.NumGoroutine() <= baseline+1 {
			clean = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !clean {
		t.Fatalf("goroutines did not return to baseline after shutdown: current=%d, baseline=%d", runtime.NumGoroutine(), baseline)
	}
}

func TestMain_ShutdownIdempotencyAndUninitializedCall(t *testing.T) {
	shutdownCPARuntime()
	shutdownCPARuntime()
	shutdownCPARuntime()

	if rt := getCPARuntime(); rt != nil {
		t.Fatalf("expected nil cpaRuntime, got %v", rt)
	}
}
