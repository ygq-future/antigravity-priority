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
            position: relative;
        }

        .cred-card-disabled {
            opacity: 0.72;
            filter: grayscale(0.25);
        }

        .cred-badge-row {
            display: flex;
            align-items: center;
            gap: 6px;
            flex-wrap: wrap;
        }

        .cred-footer {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
            margin-top: 2px;
            padding-top: 4px;
            border-top: 1px solid var(--border-color);
        }

        .cred-reason {
            font-size: 11px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            min-width: 0;
            flex: 1 1 auto;
        }

        .cred-reason-normal {
            color: var(--text-secondary);
        }

        .cred-reason-boost {
            color: var(--accent-purple-text);
            font-weight: 650;
        }

        .cred-reason-warning {
            color: var(--accent-yellow-text);
            font-weight: 650;
        }

        .cred-reason-danger {
            color: var(--accent-red-text);
            font-weight: 650;
        }

        .cred-reason-muted {
            color: var(--text-muted);
            font-weight: 600;
        }
        .cred-actions {
            display: flex;
            align-items: center;
            gap: 4px;
            flex: 0 0 auto;
        }

        .btn-card-action {
            min-height: 20px;
            height: 20px;
            padding: 0 6px;
            font-size: 11px;
            border-radius: 4px;
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
            border-left: 3px solid transparent;
            padding: 2px 6px;
            border-radius: 4px;
            box-sizing: border-box;
            transition: all 0.15s ease;
        }

        .meter-label-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 11.5px;
            line-height: 1.4;
            color: var(--text-primary);
        }

        .meter-name {
            display: inline-flex;
            align-items: center;
            font-size: 11.5px;
            color: var(--text-primary);
        }

        .meter-pool-name {
            color: var(--text-primary);
            font-weight: 700;
        }

        .meter-pct {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 11px;
            font-weight: 750;
            color: var(--text-primary);
            margin-left: 3px;
        }

        .meter-na {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 10.5px;
            color: var(--text-muted);
            font-weight: 500;
            margin-left: 3px;
        }

        .meter-track {
            height: 6px;
            min-height: 6px;
            background: var(--meter-track-bg);
            border: 1px solid var(--meter-track-border);
            border-radius: 999px;
            overflow: hidden;
            position: relative;
            margin-top: 2px;
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
            font-weight: 650;
            color: var(--text-primary);
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
            background: rgba(59, 130, 246, 0.08);
            border-left-color: var(--accent-blue);
        }
        .meter-container.active-pool-meter .meter-pool-name {
            color: var(--accent-blue-text);
            font-weight: 800;
        }

        .meter-container.active-pool-meter .meter-pct {
            color: var(--text-primary);
            font-weight: 800;
        }

        .meter-container.active-pool-meter .meter-countdown {
            color: var(--accent-blue-text);
            font-weight: 700;
        }
        .meter-container.inactive-pool-meter .meter-pool-name {
            color: var(--text-primary);
            font-weight: 700;
        }

        .meter-container.inactive-pool-meter .meter-pct {
            color: var(--text-primary);
            font-weight: 750;
        }

        .meter-container.inactive-pool-meter .meter-countdown {
            color: var(--text-primary);
            font-weight: 650;
        }

        .meter-est-burn {
            font-size: 10px;
            font-weight: 700;
            color: var(--accent-yellow-text);
            margin-left: 3px;
        }

        .badge-tier {
            background: var(--accent-blue-subtle);
            border: 1px solid rgba(59, 130, 246, 0.35);
            color: var(--accent-blue-text);
            font-weight: 750;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            letter-spacing: 0.2px;
        }

        /* Model Selector Dropdown & Badges */
        .model-select-menu {
            max-height: 380px;
            overflow-y: auto;
            min-width: 320px;
            width: max-content;
            max-width: 440px;
            right: auto;
            scrollbar-width: thin;
            scrollbar-color: var(--border-color) transparent;
            padding-right: 2px;
        }

        .model-select-menu::-webkit-scrollbar {
            width: 5px;
        }

        .model-select-menu::-webkit-scrollbar-track {
            background: transparent;
        }

        .model-select-menu::-webkit-scrollbar-thumb {
            background: var(--border-color);
            border-radius: 999px;
        }

        .model-select-menu::-webkit-scrollbar-thumb:hover {
            background: var(--text-muted);
        }

        .model-select-name {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 12.5px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            flex: 1 1 auto;
            text-align: left;
            min-width: 0;
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
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            flex: 1 1 auto;
            min-width: 0;
            margin-right: 8px;
        }

        .metric-pill-priority {
            background: var(--accent-green-subtle) !important;
            border: 1px solid rgba(16, 185, 129, 0.35) !important;
            color: var(--accent-green-text) !important;
            font-weight: 700;
        }

        .metric-pill-priority strong {
            color: var(--accent-green-text) !important;
            font-size: 12px;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            margin-left: 4px;
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

        /* Credentials Wrapper & Probing Loading Overlay */
        .credentials-wrapper {
            position: relative;
            flex: 1;
            min-height: 0;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }

        .credentials-loading-overlay {
            position: absolute;
            inset: 0;
            z-index: 20;
            display: flex;
            align-items: center;
            justify-content: center;
            pointer-events: auto;
        }

        .credentials-loading-card {
            position: relative;
            z-index: 3;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 14px;
            padding: 20px 28px;
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 12px;
            box-shadow: var(--shadow-lg);
            max-width: 380px;
            width: calc(100% - 32px);
            text-align: center;
            animation: cardScaleIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
        }

        @keyframes cardScaleIn {
            from { transform: scale(0.92); opacity: 0; }
            to { transform: scale(1); opacity: 1; }
        }

        .quota-probe-radar {
            position: relative;
            width: 52px;
            height: 52px;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 50%;
            background: var(--accent-blue-subtle);
        }

        .radar-ping {
            position: absolute;
            inset: -4px;
            border-radius: 50%;
            border: 2px solid var(--accent-blue);
            animation: radarPing 1.8s cubic-bezier(0, 0.2, 0.8, 1) infinite;
            opacity: 0.7;
            pointer-events: none;
        }

        @keyframes radarPing {
            0% { transform: scale(0.65); opacity: 0.85; }
            100% { transform: scale(1.65); opacity: 0; }
        }

        .radar-beacon {
            font-size: 24px;
            animation: beaconPulse 1.6s ease-in-out infinite;
            line-height: 1;
            user-select: none;
        }

        @keyframes beaconPulse {
            0%, 100% { transform: scale(1); }
            50% { transform: scale(1.15); }
        }

        .quota-loading-text {
            display: flex;
            flex-direction: column;
            gap: 4px;
        }

        .quota-loading-title {
            font-size: 14px;
            font-weight: 700;
            color: var(--text-primary);
            letter-spacing: -0.01em;
        }

        .quota-loading-desc {
            font-size: 12px;
            color: var(--text-muted);
            line-height: 1.45;
        }

        .quota-loading-bar {
            width: 100%;
            height: 4px;
            background: var(--meter-bg);
            border-radius: 999px;
            overflow: hidden;
            position: relative;
        }

        .quota-loading-bar-fill {
            position: absolute;
            top: 0;
            left: 0;
            bottom: 0;
            width: 35%;
            background: linear-gradient(90deg, var(--accent-blue), var(--accent-purple));
            border-radius: 999px;
            animation: indeterminateBar 1.4s ease-in-out infinite;
        }

        @keyframes indeterminateBar {
            0% { left: -35%; width: 30%; }
            50% { width: 50%; }
            100% { left: 100%; width: 30%; }
        }

        /* Probing State on Credentials Grid Cards */
        .credentials-grid.credentials-probing {
            pointer-events: none;
            user-select: none;
        }

        .credentials-grid.credentials-probing .cred-card {
            opacity: 0.45;
            transition: opacity 0.25s ease;
        }

        /* Card reorder & per-card probe feedback */
        .cred-card-enter {
            animation: credCardEnter 0.36s cubic-bezier(0.16, 1, 0.3, 1) both;
        }

        @keyframes credCardEnter {
            from { opacity: 0; transform: translateY(8px) scale(0.96); }
            to { opacity: 1; transform: translateY(0) scale(1); }
        }

        .cred-card-probing {
            animation: credCardProbeGlow 1.8s ease-in-out infinite;
        }

        .cred-card-probing::after {
            content: "";
            position: absolute;
            inset: 0;
            border-radius: 11px;
            padding: 1.5px;
            background: conic-gradient(
                from var(--cred-probe-angle, 0deg),
                transparent 0deg,
                transparent 208deg,
                var(--accent-blue) 268deg,
                var(--accent-purple) 326deg,
                transparent 360deg
            );
            -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
            -webkit-mask-composite: xor;
            mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
            mask-composite: exclude;
            animation: credCardProbeSpin 1.4s linear infinite;
            pointer-events: none;
        }

        @property --cred-probe-angle {
            syntax: "<angle>";
            initial-value: 0deg;
            inherits: false;
        }

        @keyframes credCardProbeSpin {
            from { --cred-probe-angle: 0deg; }
            to { --cred-probe-angle: 360deg; }
        }

        @keyframes credCardProbeGlow {
            0%, 100% { box-shadow: 0 0 0 1px var(--accent-blue-subtle), var(--shadow-sm); }
            50% { box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.22), 0 0 16px rgba(59, 130, 246, 0.14); }
        }

        .probe-spinner {
            display: inline-block;
            width: 9px;
            height: 9px;
            border: 2px solid currentColor;
            border-right-color: transparent;
            border-radius: 50%;
            animation: iconSpin 0.7s linear infinite;
        }

        .btn-probing .btn-probe-icon {
            display: inline-block;
            animation: iconSpin 1s linear infinite;
        }

        .btn-refreshing svg {
            animation: iconSpin 1s linear infinite;
        }

        @keyframes iconSpin {
            from { transform: rotate(0deg); }
            to { transform: rotate(360deg); }
        }
`
