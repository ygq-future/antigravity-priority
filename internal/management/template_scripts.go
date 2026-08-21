package management

// templateScripts assembles the self-contained dashboard script in dependency
// order. The fragments are private implementation details behind StatusHTML.
const templateScripts = templateScriptPrelude +
	templateScriptI18N +
	templateScriptShellCore +
	templateScriptControls +
	templateScriptOverviewActionsCore +
	templateScriptModals +
	templateScriptOverviewReset +
	templateScriptOverviewRender +
	templateScriptHistory +
	templateScriptDiagnostics +
	templateScriptLiveUI +
	templateScriptProbeSchedule +
	templateScriptHistoryDetails +
	templateScriptConfig +
	templateScriptBoot
