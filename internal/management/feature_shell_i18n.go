package management

const templateScriptI18N = featureShellTranslations +
	featureOverviewTranslations +
	featureHistoryTranslations +
	featureDiagnosticsTranslations +
	featureConfigTranslations +
	featureHelpTranslations +
	`        const I18N = {
            "zh-CN": Object.assign({},
                SHELL_ZH["zh-CN"],
                OVERVIEW_ZH["zh-CN"],
                HISTORY_ZH["zh-CN"],
                DIAGNOSTICS_ZH["zh-CN"],
                CONFIG_ZH["zh-CN"],
                HELP_ZH["zh-CN"]
            ),
            "en-US": Object.assign({},
                SHELL_EN["en-US"],
                OVERVIEW_EN["en-US"],
                HISTORY_EN["en-US"],
                DIAGNOSTICS_EN["en-US"],
                CONFIG_EN["en-US"],
                HELP_EN["en-US"]
            )
        };
`
