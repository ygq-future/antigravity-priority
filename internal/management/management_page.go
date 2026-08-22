package management

import "strings"

// managementPageAsset is the only contract between a user feature and the
// management-page assembler. A feature supplies all of its document assets
// together; the assembler owns the final CSS and JavaScript ordering.
type managementPageAsset struct {
	name            string
	markup          string
	styles          string
	scripts         string
	translationKeys []string
}

// managementPageFeatures is deliberately ordered by the visible navigation.
// The order is part of the delivered document contract, not a responsibility
// of the HTTP handler or a caller that happens to serve StatusHTML.
var managementPageFeatures = [...]managementPageAsset{
	overviewPageAsset,
	historyPageAsset,
	diagnosticsPageAsset,
	configPageAsset,
	helpPageAsset,
}

const managementPageDocumentStart = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Antigravity Priority</title>
    <style>`

const managementPageDocumentEnd = `
    </div>
</body>
</html>
`

// assembleManagementPage is the single in-process seam for the embedded
// management page. It hides CSS cascade order, JavaScript dependency order,
// shared Shell primitives, and final document construction from the handler.
func assembleManagementPage() string {
	var page strings.Builder
	page.Grow(len(managementPageDocumentStart) + len(managementPageShellMarkup) + len(managementPageDocumentEnd) + 32*1024)

	page.WriteString(managementPageDocumentStart)
	page.WriteString(managementPageShellStyles)
	for _, feature := range managementPageFeatures {
		page.WriteString(feature.styles)
	}
	page.WriteString(managementPageShellStylesTail)
	page.WriteString(`</style>`)
	page.WriteString(managementPageShellMarkup)

	for _, feature := range managementPageFeatures {
		page.WriteString(feature.markup)
	}
	page.WriteString(shellModalMarkup)
	page.WriteString(managementPageShellScripts)
	for _, feature := range managementPageFeatures {
		page.WriteString(feature.scripts)
	}
	page.WriteString(templateScriptBoot)
	page.WriteString(managementPageDocumentEnd)

	return page.String()
}

// StatusHTML is the fully assembled, self-contained management page served by
// /status. It is intentionally the only page resource exposed to the handler.
var StatusHTML = assembleManagementPage()
