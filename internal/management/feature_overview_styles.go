package management

const templateStyleOverviewCards = `        /* Credentials Display (Grid 3 Columns) */
        .credentials-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
            grid-auto-rows: max-content !important;
            align-content: start !important;
            gap: 12px;
        }

        .cred-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 10px 12px;
            display: flex;
            flex-direction: column;
            gap: 6px;
            box-shadow: var(--shadow-sm);
            height: fit-content !important;
            align-self: start !important;
        }

        .cred-badge-row {
            display: flex;
            align-items: center;
            gap: 6px;
            flex-wrap: wrap;
        }

        .cred-priority {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
        }

        .cred-info {
            display: flex;
            flex-direction: column;
            gap: 3px;
            min-width: 0;
        }

		.cred-name {
            font-size: 13px;
            font-weight: 700;
            color: var(--text-primary);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            display: flex;
            align-items: center;
            gap: 6px;
		}

		.cred-email {
			min-width: 0;
			overflow: hidden;
			text-overflow: ellipsis;
		}

		.cred-meta {
			flex: 0 0 auto;
            font-size: 11px;
            color: var(--text-muted);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .metric-pill {
            display: inline-flex;
            align-items: center;
            font-size: 11px;
            padding: 1px 6px;
            border-radius: 4px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
        }

        .metric-pill strong {
            color: var(--text-primary);
            margin-left: 3px;
        }

        .metric-pill-urgency {
            background: var(--accent-blue-subtle) !important;
            border: 1px solid rgba(37, 99, 235, 0.2) !important;
            color: var(--accent-blue-text) !important;
        }
        .metric-pill-urgency strong {
            color: var(--accent-blue-text) !important;
        }

        .metric-pill-burn {
            background: var(--accent-purple-subtle) !important;
            border: 1px solid rgba(124, 58, 237, 0.2) !important;
            color: var(--accent-purple-text) !important;
        }
        .metric-pill-burn strong {
            color: var(--accent-purple-text) !important;
        }

        .meter-container {
            display: flex;
            flex-direction: column;
            gap: 2px;
            width: 100%;
        }

        .meter-container-stale {
            opacity: 0.75;
        }

        .meter-stale-tag {
            display: inline-block;
            font-size: 10px;
            font-weight: 500;
            padding: 0 4px;
            border-radius: 3px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-color);
            color: var(--text-muted);
            margin-left: 4px;
            line-height: 14px;
            vertical-align: middle;
        }
        .meter-label-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 11px;
            font-weight: 600;
            color: var(--text-muted);
        }

        .meter-track {
            height: 7px;
            min-height: 7px;
            background: var(--meter-track-bg);
            border: 1px solid var(--meter-track-border);
            border-radius: 999px;
            overflow: hidden;
            position: relative;
        }

        .meter-fill {
            height: 100%;
            border-radius: 999px;
            transition: width 0.4s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.3s ease;
        }

        .meter-fill-healthy { background: var(--accent-green); }
        .meter-fill-warning { background: var(--meter-warn); }
        .meter-fill-danger  { background: var(--meter-danger); }

        .meter-countdown {
            font-family: SFMono-Regular, Consolas, "Liberation Mono", Menlo, monospace;
            font-size: 11px;
            color: var(--text-secondary);
        }

        .priority-score-box {
            display: inline-flex;
            align-items: center;
            gap: 5px;
        }

        .priority-score {
            font-size: 16px;
            font-weight: 800;
            color: var(--accent-blue-text);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            line-height: 1;
        }

        .priority-predicted {
            color: var(--accent-purple-text);
        }

        .empty-state {
            grid-column: 1 / -1;
            width: 100%;
            box-sizing: border-box;
            text-align: center;
            padding: 48px 20px;
            color: var(--text-muted);
            font-size: 14px;
        }

        .sample-table {
            width: 100%;
            border-collapse: separate;
            border-spacing: 0;
            font-size: 12px;
            text-align: left;
            border-radius: 8px;
            overflow: hidden;
            border: 1px solid var(--border-color);
        }

        .sample-table th {
            padding: 9px 12px;
            font-weight: 700;
            color: var(--text-muted);
            border-bottom: 1px solid var(--border-color);
            background: var(--bg-subtle);
            font-size: 11px;
            white-space: nowrap;
        }

        .sample-table td {
            padding: 7px 12px;
            color: var(--text-primary);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 11.5px;
            white-space: nowrap;
            transition: background 0.12s ease;
        }

        .sample-table tbody.sample-group:hover td {
            background: var(--bg-hover) !important;
        }

        .sample-table tbody.sample-group:not(:last-child) td.sample-group-bottom {
            border-bottom: 1px solid var(--border-color);
        }

        /* 5-Bar Quota Progress Grid */
        .quota-bars-grid {
            display: flex;
            flex-direction: column;
            gap: 4px;
            width: 100%;
        }

        .meter-container.active-pool-meter {
            background: var(--bg-subtle);
            border-left: 2px solid var(--accent-blue);
            padding-left: 6px;
            border-radius: 4px;
            padding-top: 2px;
            padding-bottom: 2px;
        }

        .meter-container.inactive-pool-meter {
            opacity: 0.85;
        }

        .meter-est-burn {
            font-size: 10px;
            font-weight: 700;
            color: var(--accent-yellow-text);
            margin-left: 3px;
        }

        .badge-tier {
            background: var(--bg-subtle);
            border: 1px solid var(--border-color);
            color: var(--text-primary);
            font-weight: 700;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
        }

        /* Model Selector Dropdown & Badges */
        .model-select-menu {
            max-height: 380px;
            overflow-y: auto;
            min-width: 260px;
        }

        .model-select-group-header {
            font-size: 10.5px;
            font-weight: 700;
            color: var(--text-muted);
            padding: 6px 10px 4px;
            background: var(--bg-subtle);
            border-top: 1px solid var(--border-subtle);
            border-bottom: 1px solid var(--border-subtle);
            user-select: none;
            pointer-events: none;
        }

        .model-select-group-header:first-child {
            border-top: none;
        }

        .model-pool-badge {
            display: inline-flex;
            align-items: center;
            font-size: 10px;
            font-weight: 700;
            padding: 1px 5px;
            border-radius: 4px;
            line-height: 14px;
            flex-shrink: 0;
            margin-right: 6px;
        }

        .badge-gemini {
            background: rgba(37, 99, 235, 0.12);
            color: var(--accent-blue-text, #2563eb);
            border: 1px solid rgba(37, 99, 235, 0.25);
        }

        .badge-argon {
            background: rgba(147, 51, 234, 0.12);
            color: var(--accent-purple-text, #9333ea);
            border: 1px solid rgba(147, 51, 234, 0.25);
        }

        .badge-claude {
            background: rgba(217, 119, 6, 0.12);
            color: var(--accent-yellow-text, #d97706);
            border: 1px solid rgba(217, 119, 6, 0.25);
        }

        .model-option-name {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 12px;
        }

        .metric-pill-priority {
            background: var(--bg-subtle);
            border: 1px solid var(--border-color);
        }

        /* Samples Modal Pool Switcher */
        .samples-pool-switcher {
            display: flex;
            gap: 6px;
            margin: 8px 0;
        }

        .btn-pool-pill {
            padding: 3px 10px;
            font-size: 12px;
            font-weight: 600;
            border-radius: 999px;
            border: 1px solid var(--border-color);
            background: var(--bg-subtle);
            color: var(--text-secondary);
            cursor: pointer;
            transition: all 0.15s ease;
        }

        .btn-pool-pill.active {
            background: var(--accent-blue);
            color: #ffffff;
            border-color: var(--accent-blue);
        }
`
