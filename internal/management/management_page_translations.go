package management

// Feature translation ownership is declared beside the asset contract. The
// runtime dictionary remains one lookup table so language switching can be a
// Shell primitive, while these lists prevent a feature from silently losing a
// bilingual key during asset movement.
var overviewTranslationKeys = []string{
	"kpiTotal", "kpiBoosted", "kpiBoostedDesc", "kpiDepleted", "kpiDepletedDesc", "kpiLastAudit",
	"btnRefresh", "btnReset", "btnProbe", "btnSamples", "btnTier",
	"customTierModalTitle", "customTierSelectLabel", "customTierAutoOption", "customTierCustomInput", "customTierExplanation", "btnSaveCustomTier",
	"samplesModalTitle", "colObservedAt", "colShortRem", "colLongRem", "noSamples", "actualPriority", "targetPriority",
	"unsetPriority", "syncSuccess", "confirmReset", "resetSuccess", "loading", "noCreds",
	"noChanges", "statusActive", "statusBoosted", "statusWeeklyDepleted", "statusShortDepleted",
	"statusFailed", "shortWindow", "longWindow", "argonWindow", "resetIn", "priority", "urgencyLabel", "burnLabel", "running",
	"scheduleActive", "schedulePaused", "scheduleDisabled",
	"scheduleModeIdle", "scheduleModeLowFreq",
	"probeSuccess", "probeCooldown", "probing", "probingQuotaTitle", "probingQuotaDesc", "syncingQuotaTitle", "syncingQuotaDesc", "resettingTitle", "resettingDesc", "viewDetails", "statusCooldown", "statusDisabled", "btnProbeSingle", "probeSingleSuccess",
	"autoSchedulingTitle", "autoSchedulingDesc", "scheduleExecuting",
	"probeSingleFailed", "probeUnavailable", "probeSingleNoResult", "probeBatchFailed",
	"poolGemini", "poolArgon", "poolClaudeGPT", "badgeGemini", "badgeArgon", "badgeClaudeGPT",
	"statusUnsupported", "statusBypassedCooldown", "statusBypassedDepleted", "dynamicPriority", "estimatedBurn", "tierPrefix",
	"accountMaskOn", "accountMaskOff", "accountMaskToggleTitle",
	"warmupIconFollowTitle", "warmupIconOverrideTitle", "warmupCardEnabled", "warmupCardDisabled",
	"btnExcludeModel", "btnExcludedModel", "excludeModelTitle", "restoreModelTitle",
	"statusExcludedModel", "excludedModelBadgeTitle", "modelExcludedToast", "modelRestoredToast",
}

var historyTranslationKeys = []string{
	"historyTitle", "noHistory", "btnRefresh", "btnClearHistory",
	"historyRetainLabel", "historyRetainUnit", "btnSaveHistoryLimit", "historyLimitSaved", "historyErrorDetails",
	"valErrRequestHistoryLimit",
	"thTime", "thModel", "thCredential", "thTokens", "thStatus",
	"statusRateLimited", "statusError",
}

var diagnosticsTranslationKeys = []string{
	"diagnosticsTitle", "btnCopyDiagnostics",
	"diagKpiScheduler", "diagKpiCooldown", "diagKpiLastApply",
	"diagSectionPools", "diagSectionPlan", "diagSectionCooldown", "diagSectionTimeline", "diagSectionStorage",
	"diagPoolLabel", "diagColActive", "diagColBoost", "diagColDepleted", "diagColCooldown",
	"diagColMin5h", "diagColMin7d", "diagColNearestReset", "diagColAvgBurn",
	"diagColCredential", "diagColPriority", "diagColDecision", "diagColUrgency", "diagColHorizon", "diagColCycle", "diagColReason",
	"diagHealthOk", "diagHealthWarning", "diagHealthCritical", "diagHealthStorm", "diagHealthStorageError",
	"diagPending429Title", "diagPending429Empty", "diagStormTitle", "diagStormUntil", "diagStormAccounts", "diagStormRecovered",
	"diagTimelineEmpty", "diagRunRecordPersisted", "diagRunRecordFailed", "diagRunRecordNotAttempted",
	"diagStorageRoleState", "diagStorageRoleRuntime", "diagStorageRoleRuns", "diagStorageRoleRequests", "diagStorageMissing", "diagStorageEmpty",
	"diagCooldownSubText", "diagCooldownEmptyText", "diagNoLastRun", "diagStatusRunning", "diagStatusPaused", "diagStatusDisabled",
	"diagModeIdle", "diagModeLowFreq", "diagModeFrequent", "diagSchedIdleDesc",
	"diagHealthy", "diagTripped", "diagCoolingDownCount",
	"diagAllPassed", "diagHasFailed", "diagNoChanges", "diagSuccessPill", "diagFailedPill", "diagAttemptedPill",
	"diagRemainingSec", "copiedSuccess",
}

var configTranslationKeys = []string{
	"cfgCardScheduleTitle", "cfgAutoApply", "cfgAutoApplyHint", "cfgInterval", "cfgIntervalHint",
	"cfgCardPerfTitle", "cfgMaxConcurrency", "cfgMaxConcurrencyHint",
	"cfgUrgencyTolerance", "cfgUrgencyToleranceHint", "cfgSampleCapacity", "cfgSampleCapacityHint",
	"cfgCooldownMinutes", "cfgCooldownMinutesHint",
	"btnSaveConfig", "btnResetToDefaults", "configSaveSuccess", "confirmResetConfigTitle", "confirmResetConfigMsg",
	"optInterval5m", "optInterval15m", "optInterval30m", "optInterval1h", "optIntervalCustom", "valErrInterval",
	"valErrConcurrency", "valErrUrgencyTol", "valErrSampleCapacity", "valErrCooldown",
	"cfgCardWarmupTitle", "cfgWarmupGemini", "cfgWarmupGeminiHint", "cfgWarmupGeminiHintTitle", "cfgWarmupGeminiModel", "cfgWarmupGeminiModelHint",
	"cfgWarmupClaude", "cfgWarmupClaudeHint", "cfgWarmupClaudeHintTitle", "cfgWarmupClaudeModel", "cfgWarmupClaudeModelHint",

	"valErrWarmupGeminiModel", "valErrWarmupClaudeModel",
}

var helpTranslationKeys = []string{
	"helpTitle", "helpGithub", "helpP1", "help5hTitle", "help5hDesc", "help7dTitle", "help7dDesc", "helpBurnTitle", "helpBurnDesc",
	"helpUrgencyTitle", "helpUrgencyDesc", "helpBoostTitle", "helpBoostDesc", "helpWarmupTitle", "helpWarmupDesc",
	"helpTierTitle", "helpTierDesc", "help429Title", "help429Desc", "helpUITitle", "helpUIDesc",
}
