package management

// TemplateCSS contains the complete, self-contained CSS styles for the dashboard.
// It complies with strict CSP and CPA host dual-theme standards.
const TemplateCSS = `
        :root {
            color-scheme: light dark;
            --bg-primary: #f8fafc;
            --bg-secondary: #f1f5f9;
            --bg-tertiary: #e2e8f0;
            --bg-surface: #ffffff;
            --bg-card: #ffffff;
            --bg-subtle: #f1f5f9;
            --bg-hover: #e2e8f0;
            --bg-overlay: rgba(15, 23, 42, 0.45);
            --border-color: #e2e8f0;
            --border-subtle: #f1f5f9;
            --border-focus: #3b82f6;
            --text-primary: #0f172a;
            --text-secondary: #475569;
            --text-muted: #94a3b8;
            --text-inverse: #ffffff;
            --accent-blue: #2563eb;
            --accent-blue-hover: #1d4ed8;
            --accent-blue-subtle: #eff6ff;
            --accent-blue-text: #1d4ed8;
            --accent-green: #10b981;
            --accent-green-subtle: #ecfdf5;
            --accent-green-text: #047857;
            --accent-yellow: #f59e0b;
            --accent-yellow-subtle: #fffbeb;
            --accent-yellow-text: #b45309;
            --accent-red: #ef4444;
            --accent-red-subtle: #fef2f2;
            --accent-red-text: #b91c1c;
            --accent-purple: #7c3aed;
            --accent-purple-subtle: #faf5ff;
            --accent-purple-text: #6d28d9;
            --meter-bg: #e2e8f0;
            --meter-fill: #10b981;
            --meter-warn: #f59e0b;
            --meter-danger: #ef4444;
            --badge-boost-bg: #fdf4ff;
            --badge-boost-border: #f5d0fe;
            --badge-boost-text: #9333ea;
            --diff-from-bg: #fee2e2;
            --diff-from-text: #991b1b;
            --diff-to-bg: #dcfce7;
            --diff-to-text: #166534;
            --shadow-sm: 0 1px 3px 0 rgba(0, 0, 0, 0.04), 0 1px 2px -1px rgba(0, 0, 0, 0.03);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.03);
            --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.06), 0 4px 6px -4px rgba(0, 0, 0, 0.03);
        }

        @media (prefers-color-scheme: dark) {
            :root:not([data-theme="light"]) {
                --bg-primary: #121214;
                --bg-secondary: #18181b;
                --bg-tertiary: #27272a;
                --bg-surface: #18181b;
                --bg-card: #1f1f23;
                --bg-subtle: #27272a;
                --bg-hover: #3f3f46;
                --bg-overlay: rgba(0, 0, 0, 0.7);
                --border-color: #27272a;
                --border-subtle: #27272a;
                --border-focus: #a1a1aa;
                --text-primary: #f4f4f5;
                --text-secondary: #a1a1aa;
                --text-muted: #71717a;
                --text-inverse: #0f172a;
                --accent-blue: #3b82f6;
                --accent-blue-hover: #60a5fa;
                --accent-blue-subtle: #1e293b;
                --accent-blue-text: #60a5fa;
                --accent-green: #22c55e;
                --accent-green-subtle: rgba(20, 83, 45, 0.25);
                --accent-green-text: #4ade80;
                --accent-yellow: #f59e0b;
                --accent-yellow-subtle: rgba(120, 53, 15, 0.25);
                --accent-yellow-text: #fbbf24;
                --accent-red: #ef4444;
                --accent-red-subtle: rgba(127, 29, 29, 0.25);
                --accent-red-text: #f87171;
                --accent-purple: #a855f7;
                --accent-purple-subtle: rgba(88, 28, 135, 0.25);
                --accent-purple-text: #c084fc;
                --meter-bg: #27272a;
                --meter-fill: #3b82f6;
                --meter-warn: #f59e0b;
                --meter-danger: #ef4444;
                --badge-boost-bg: rgba(74, 4, 78, 0.35);
                --badge-boost-border: #a21caf;
                --badge-boost-text: #f0abfc;
                --diff-from-bg: rgba(127, 29, 29, 0.35);
                --diff-from-text: #fca5a5;
                --diff-to-bg: rgba(20, 83, 45, 0.35);
                --diff-to-text: #86efac;
                --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.4);
                --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.4), 0 2px 4px -2px rgba(0, 0, 0, 0.4);
                --shadow-lg: 0 10px 18px -3px rgba(0, 0, 0, 0.5), 0 4px 6px -4px rgba(0, 0, 0, 0.5);
            }
        }

        :root[data-theme="dark"] {
            --bg-primary: #121214;
            --bg-secondary: #18181b;
            --bg-tertiary: #27272a;
            --bg-surface: #18181b;
            --bg-card: #1f1f23;
            --bg-subtle: #27272a;
            --bg-hover: #3f3f46;
            --bg-overlay: rgba(0, 0, 0, 0.7);
            --border-color: #27272a;
            --border-subtle: #27272a;
            --border-focus: #a1a1aa;
            --text-primary: #f4f4f5;
            --text-secondary: #a1a1aa;
            --text-muted: #71717a;
            --text-inverse: #0f172a;
            --accent-blue: #3b82f6;
            --accent-blue-hover: #60a5fa;
            --accent-blue-subtle: #1e293b;
            --accent-blue-text: #60a5fa;
            --accent-green: #22c55e;
            --accent-green-subtle: rgba(20, 83, 45, 0.25);
            --accent-green-text: #4ade80;
            --accent-yellow: #f59e0b;
            --accent-yellow-subtle: rgba(120, 53, 15, 0.25);
            --accent-yellow-text: #fbbf24;
            --accent-red: #ef4444;
            --accent-red-subtle: rgba(127, 29, 29, 0.25);
            --accent-red-text: #f87171;
            --accent-purple: #a855f7;
            --accent-purple-subtle: rgba(88, 28, 135, 0.25);
            --accent-purple-text: #c084fc;
            --meter-bg: #27272a;
            --meter-fill: #3b82f6;
            --meter-warn: #f59e0b;
            --meter-danger: #ef4444;
            --badge-boost-bg: rgba(74, 4, 78, 0.35);
            --badge-boost-border: #a21caf;
            --badge-boost-text: #f0abfc;
            --diff-from-bg: rgba(127, 29, 29, 0.35);
            --diff-from-text: #fca5a5;
            --diff-to-bg: rgba(20, 83, 45, 0.35);
            --diff-to-text: #86efac;
            --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.4);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.4), 0 2px 4px -2px rgba(0, 0, 0, 0.4);
            --shadow-lg: 0 10px 18px -3px rgba(0, 0, 0, 0.5), 0 4px 6px -4px rgba(0, 0, 0, 0.5);
        }

        :root[data-theme="light"] {
            --bg-primary: #f8fafc;
            --bg-secondary: #f1f5f9;
            --bg-tertiary: #e2e8f0;
            --bg-surface: #ffffff;
            --bg-card: #ffffff;
            --bg-subtle: #f1f5f9;
            --bg-hover: #e2e8f0;
            --bg-overlay: rgba(15, 23, 42, 0.45);
            --border-color: #e2e8f0;
            --border-subtle: #f1f5f9;
            --border-focus: #3b82f6;
            --text-primary: #0f172a;
            --text-secondary: #475569;
            --text-muted: #94a3b8;
            --text-inverse: #ffffff;
            --accent-blue: #2563eb;
            --accent-blue-hover: #1d4ed8;
            --accent-blue-subtle: #eff6ff;
            --accent-blue-text: #1d4ed8;
            --accent-green: #10b981;
            --accent-green-subtle: #ecfdf5;
            --accent-green-text: #047857;
            --accent-yellow: #f59e0b;
            --accent-yellow-subtle: #fffbeb;
            --accent-yellow-text: #b45309;
            --accent-red: #ef4444;
            --accent-red-subtle: #fef2f2;
            --accent-red-text: #b91c1c;
            --accent-purple: #7c3aed;
            --accent-purple-subtle: #faf5ff;
            --accent-purple-text: #6d28d9;
            --meter-bg: #e2e8f0;
            --meter-fill: #10b981;
            --meter-warn: #f59e0b;
            --meter-danger: #ef4444;
            --badge-boost-bg: #fdf4ff;
            --badge-boost-border: #f5d0fe;
            --badge-boost-text: #9333ea;
            --diff-from-bg: #fee2e2;
            --diff-from-text: #991b1b;
            --diff-to-bg: #dcfce7;
            --diff-to-text: #166534;
            --shadow-sm: 0 1px 3px 0 rgba(0, 0, 0, 0.04), 0 1px 2px -1px rgba(0, 0, 0, 0.03);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.03);
            --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.06), 0 4px 6px -4px rgba(0, 0, 0, 0.03);
        }

        * { box-sizing: border-box; }
        html, body {
            height: 100%;
            margin: 0;
            padding: 0;
            overflow: hidden;
        }

        body {
            background: var(--bg-primary);
            color: var(--text-primary);
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.5;
            display: flex;
            flex-direction: column;
        }

        .container {
            width: 100%;
            max-width: 1320px;
            height: 100%;
            margin: 0 auto;
            display: flex;
            flex-direction: column;
            padding: 16px 20px;
            box-sizing: border-box;
            overflow: hidden;
            position: relative;
        }

        .topbar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
            flex-wrap: wrap;
            margin-bottom: 12px;
            padding-bottom: 12px;
            border-bottom: 1px solid var(--border-subtle);
            flex-shrink: 0;
        }

        .brand-zone {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        h1 {
            margin: 0;
            font-size: 20px;
            font-weight: 750;
            letter-spacing: -0.02em;
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .version-badge {
            display: inline-flex;
            align-items: center;
            padding: 2px 8px;
            border-radius: 999px;
            background: var(--accent-blue-subtle);
            color: var(--accent-blue-text);
            font-size: 12px;
            font-weight: 700;
        }

        .topbar-actions {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-wrap: wrap;
            margin-right: 140px;
        }

        .tabs {
            display: flex;
            gap: 4px;
            padding: 3px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 10px;
            margin-bottom: 12px;
            flex-shrink: 0;
        }

        .tab {
            flex: 1;
            min-height: 34px;
            background: transparent;
            color: var(--text-secondary);
            border: 0;
            border-radius: 7px;
            padding: 6px 14px;
            font-size: 13px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.15s ease;
            white-space: nowrap;
        }

        .tab.active {
            background: var(--bg-card);
            color: var(--text-primary);
            box-shadow: var(--shadow-sm);
        }

        section[id^="panel"] {
            flex: 1;
            min-height: 0;
            display: flex;
            flex-direction: column;
            overflow: hidden;
            transition: opacity 0.18s ease-out;
            position: relative;
        }

        .card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 16px;
        }

        /* KPI Cards Grid */
        .summary-grid {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr));
            gap: 12px;
            margin-bottom: 12px;
            flex-shrink: 0;
        }

        .kpi-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 10px;
            padding: 10px 14px;
            display: flex;
            flex-direction: column;
            gap: 2px;
            justify-content: center;
        }

        .kpi-title {
            margin: 0;
            font-size: 12px;
            font-weight: 600;
            color: var(--text-muted);
        }

        .kpi-value {
            font-size: 22px;
            font-weight: 800;
            letter-spacing: -0.02em;
            color: var(--text-primary);
            line-height: 1.2;
        }

        .kpi-desc {
            font-size: 11px;
            color: var(--text-secondary);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        /* Control Bar - Single row unified dimensions */
        .control-bar {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-wrap: wrap;
            padding: 8px 12px;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 10px;
            margin-bottom: 12px;
            flex-shrink: 0;
        }

        .control-bar button,
        .control-bar .custom-select-trigger,
        .control-bar .schedule-status {
            height: 34px !important;
            min-height: 34px !important;
            padding: 0 12px !important;
            font-size: 12.5px !important;
            font-weight: 650 !important;
            border-radius: 8px !important;
            box-sizing: border-box !important;
            display: inline-flex !important;
            align-items: center !important;
            justify-content: center !important;
            gap: 5px !important;
            line-height: 1 !important;
            margin: 0 !important;
            white-space: nowrap !important;
        }

        .custom-select-wrapper {
            position: relative;
            user-select: none;
            min-width: 140px;
            max-width: 190px;
        }

        .custom-select-trigger {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            color: var(--text-primary);
            cursor: pointer;
            box-shadow: var(--shadow-sm);
            transition: all 0.15s ease;
            width: 100%;
        }

        .custom-select-trigger:hover {
            border-color: var(--border-focus);
            background: var(--bg-subtle);
        }

        .custom-select-wrapper.open .custom-select-trigger {
            border-color: var(--border-focus);
            box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
        }

        .custom-select-arrow {
            display: flex;
            align-items: center;
            color: var(--text-muted);
            transition: transform 0.2s ease;
            flex-shrink: 0;
        }

        .custom-select-wrapper.open .custom-select-arrow {
            transform: rotate(180deg);
        }

        .custom-select-options {
            position: absolute;
            top: calc(100% + 4px);
            left: 0;
            right: 0;
            z-index: 60;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 10px;
            padding: 4px;
            box-shadow: var(--shadow-lg);
            display: flex;
            flex-direction: column;
            gap: 2px;
            min-width: 160px;
        }

        .custom-select-option {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 6px 10px;
            border-radius: 6px;
            font-size: 12px;
            font-weight: 600;
            color: var(--text-secondary);
            cursor: pointer;
            transition: all 0.12s ease;
        }

        .custom-select-option:hover {
            background: var(--bg-hover, var(--bg-subtle));
            color: var(--text-primary);
        }

        .custom-select-option.selected {
            background: var(--bg-subtle);
            color: var(--text-primary);
            font-weight: 700;
        }

        .custom-select-check {
            display: none;
            color: var(--accent-blue);
        }

        .custom-select-option.selected .custom-select-check {
            display: inline-flex;
        }

        input[type="password"], input[type="text"], input[type="number"] {
            min-height: 34px;
            height: 34px;
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 6px 10px;
            font: inherit;
            font-size: 13px;
            background: var(--bg-card);
            color: var(--text-primary);
            box-sizing: border-box;
        }

        button {
            min-height: 34px;
            border-radius: 8px;
            border: 1px solid transparent;
            padding: 6px 12px;
            font: inherit;
            font-size: 13px;
            font-weight: 650;
            cursor: pointer;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 6px;
            transition: all 0.15s ease;
            white-space: nowrap;
        }

        .btn-primary {
            background: var(--accent-blue);
            color: var(--text-inverse);
        }
        .btn-primary:hover { background: var(--accent-blue-hover); }

        .btn-secondary {
            background: var(--bg-card);
            border-color: var(--border-color);
            color: var(--text-primary);
        }
        .btn-secondary:hover { background: var(--bg-subtle); }

        .btn-danger {
            background: var(--accent-red-subtle);
            color: var(--accent-red-text);
            border-color: rgba(239, 68, 68, 0.2);
        }
        .btn-danger:hover {
            background: rgba(239, 68, 68, 0.15);
            border-color: rgba(239, 68, 68, 0.35);
        }

        button:disabled {
            opacity: 0.6;
            cursor: not-allowed !important;
        }

        .badge {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 2px 7px;
            border-radius: 5px;
            font-size: 11px;
            font-weight: 700;
            line-height: 1.3;
        }

        .badge-boost {
            background: var(--badge-boost-bg);
            border: 1px solid var(--badge-boost-border);
            color: var(--badge-boost-text);
        }

        .badge-success {
            background: var(--accent-green-subtle);
            color: var(--accent-green-text);
        }

        .badge-warning {
            background: var(--accent-yellow-subtle);
            color: var(--accent-yellow-text);
        }

        .badge-danger {
            background: var(--accent-red-subtle);
            color: var(--accent-red-text);
        }

        .badge-subtle {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            color: var(--text-secondary);
        }

        .badge-predicted {
            background: var(--accent-purple-subtle);
            border: 1px solid rgba(124, 58, 237, 0.2);
            color: var(--accent-purple-text);
            font-size: 10px;
        }

        .badge-pending {
            background: var(--accent-yellow-subtle);
            border: 1px solid rgba(245, 158, 11, 0.25);
            color: var(--accent-yellow-text);
            font-size: 10px;
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

        .schedule-status {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            color: var(--text-secondary);
            cursor: pointer;
            transition: all 0.15s ease;
            user-select: none;
        }
        .schedule-status:hover { background: var(--bg-hover); }
        .schedule-status.active { color: var(--accent-green-text); }
        .schedule-status.paused { color: var(--accent-yellow-text); }
        .schedule-status.sleeping { color: var(--accent-purple-text) !important; }

        .btn-cooldown { opacity: 0.6; cursor: not-allowed !important; }

        /* Scroll Containment */
        .scroll-container {
            flex: 1;
            min-height: 0;
            overflow-y: auto;
            scrollbar-width: thin;
            scrollbar-color: var(--border-color) transparent;
            padding-right: 2px;
        }
        .scroll-container::-webkit-scrollbar { width: 5px; }
        .scroll-container::-webkit-scrollbar-track { background: transparent; }
        .scroll-container::-webkit-scrollbar-thumb { background: var(--border-color); border-radius: 999px; }

        /* Credentials Display (Grid 3 Columns) */
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

        .cred-meta {
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

        .meter-label-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 11px;
            font-weight: 600;
            color: var(--text-muted);
        }

        .meter-track {
            height: 5px;
            background: var(--meter-bg);
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

        /* History Items - 100% full width and unified height */
        #historyList {
            display: flex !important;
            flex-direction: column !important;
            justify-content: flex-start !important;
            align-items: stretch !important;
            gap: 10px !important;
            width: 100% !important;
            height: 100% !important;
            box-sizing: border-box !important;
            overflow-y: auto !important;
        }

        .history-item {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 10px;
            padding: 10px 16px;
            display: flex;
            flex-direction: column;
            justify-content: space-between;
            gap: 6px;
            flex-shrink: 0 !important;
            width: 100% !important;
            box-sizing: border-box !important;
            min-height: 64px !important;
        }

        .history-head {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            flex-wrap: wrap;
            width: 100%;
        }

        .history-stats {
            display: flex;
            gap: 6px;
            flex-wrap: wrap;
            align-items: center;
        }

        pre.code-block {
            margin: 0;
            padding: 12px;
            border-radius: 8px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 12px;
            overflow: auto;
            color: var(--text-primary);
        }

        .diag-scroll { flex: 1; min-height: 200px; overflow: auto; }

        .diag-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .diag-scheduler-box {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            padding: 10px 14px;
            font-size: 13px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            flex-wrap: wrap;
        }

        /* Config Center - Two columns with horizontal space optimization */
        .config-scroll {
            flex: 1;
            min-height: 0;
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 12px;
            padding-bottom: 60px;
        }

        .config-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 14px 16px;
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .config-card-title {
            margin: 0 0 6px 0;
            font-size: 14px;
            font-weight: 750;
            color: var(--text-primary);
            display: flex;
            align-items: center;
            gap: 6px;
            background: transparent !important;
            border: none !important;
            padding: 0 !important;
        }

        .config-form-grid {
            display: grid;
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: 8px 20px;
        }

        .form-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            padding: 6px 0;
            border-bottom: 1px solid var(--border-subtle);
        }

        .form-row.full-width {
            grid-column: 1 / -1;
        }

        .form-label-box {
            display: flex;
            flex-direction: column;
            gap: 1px;
            flex: 1;
            min-width: 0;
        }

        .form-title {
            font-size: 13px;
            font-weight: 700;
            color: var(--text-primary);
        }

        .form-hint {
            font-size: 11px;
            color: var(--text-muted);
            line-height: 1.3;
        }

        .form-input-group {
            flex-shrink: 0;
            display: flex;
            align-items: center;
            gap: 6px;
        }

        .config-num-input {
            width: 120px !important;
            min-width: 120px !important;
            max-width: 120px !important;
            text-align: center !important;
            box-sizing: border-box !important;
        }

        .config-time-input {
            width: 80px !important;
            min-width: 80px !important;
            max-width: 80px !important;
            text-align: center !important;
            box-sizing: border-box !important;
        }

        .toggle-label {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            cursor: pointer;
            user-select: none;
            font-size: 13px;
            font-weight: 600;
        }

        .toggle-switch {
            position: relative;
            display: inline-block;
            width: 40px;
            height: 22px;
        }
        .toggle-switch input { opacity: 0; width: 0; height: 0; }
        .toggle-slider {
            position: absolute;
            cursor: pointer;
            top: 0; left: 0; right: 0; bottom: 0;
            background-color: var(--meter-bg);
            border-radius: 22px;
            transition: .25s;
        }
        .toggle-slider:before {
            position: absolute;
            content: "";
            height: 16px;
            width: 16px;
            left: 3px;
            bottom: 3px;
            background-color: white;
            border-radius: 50%;
            transition: .25s;
        }
        .toggle-switch input:checked + .toggle-slider { background-color: var(--accent-green); }
        .toggle-switch input:checked + .toggle-slider:before { transform: translateX(18px); }

        /* Floating Capsule Buttons at Bottom Right */
        .config-floating-actions {
            position: absolute;
            bottom: 16px;
            right: 20px;
            z-index: 40;
            display: flex;
            align-items: center;
            gap: 10px;
            background: transparent;
        }

        .config-floating-actions button {
            height: 38px;
            min-height: 38px;
            padding: 0 18px;
            border-radius: 999px;
            box-shadow: 0 4px 14px rgba(0, 0, 0, 0.08);
            font-size: 13px;
            font-weight: 700;
            transition: all 0.2s ease;
        }

        .config-floating-actions button:hover:not(:disabled) {
            box-shadow: 0 6px 20px rgba(0, 0, 0, 0.12);
            transform: translateY(-1px);
        }

        .config-floating-actions button:disabled {
            opacity: 0.45 !important;
            cursor: not-allowed !important;
            box-shadow: none !important;
            background: var(--bg-subtle) !important;
            color: var(--text-muted) !important;
            border: 1px solid var(--border-color) !important;
        }

        /* Modals */
        .modal-backdrop {
            position: fixed;
            inset: 0;
            display: grid;
            place-items: center;
            background: var(--bg-overlay);
            padding: 20px;
            z-index: 100;
            animation: modalBgIn 200ms ease forwards;
        }

        .modal {
            width: min(720px, 100%);
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 14px;
            padding: 20px;
            box-shadow: var(--shadow-lg);
            display: flex;
            flex-direction: column;
            gap: 14px;
            max-height: 85vh;
            animation: modalIn 260ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
        }

        .modal-body {
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 8px;
            padding-right: 4px;
            max-height: 60vh;
        }

        .diff-card {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            padding: 10px 12px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            flex-wrap: wrap;
        }

        .diff-value-box {
            display: flex;
            align-items: center;
            gap: 8px;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-weight: 700;
            font-size: 13px;
        }

        .diff-from {
            padding: 2px 6px;
            border-radius: 5px;
            background: var(--diff-from-bg);
            color: var(--diff-from-text);
        }

        .diff-to {
            padding: 2px 6px;
            border-radius: 5px;
            background: var(--diff-to-bg);
            color: var(--diff-to-text);
        }

        .confirm-title {
            margin: 0;
            font-size: 17px;
            font-weight: 700;
            display: flex;
            align-items: center;
            gap: 6px;
        }

        .confirm-message {
            font-size: 13px;
            color: var(--text-secondary);
            margin: 0;
            line-height: 1.5;
        }

        .confirm-actions {
            display: flex;
            justify-content: flex-end;
            gap: 8px;
            margin-top: 6px;
        }

        /* Toast Notifications */
        .toast-root {
            position: fixed;
            top: 70px;
            right: 20px;
            z-index: 200;
            display: flex;
            flex-direction: column;
            gap: 8px;
            max-width: 360px;
        }

        .toast {
            border-radius: 8px;
            padding: 10px 14px;
            font-size: 13px;
            font-weight: 600;
            box-shadow: var(--shadow-md);
            border-left: 4px solid;
            background: var(--bg-card);
            color: var(--text-primary);
            animation: toastIn 240ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
        }
        .toast-success { border-color: var(--accent-green); }
        .toast-error   { border-color: var(--accent-red); }
        .toast-info    { border-color: var(--accent-blue); }
        .toast.toast-exit { animation: toastOut 200ms ease forwards; }

        @keyframes toastIn {
            from { transform: translateX(30px) scale(0.95); opacity: 0; }
            to   { transform: translateX(0) scale(1); opacity: 1; }
        }
        @keyframes toastOut {
            from { opacity: 1; transform: translateY(0); }
            to   { opacity: 0; transform: translateY(-8px); }
        }
        @keyframes modalBgIn {
            from { opacity: 0; }
            to   { opacity: 1; }
        }
        @keyframes modalIn {
            from { transform: translateY(12px) scale(0.96); opacity: 0; }
            to   { transform: translateY(0) scale(1); opacity: 1; }
        }

        [hidden] { display: none !important; }

        @media (max-width: 960px) {
            .summary-grid { grid-template-columns: repeat(2, 1fr); }
            .config-form-grid { grid-template-columns: 1fr; }
        }

        @media (max-width: 600px) {
            .container { padding: 10px; }
            .summary-grid { grid-template-columns: 1fr; }
            .topbar { flex-direction: column; align-items: flex-start; }
            .topbar-actions { width: 100%; justify-content: space-between; margin-right: 0; }
        }
`
