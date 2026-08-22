package management

// Feature translation ownership is declared beside the asset contract. The
// runtime dictionary remains one lookup table so language switching can be a
// Shell primitive, while these lists prevent a feature from silently losing a
// bilingual key during asset movement.
var overviewTranslationKeys = []string{
	"kpiTotal", "kpiBoosted", "kpiBoostedDesc", "kpiDepleted", "kpiDepletedDesc", "kpiLastAudit",
	"labelModelGroup", "optGemini", "optClaudeGPT", "btnRefresh", "btnApply", "btnReset", "btnProbe", "btnSamples",
	"samplesModalTitle", "colObservedAt", "colShortRem", "colLongRem", "noSamples", "actualPriority", "targetPriority",
	"predictedPriority", "pendingApply", "targetChanged", "manualApplyRecheck", "unsetPriority", "syncSuccess", "confirmReset", "resetSuccess", "loading", "noCreds",
	"noChanges", "noChangesToApply", "statusActive", "statusBoosted", "statusWeeklyDepleted", "statusShortDepleted",
	"statusFailed", "shortWindow", "longWindow", "resetIn", "priority", "urgencyLabel", "burnLabel", "running",
	"scheduleActive", "schedulePaused", "scheduleSleeping", "scheduleDisabled", "predictedBadge", "predictedNote",
	"probeSuccess", "probeCooldown", "viewDetails", "statusCooldown",
}

var historyTranslationKeys = []string{
	"historyTitle", "noHistory", "btnRefresh",
}

var diagnosticsTranslationKeys = []string{
	"diagnosticsTitle", "btnCopyDiagnostics", "diagKpiScheduler", "diagKpiCooldown", "diagCooldownSubText", "diagKpiLastApply",
	"diagSectionScheduler", "diagSchedIntervalLabel", "diagSchedWorkerLabel", "diagSchedLastRunLabel", "diagSchedNextRunLabel",
	"diagWindowPolicyLabel", "diagSectionCooldown", "diagCooldownEmptyText", "diagSectionAudit", "diagAuditSummaryLabel",
	"diagWindowContinuous", "diagWindowActive", "diagWindowSleeping", "diagNoLastRun", "diagStatusRunning", "diagStatusPaused",
	"diagStatusSleeping", "diagStatusDisabled", "diagWorkerActive", "diagWorkerInactive", "diagHealthy", "diagTripped",
	"diagCoolingDownCount", "diagAllPassed", "diagHasFailed", "diagNoChanges", "diagSuccessPill", "diagFailedPill",
	"diagSkippedPill", "diagAttemptedPill", "diagRemainingSec",
}

var configTranslationKeys = []string{
	"cfgCardScheduleTitle", "cfgAutoApply", "cfgAutoApplyHint", "cfgInterval", "cfgIntervalHint", "cfgModelGroup", "cfgModelGroupHint",
	"cfgScheduleWindow", "cfgScheduleWindowHint", "cfgWindowEnabledLabel", "cfgCardPerfTitle", "cfgMaxConcurrency", "cfgMaxConcurrencyHint",
	"cfgMinChange", "cfgMinChangeHint", "cfgUrgencyTolerance", "cfgUrgencyToleranceHint", "cfgSampleCapacity", "cfgSampleCapacityHint",
	"cfgCooldownMinutes", "cfgCooldownMinutesHint", "cfgCardRulesTitle", "cfgBoostStart", "cfgBoostStartHint", "cfgNormalStart",
	"cfgNormalStartHint", "btnSaveConfig", "btnResetToDefaults", "configSaveSuccess", "confirmResetConfigTitle", "confirmResetConfigMsg",
	"optInterval5m", "optInterval15m", "optInterval30m", "optInterval1h", "optIntervalCustom", "valErrInterval", "valErrWindow",
	"valErrConcurrency", "valErrMinChange", "valErrUrgencyTol", "valErrSampleCapacity", "valErrCooldown", "valErrPriorityRange",
	"valErrPriorityOrder", "optGemini", "optClaudeGPT",
}

var helpTranslationKeys = []string{
	"helpTitle", "helpP1", "help5hTitle", "help5hDesc", "help7dTitle", "help7dDesc", "helpBurnTitle", "helpBurnDesc",
	"helpUrgencyTitle", "helpUrgencyDesc", "helpBoostTitle", "helpBoostDesc", "help429Title", "help429Desc",
}
