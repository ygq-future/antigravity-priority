package management

// templateCSS assembles dashboard styles in cascade order. The fragments are
// private implementation details behind StatusHTML.
const templateCSS = templateStyleTokens +
	templateStyleShell +
	templateStyleControls +
	templateStyleOverviewCards +
	templateStyleHistory +
	templateStyleDiagnostics +
	templateStyleConfig +
	templateStyleOverlays +
	templateStyleResponsive
