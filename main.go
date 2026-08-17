package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct cliproxy_buffer {
	uint8_t* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void* host_ctx, const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response);
typedef void (*cliproxy_host_free_buffer_fn)(void* ptr, size_t len);

typedef struct cliproxy_host_api {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_buffer_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char* method, uint8_t* request, size_t request_len, cliproxy_buffer* response);
typedef void (*cliproxy_plugin_free_buffer_fn)(void* ptr, size_t len);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct cliproxy_plugin_api {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_buffer_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

#ifdef _WIN32
#define CPA_PLUGIN_EXPORT __declspec(dllexport)
#else
#define CPA_PLUGIN_EXPORT
#endif

extern CPA_PLUGIN_EXPORT int antigravityPriorityPluginCall(char* method, uint8_t* request, size_t request_len, cliproxy_buffer* response);
extern CPA_PLUGIN_EXPORT void antigravityPriorityPluginFreeBuffer(void* ptr, size_t len);
extern CPA_PLUGIN_EXPORT void antigravityPriorityPluginShutdown(void);

static inline void set_antigravity_priority_plugin_api(cliproxy_plugin_api* plugin) {
	plugin->abi_version = 1;
	plugin->call = antigravityPriorityPluginCall;
	plugin->free_buffer = antigravityPriorityPluginFreeBuffer;
	plugin->shutdown = antigravityPriorityPluginShutdown;
}

static const cliproxy_host_api* stored_host;

static inline void store_antigravity_priority_host_api(const cliproxy_host_api* host) {
	stored_host = host;
}

static inline int call_antigravity_priority_host_api(const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	if (stored_host == NULL || stored_host->call == NULL) {
		return 1;
	}
	return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}

static inline void free_antigravity_priority_host_buffer(void* ptr, size_t len) {
	if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) {
		stored_host->free_buffer(ptr, len);
	}
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"unsafe"

	"antigravity-priority/internal/host"
	pluginruntime "antigravity-priority/internal/runtime"
)

var cpaRuntime = pluginruntime.New(pluginruntime.Options{Host: hostCallbackAdapter{}})

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return -1
	}
	C.store_antigravity_priority_host_api(host)
	cpaRuntime = pluginruntime.New(pluginruntime.Options{Host: hostCallbackAdapter{}})
	C.set_antigravity_priority_plugin_api(plugin)
	return 0
}

//export antigravityPriorityPluginCall
func antigravityPriorityPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response == nil {
		return -1
	}
	methodName := ""
	if method != nil {
		methodName = C.GoString(method)
	}
	requestBytes, ok := copyRequestBytes(request, requestLen)
	if !ok {
		return writeResponse(response, []byte(`{"ok":false,"error":{"code":"invalid_request","message":"request length is invalid","retryable":false}}`))
	}
	return writeResponse(response, cpaRuntime.Handle(context.Background(), methodName, requestBytes))
}

//export antigravityPriorityPluginFreeBuffer
func antigravityPriorityPluginFreeBuffer(ptr unsafe.Pointer, length C.size_t) {
	_ = length
	C.free(ptr)
}

//export antigravityPriorityPluginShutdown
func antigravityPriorityPluginShutdown() {
	_ = cpaRuntime.Shutdown(context.Background())
}

func copyRequestBytes(request *C.uint8_t, requestLen C.size_t) ([]byte, bool) {
	length := int(requestLen)
	if length < 0 || C.size_t(length) != requestLen {
		return nil, false
	}
	if length == 0 {
		return nil, true
	}
	if request == nil {
		return nil, false
	}
	requestSlice := unsafe.Slice((*byte)(unsafe.Pointer(request)), length)
	return append([]byte(nil), requestSlice...), true
}

func writeResponse(response *C.cliproxy_buffer, data []byte) C.int {
	if len(data) == 0 {
		response.ptr = nil
		response.len = 0
		return 0
	}
	ptr := C.malloc(C.size_t(len(data)))
	if ptr == nil {
		return -1
	}
	C.memcpy(ptr, unsafe.Pointer(&data[0]), C.size_t(len(data)))
	response.ptr = (*C.uint8_t)(ptr)
	response.len = C.size_t(len(data))
	return 0
}

type hostCallbackAdapter struct{}

func (hostCallbackAdapter) ListAuthFiles(ctx context.Context) ([]host.AuthFile, error) {
	var response struct {
		Files []host.AuthFile `json:"files"`
	}
	if err := callHost(ctx, "host.auth.list", map[string]any{}, &response); err != nil {
		return nil, err
	}
	return response.Files, nil
}

func (hostCallbackAdapter) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	var response host.AuthDocument
	if err := callHost(ctx, "host.auth.get", map[string]string{"auth_index": authIndex}, &response); err != nil {
		return host.AuthDocument{}, err
	}
	return response, nil
}

func (hostCallbackAdapter) GetRuntime(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
	var response host.RuntimeAuth
	if err := callHost(ctx, "host.auth.get_runtime", map[string]string{"auth_index": authIndex}, &response); err != nil {
		return host.RuntimeAuth{}, err
	}
	return response, nil
}

func (hostCallbackAdapter) SaveAuth(ctx context.Context, name string, doc json.RawMessage) error {
	var response json.RawMessage
	return callHost(ctx, "host.auth.save", map[string]any{"name": name, "json": doc}, &response)
}

func (hostCallbackAdapter) HTTPDo(ctx context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
	var response host.HTTPResponse
	if err := callHost(ctx, "host.http.do", request, &response); err != nil {
		return host.HTTPResponse{}, err
	}
	return response, nil
}

func callHost(ctx context.Context, method string, payload any, target any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal host callback %s: %w", method, err)
	}
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))
	var requestPtr *C.uint8_t
	if len(rawPayload) > 0 {
		cPayload := C.CBytes(rawPayload)
		if cPayload == nil {
			return fmt.Errorf("allocate host callback %s", method)
		}
		defer C.free(cPayload)
		requestPtr = (*C.uint8_t)(cPayload)
	}
	var response C.cliproxy_buffer
	callCode := C.call_antigravity_priority_host_api(cMethod, requestPtr, C.size_t(len(rawPayload)), &response)
	rawResponse := copyHostResponse(response)
	if response.ptr != nil {
		C.free_antigravity_priority_host_buffer(unsafe.Pointer(response.ptr), response.len)
	}
	if len(rawResponse) == 0 {
		return fmt.Errorf("host callback %s returned no response, code=%d", method, int(callCode))
	}
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rawResponse, &envelope); err != nil {
		return fmt.Errorf("decode host callback %s: %w", method, err)
	}
	if !envelope.OK {
		if envelope.Error != nil {
			return fmt.Errorf("host callback %s: %s: %s", method, envelope.Error.Code, envelope.Error.Message)
		}
		return fmt.Errorf("host callback %s failed", method)
	}
	if callCode != 0 {
		return fmt.Errorf("host callback %s returned code=%d", method, int(callCode))
	}
	if target == nil || len(envelope.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, target); err != nil {
		return fmt.Errorf("decode host callback result %s: %w", method, err)
	}
	return nil
}

func copyHostResponse(response C.cliproxy_buffer) []byte {
	if response.ptr == nil || response.len == 0 {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(response.ptr), C.int(response.len))
}

var _ host.HostCallbacks = hostCallbackAdapter{}
