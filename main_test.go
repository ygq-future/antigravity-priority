package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

	// 1. Verify init with nil
	ret, _, _ := initProc.Call(0, 0)
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
	methodBytes := append([]byte("plugin.register"), 0)
	reqJSON := []byte("{\"config_yaml\":\"enabled: true\\nantigravity_model_group: gemini\\n\"}")

	var respBuf cliproxyBuffer
	ret, _, _ = callProc.Call(
		uintptr(unsafe.Pointer(&methodBytes[0])),
		uintptr(unsafe.Pointer(&reqJSON[0])),
		uintptr(len(reqJSON)),
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

	// 4. Free buffer
	freeProc.Call(uintptr(unsafe.Pointer(respBuf.ptr)), respBuf.len)

	// 5. Shutdown
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
