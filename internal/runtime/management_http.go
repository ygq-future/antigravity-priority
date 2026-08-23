package runtime

import "net/http"

// ManagementHandler exposes the same management handler used by CPA. It is
// intended for local integrations such as the devserver and does not change
// the plugin's host callback or ABI surface.
func (r *Runtime) ManagementHandler() http.Handler {
	return r.management
}
