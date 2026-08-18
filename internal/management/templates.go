package management

// StatusHTML is the self-contained, zero-external-CDN dashboard for Antigravity Priority.
// It complies with strict CSP and CPA host dual-theme standards.
const StatusHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Antigravity Priority</title>
    <style>
        :root {
            color-scheme: light dark;
            --bg-primary: #ffffff;
            --bg-secondary: #ffffff;
            --bg-tertiary: #f6f6f6;
            --bg-surface: #ffffff;
            --bg-card: #ffffff;
            --bg-subtle: #f6f6f6;
            --bg-hover: #f0f0f0;
            --bg-overlay: rgba(45, 42, 38, 0.45);
            --border-color: #e5e5e5;
            --border-subtle: #f0eee9;
            --border-focus: #8b8680;
            --text-primary: #2d2a26;
            --text-secondary: #6d6760;
            --text-muted: #a29c95;
            --text-inverse: #ffffff;
            --accent-blue: #2563eb;
            --accent-blue-hover: #1d4ed8;
            --accent-blue-subtle: #eff6ff;
            --accent-blue-text: #1d4ed8;
            --accent-green: #10b981;
            --accent-green-subtle: #ecfdf5;
            --accent-green-text: #047857;
            --accent-yellow: #d97706;
            --accent-yellow-subtle: #fffbeb;
            --accent-yellow-text: #b45309;
            --accent-red: #c65746;
            --accent-red-subtle: #fef2f2;
            --accent-red-text: #991b1b;
            --accent-purple: #7c3aed;
            --accent-purple-subtle: #faf5ff;
            --accent-purple-text: #6d28d9;
            --meter-bg: #e5e5e5;
            --meter-fill: #10b981;
            --meter-warn: #e0aa14;
            --meter-danger: #c65746;
            --badge-boost-bg: #fdf4ff;
            --badge-boost-border: #f0abfc;
            --badge-boost-text: #a21caf;
            --diff-from-bg: #fee2e2;
            --diff-from-text: #991b1b;
            --diff-to-bg: #dcfce7;
            --diff-to-text: #166534;
            --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.08), 0 2px 4px -2px rgba(0, 0, 0, 0.05);
            --shadow-lg: 0 10px 18px -3px rgba(0, 0, 0, 0.08), 0 4px 6px -4px rgba(0, 0, 0, 0.04);
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
            --bg-primary: #ffffff;
            --bg-secondary: #f8fafc;
            --bg-tertiary: #f1f5f9;
            --bg-surface: #ffffff;
            --bg-card: #ffffff;
            --bg-subtle: #f8fafc;
            --bg-hover: #f1f5f9;
            --bg-overlay: rgba(15, 23, 42, 0.45);
            --border-color: #e2e8f0;
            --border-subtle: #f1f5f9;
            --border-focus: #3b82f6;
            --text-primary: #0f172a;
            --text-secondary: #475569;
            --text-muted: #64748b;
            --text-inverse: #ffffff;
            --accent-blue: #2563eb;
            --accent-blue-hover: #1d4ed8;
            --accent-blue-subtle: #eff6ff;
            --accent-blue-text: #1d4ed8;
            --meter-bg: #e2e8f0;
            --meter-fill: #2563eb;
            --meter-warn: #d97706;
            --meter-danger: #dc2626;
        }

        * { box-sizing: border-box; }
        body {
            margin: 0;
            padding: 20px;
            background: var(--bg-primary);
            color: var(--text-primary);
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.5;
        }

        .container {
            width: 100%;
            max-width: 1200px;
            margin: 0 auto;
            background: transparent;
            border: 0;
            border-radius: 0;
            padding: 0;
            box-shadow: none;
        }

        .topbar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
            flex-wrap: wrap;
            margin-bottom: 20px;
            padding-bottom: 16px;
            border-bottom: 1px solid var(--border-subtle);
        }

        .brand-zone {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        h1 {
            margin: 0;
            font-size: 22px;
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
            gap: 10px;
            flex-wrap: wrap;
            margin-right: 160px;
        }

        .tabs {
            display: flex;
            gap: 6px;
            padding: 4px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 12px;
            margin-bottom: 20px;
        }

        .tab {
            flex: 1;
            min-height: 38px;
            background: transparent;
            color: var(--text-secondary);
            border: 0;
            border-radius: 8px;
            padding: 8px 16px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.15s ease;
        }

        .tab.active {
            background: var(--bg-card);
            color: var(--text-primary);
            box-shadow: var(--shadow-sm);
        }

        .card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 14px;
            padding: 20px;
        }

        .summary-grid {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr));
            gap: 16px;
            margin-bottom: 20px;
        }

        .kpi-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 16px;
            display: flex;
            flex-direction: column;
            gap: 6px;
        }

        .kpi-title {
            margin: 0;
            font-size: 13px;
            font-weight: 600;
            color: var(--text-muted);
        }

        .kpi-value {
            font-size: 26px;
            font-weight: 800;
            letter-spacing: -0.03em;
            color: var(--text-primary);
        }

        .kpi-desc {
            font-size: 12px;
            color: var(--text-secondary);
        }

        .control-bar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
            flex-wrap: wrap;
            padding: 16px 20px;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 14px;
            margin-bottom: 20px;
        }

        .control-group {
            display: flex;
            align-items: center;
            gap: 12px;
            flex-wrap: wrap;
        }

        label {
            font-size: 13px;
            font-weight: 650;
            color: var(--text-secondary);
        }

        .custom-select-wrapper {
            position: relative;
            user-select: none;
            min-width: 210px;
        }

        .custom-select-trigger {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            min-height: 38px;
            padding: 8px 14px;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 10px;
            color: var(--text-primary);
            font-size: 13px;
            font-weight: 600;
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
        }

        .custom-select-wrapper.open .custom-select-arrow {
            transform: rotate(180deg);
        }

        .custom-select-options {
            position: absolute;
            top: calc(100% + 6px);
            left: 0;
            right: 0;
            z-index: 50;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 6px;
            box-shadow: var(--shadow-lg);
            display: flex;
            flex-direction: column;
            gap: 4px;
        }

        .custom-select-option {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 8px 12px;
            border-radius: 8px;
            font-size: 13px;
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
            color: var(--primary-color, var(--accent-blue));
        }

        .custom-select-option.selected .custom-select-check {
            display: inline-flex;
        }

        input[type="password"], input[type="text"] {
            min-height: 38px;
            border: 1px solid var(--border-color);
            border-radius: 10px;
            padding: 8px 14px;
            font: inherit;
            background: var(--bg-card);
            color: var(--text-primary);
            width: 100%;
        }

        button {
            min-height: 38px;
            border-radius: 10px;
            border: 1px solid transparent;
            padding: 8px 16px;
            font: inherit;
            font-size: 13px;
            font-weight: 650;
            cursor: pointer;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 6px;
            transition: all 0.15s ease;
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

        .btn-success {
            background: var(--accent-green);
            color: var(--text-inverse);
        }

        .btn-danger {
            background: var(--accent-red-subtle);
            color: var(--accent-red-text);
            border-color: var(--accent-red);
        }

        button:disabled {
            opacity: 0.6;
            cursor: not-allowed !important;
        }

        .badge {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 3px 8px;
            border-radius: 6px;
            font-size: 12px;
            font-weight: 700;
            line-height: 1.2;
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

        .meter-container {
            display: flex;
            flex-direction: column;
            gap: 4px;
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
            height: 7px;
            background: var(--meter-bg);
            border-radius: 999px;
            overflow: hidden;
            position: relative;
        }

        .meter-fill {
            height: 100%;
            border-radius: 999px;
            transition: width 0.3s ease, background-color 0.3s ease;
        }

        .meter-fill-healthy { background: var(--accent-green); }
        .meter-fill-warning { background: var(--meter-warn); }
        .meter-fill-danger  { background: var(--meter-danger); }

        .meter-countdown {
            font-family: SFMono-Regular, Consolas, "Liberation Mono", Menlo, monospace;
            font-size: 11px;
            color: var(--text-secondary);
        }

        .credentials-grid {
            display: grid;
            gap: 14px;
        }

        .cred-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 14px;
            padding: 18px 20px;
            display: grid;
            grid-template-columns: minmax(200px, 1.4fr) minmax(130px, 0.8fr) minmax(220px, 1.5fr) minmax(220px, 1.5fr) minmax(120px, 0.9fr);
            gap: 20px;
            align-items: center;
            box-shadow: var(--shadow-sm);
        }

        .cred-info {
            display: flex;
            flex-direction: column;
            gap: 6px;
            min-width: 0;
        }

        .cred-name {
            font-size: 14px;
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
            font-size: 12px;
            color: var(--text-muted);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .cred-metrics {
            display: flex;
            flex-direction: column;
            gap: 4px;
        }

        .metric-pill {
            display: inline-flex;
            align-items: center;
            justify-content: space-between;
            font-size: 12px;
            padding: 2px 6px;
            border-radius: 6px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            color: var(--text-secondary);
        }

        .metric-pill strong {
            color: var(--text-primary);
            margin-left: 6px;
        }

        .cred-priority {
            display: flex;
            flex-direction: column;
            align-items: flex-end;
            gap: 6px;
        }

        .priority-score {
            font-size: 20px;
            font-weight: 800;
            color: var(--accent-blue-text);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
        }

        .modal-backdrop {
            position: fixed;
            inset: 0;
            display: grid;
            place-items: center;
            background: var(--bg-overlay);
            padding: 20px;
            z-index: 100;
        }

        .modal {
            width: min(720px, 100%);
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 18px;
            padding: 24px;
            box-shadow: var(--shadow-lg);
            display: flex;
            flex-direction: column;
            gap: 16px;
            max-height: 85vh;
        }

        .modal-body {
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 12px;
            padding-right: 4px;
        }

        .diff-card {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 10px;
            padding: 12px 14px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            flex-wrap: wrap;
        }

        .diff-value-box {
            display: flex;
            align-items: center;
            gap: 8px;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-weight: 700;
        }

        .diff-from {
            padding: 2px 6px;
            border-radius: 6px;
            background: var(--diff-from-bg);
            color: var(--diff-from-text);
        }

        .diff-to {
            padding: 2px 6px;
            border-radius: 6px;
            background: var(--diff-to-bg);
            color: var(--diff-to-text);
        }

        .toast-root {
            position: fixed;
            top: 90px;
            right: 20px;
            z-index: 200;
            display: flex;
            flex-direction: column;
            gap: 8px;
            max-width: 360px;
        }

        .toast {
            border-radius: 10px;
            padding: 12px 16px;
            font-size: 13px;
            font-weight: 600;
            box-shadow: var(--shadow-md);
            border-left: 4px solid;
            background: var(--bg-card);
            color: var(--text-primary);
        }
        .toast-success { border-color: var(--accent-green); }
        .toast-error   { border-color: var(--accent-red); }
        .toast-info    { border-color: var(--accent-blue); }

        .empty-state {
            text-align: center;
            padding: 48px 20px;
            color: var(--text-muted);
            font-size: 14px;
        }

        .history-item {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 14px 18px;
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .history-head {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            flex-wrap: wrap;
        }

        .history-stats {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
        }

        pre.code-block {
            margin: 0;
            padding: 14px;
            border-radius: 10px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 12px;
            overflow: auto;
            color: var(--text-primary);
        }

        [hidden] { display: none !important; }

        @media (max-width: 960px) {
            .cred-card {
                grid-template-columns: 1fr;
                gap: 14px;
            }
            .summary-grid {
                grid-template-columns: repeat(2, 1fr);
            }
            .cred-priority {
                align-items: flex-start;
            }
        }

        @media (max-width: 600px) {
            body { padding: 12px; }
            .summary-grid { grid-template-columns: 1fr; }
            .topbar { flex-direction: column; align-items: flex-start; }
            .topbar-actions { width: 100%; justify-content: space-between; margin-right: 0; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header class="topbar">
            <div class="brand-zone">
                <h1>
                    <span data-i18n="title">Antigravity Priority</span>
                    <span class="version-badge">v1.0.1</span>
                </h1>
            </div>
            <div class="topbar-actions">
                <div id="toastRoot" class="toast-root" aria-live="polite"></div>
                <button type="button" class="btn-secondary" onclick="openKeyModal()" aria-label="Set Management Key">
                    <span>🔑 <span data-i18n="btnKey">密钥</span></span>
                </button>
                <button type="button" class="btn-secondary" onclick="toggleLanguage()" aria-label="Toggle Language">
                    <span id="langLabel">EN / 中文</span>
                </button>
            </div>
        </header>

        <nav class="tabs" aria-label="Dashboard Navigation">
            <button type="button" class="tab active" data-tab="overview" onclick="switchTab('overview')" data-i18n="tabOverview">概览与仪表盘</button>
            <button type="button" class="tab" data-tab="history" onclick="switchTab('history')" data-i18n="tabHistory">执行历史</button>
            <button type="button" class="tab" data-tab="diagnostics" onclick="switchTab('diagnostics')" data-i18n="tabDiagnostics">系统诊断</button>
            <button type="button" class="tab" data-tab="help" onclick="switchTab('help')" data-i18n="tabHelp">使用帮助</button>
        </nav>

        <section id="panelOverview">
            <div class="summary-grid">
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiTotal">总凭证数</p>
                    <div id="valTotalCreds" class="kpi-value">0</div>
                    <div id="valTotalDesc" class="kpi-desc">0 活跃</div>
                </div>
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiBoosted">🚀 动态 Boost</p>
                    <div id="valBoosted" class="kpi-value">0</div>
                    <div class="kpi-desc" data-i18n="kpiBoostedDesc">高配额高充裕度</div>
                </div>
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiDepleted">耗尽 / 降级</p>
                    <div id="valDepleted" class="kpi-value">0</div>
                    <div class="kpi-desc" data-i18n="kpiDepletedDesc">等待窗口重置</div>
                </div>
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiLastAudit">最新调度审计</p>
                    <div id="valLastAudit" class="kpi-value" style="font-size:16px; word-break:break-word;">-</div>
                    <div id="valNextProbe" class="kpi-desc">-</div>
                </div>
            </div>

            <div class="control-bar">
                <div class="control-group">
                    <label for="modelGroupSelect" data-i18n="labelModelGroup">模型组：</label>
                    <select id="modelGroupSelect" hidden>
                        <option value="gemini" data-i18n="optGemini">Gemini 模型</option>
                        <option value="claude_gpt" data-i18n="optClaudeGPT">Claude 与 GPT 模型</option>
                    </select>
                    <div id="customModelGroupSelect" class="custom-select-wrapper">
                        <button type="button" class="custom-select-trigger" onclick="toggleCustomSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                            <span id="customSelectLabel" data-i18n="optGemini">Gemini 模型</span>
                            <span class="custom-select-arrow">
                                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                            </span>
                        </button>
                        <div id="customSelectMenu" class="custom-select-options" hidden role="listbox">
                            <div class="custom-select-option selected" data-value="gemini" onclick="selectModelGroup('gemini', event)" role="option" aria-selected="true">
                                <span data-i18n="optGemini">Gemini 模型</span>
                                <span class="custom-select-check">
                                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg>
                                </span>
                            </div>
                            <div class="custom-select-option" data-value="claude_gpt" onclick="selectModelGroup('claude_gpt', event)" role="option" aria-selected="false">
                                <span data-i18n="optClaudeGPT">Claude 与 GPT 模型</span>
                                <span class="custom-select-check">
                                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg>
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="control-group">
                    <button type="button" class="btn-secondary" onclick="refreshDashboard()" id="btnRefresh" data-i18n="btnRefresh">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
                        <span>刷新</span>
                    </button>
                    <button type="button" class="btn-secondary" onclick="triggerRun('dry-run')" id="btnDryRun" data-i18n="btnDryRun">
                        <span>🔍 试运行 (Dry-Run)</span>
                    </button>
                    <button type="button" class="btn-primary" onclick="triggerRun('apply')" id="btnApply" data-i18n="btnApply">
                        <span>⚡ 立即写回 (Apply)</span>
                    </button>
                    <button type="button" class="btn-danger" onclick="triggerReset()" id="btnReset" data-i18n="btnReset">
                        <span>🔄 重置默认 (Reset)</span>
                    </button>
                </div>
            </div>

            <div id="credentialsContainer" class="credentials-grid">
                <div class="empty-state" data-i18n="loading">加载中...</div>
            </div>
        </section>

        <section id="panelHistory" hidden>
            <div class="card">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
                    <h2 style="margin:0; font-size:16px;" data-i18n="historyTitle">最近执行记录 (最近 10 次)</h2>
                    <button type="button" class="btn-secondary" onclick="fetchDiagnostics()" data-i18n="btnRefresh">刷新</button>
                </div>
                <div id="historyList" style="display:grid; gap:12px;">
                    <div class="empty-state" data-i18n="noHistory">暂无执行历史</div>
                </div>
            </div>
        </section>

        <section id="panelDiagnostics" hidden>
            <div class="card" style="display:grid; gap:16px;">
                <h2 style="margin:0; font-size:16px;" data-i18n="diagnosticsTitle">系统运行诊断与调度状态</h2>
                <div id="schedulerInfo" class="diff-card" style="font-size:13px;"></div>
                <pre id="rawDiagnostics" class="code-block"></pre>
            </div>
        </section>

        <section id="panelHelp" hidden>
            <div class="card" style="line-height:1.6; font-size:14px; color:var(--text-secondary);">
                <h2 style="margin:0 0 12px; font-size:16px; color:var(--text-primary);" data-i18n="helpTitle">Antigravity Priority 原理与规则说明</h2>
                <p data-i18n="helpP1">本插件专为 CLIProxyAPI 的 Google Antigravity 设计，实现双窗口自适应优先级调度与智能写回：</p>
                <ul style="margin-top: 12px; padding-left: 20px; display: grid; gap: 8px;">
                    <li data-i18n="help5hDesc"><strong>5小时短窗口 (5h)</strong>：实时监控高频请求配额与重置时间，防止短期突发请求触发 429 速率限制。</li>
                    <li data-i18n="help7dDesc"><strong>7天长窗口 (7d)</strong>：跟踪周配额剩余量及重置进度，避免单张凭证在周期前半段过早耗尽。</li>
                    <li data-i18n="helpBurnDesc"><strong>自适应燃烧率 (C_cycle)</strong>：在线学习估算周期消耗速率，实时动态计算保底所需剩余额度。</li>
                    <li data-i18n="helpUrgencyDesc"><strong>周紧迫度指标 (Urgency)</strong>：计算配额紧迫指数，优先调度充裕健康的凭证。</li>
                    <li data-i18n="helpBoostDesc"><strong>🚀 动态 Boost 机制</strong>：对双窗口余量充裕且周期健康的凭证自动赋予第一梯队超高优先级（900-999）。</li>
                </ul>
                <h3 style="margin:16px 0 8px; font-size:15px; color:var(--text-primary);" data-i18n="helpConfigTitle">配置字段详细说明</h3>
                <ul style="padding-left: 20px; display: grid; gap: 6px;">
                    <li data-i18n="helpConfigAutoApply"><code>auto_apply</code>: 是否开启定时自动优先级排序并写回宿主凭证。</li>
                    <li data-i18n="helpConfigInterval"><code>interval</code>: 自动探测与调度周期（如 <code>15m</code>、<code>30m</code>）。</li>
                    <li data-i18n="helpConfigGroup"><code>antigravity_model_group</code>: 配额主控模型组，可选 <code>gemini</code> 或 <code>claude_gpt</code>。</li>
                    <li data-i18n="helpConfigConcurrency"><code>max_concurrency</code>: 探测并发 HTTP 请求数上限。</li>
                    <li data-i18n="helpConfigMinChange"><code>min_change</code>: 优先级写回变动最小阈值。</li>
                    <li data-i18n="helpConfigCachePath"><code>state_cache_path</code>: 状态缓存与自适应学习率持久化文件路径。</li>
                    <li data-i18n="helpConfigRulesEnabled"><code>priority_rules.enabled</code>: 是否开启自定义优先级规则。</li>
                    <li data-i18n="helpConfigBoostStart"><code>priority_rules.boost_start_priority</code>: 动态提权区间的起始基准优先级。</li>
                    <li data-i18n="helpConfigNormalStart"><code>priority_rules.normal_start_priority</code>: 常规健康凭证的起始基准优先级。</li>
                </ul>
            </div>
        </section>
    </div>

    <!-- Management Key Modal -->
    <div id="keyModal" class="modal-backdrop" hidden>
        <div class="modal" role="dialog" aria-modal="true" aria-labelledby="keyModalTitle" style="max-width: 480px;">
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <h2 id="keyModalTitle" style="margin:0; font-size:18px;" data-i18n="keyModalTitle">CPA 管理密钥认证</h2>
                <button type="button" class="btn-secondary" onclick="closeKeyModal()" style="min-height:30px; padding:4px 10px;">✕</button>
            </div>
            <p style="font-size:13px; color:var(--text-secondary); margin:0; line-height:1.6;" data-i18n="keyModalDesc">CPA 原生管理界面下请输入 config.yaml 中的 Management Key；若在 CPA-Plus 等增强面板中，请输入 CPA-Plus 登录密码（通常为 cpamp_... 格式）。</p>
            <div>
                <input type="password" id="manualKeyInput" placeholder="Management Key" autocomplete="off">
            </div>
            <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:8px;">
                <button type="button" class="btn-secondary" onclick="closeKeyModal()" data-i18n="btnClose">取消</button>
                <button type="button" class="btn-primary" onclick="saveKeyAndRefresh()" data-i18n="btnSaveKey">保存并验证</button>
            </div>
        </div>
    </div>

    <!-- Diff & Run Result Modal -->
    <div id="diffModal" class="modal-backdrop" hidden>
        <div class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <h2 id="modalTitle" style="margin:0; font-size:18px;">调度变更预览</h2>
                <button type="button" class="btn-secondary" onclick="closeModal()" style="min-height:30px; padding:4px 10px;">✕</button>
            </div>
            <div id="modalSummary" style="font-size:13px; color:var(--text-secondary);"></div>
            <div id="modalDiffList" class="modal-body"></div>
            <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:8px;">
                <button type="button" class="btn-secondary" onclick="closeModal()" data-i18n="btnClose">关闭</button>
                <button type="button" class="btn-primary" id="btnModalApply" onclick="applyFromModal()" data-i18n="btnApplyNow">立即写回</button>
            </div>
        </div>
    </div>

    <script>
        const BASE_PATH = "/v0/management/plugins/antigravity-priority";
        const SNAPSHOT_PATH = BASE_PATH + "/snapshot/latest";
        const DIAGNOSTICS_PATH = BASE_PATH + "/diagnostics";
        const RUN_PATH = BASE_PATH + "/run";
        const RESET_PATH = BASE_PATH + "/reset";

        const I18N = {
            "zh-CN": {
                title: "Antigravity 优先级管理",
                tabOverview: "概览与仪表盘",
                tabHistory: "执行历史",
                tabDiagnostics: "系统诊断",
                tabHelp: "使用帮助",
                kpiTotal: "总凭证数",
                kpiBoosted: "🚀 动态 Boost",
                kpiBoostedDesc: "高配额充裕候选",
                kpiDepleted: "耗尽 / 降级",
                kpiDepletedDesc: "等待窗口重置",
                kpiLastAudit: "最新调度审计",
                labelModelGroup: "模型组：",
                optGemini: "Gemini 模型",
                optClaudeGPT: "Claude 与 GPT 模型",
                btnRefresh: "刷新",
                btnKey: "密钥",
                btnDryRun: "🔍 试运行 (Dry-Run)",
                btnApply: "⚡ 立即写回 (Apply)",
                btnReset: "🔄 重置默认 (Reset)",
                confirmReset: "确定要将所有 Antigravity 凭证的优先级重置为默认未设置状态吗？",
                resetSuccess: "所有凭证优先级已重置为默认未设置状态",
                loading: "正在加载凭证与配额状态...",
                noCreds: "未发现 Antigravity 凭证",
                historyTitle: "最近执行记录 (最近 10 次)",
                noHistory: "暂无执行历史记录",
                diagnosticsTitle: "系统运行诊断与调度状态",
                helpTitle: "Antigravity Priority 原理与规则说明",
                helpP1: "本插件专为 CLIProxyAPI 的 Google Antigravity 设计，实现双窗口自适应优先级调度与智能写回：",
                help5hDesc: "<strong>5小时短窗口 (5h)</strong>：实时监控高频请求配额与重置时间，防止短期突发请求触发 429 速率限制。",
                help7dDesc: "<strong>7天长窗口 (7d)</strong>：跟踪周配额剩余量及重置进度，避免单张凭证在周期前半段过早耗尽。",
                helpBurnDesc: "<strong>自适应燃烧率 (C_cycle)</strong>：在线学习估算周期消耗速率，实时动态计算保底所需剩余额度。",
                helpUrgencyDesc: "<strong>周紧迫度指标 (Urgency)</strong>：计算配额紧迫指数，优先调度充裕健康的凭证。",
                helpBoostDesc: "<strong>🚀 动态 Boost 机制</strong>：对双窗口余量充裕且周期健康的凭证自动赋予第一梯队超高优先级（900-999）。",
                helpConfigTitle: "配置字段详细说明",
                helpConfigAutoApply: "<code>auto_apply</code>: 是否开启定时自动优先级排序并写回宿主凭证（默认 <code>false</code>，关闭时仅在管理页手动写回）。",
                helpConfigInterval: "<code>interval</code>: 自动探测与调度周期，例如 <code>15m</code>、<code>30m</code>、<code>1h</code>（默认 <code>15m</code>）。",
                helpConfigGroup: "<code>antigravity_model_group</code>: 配额主控模型组，可选 <code>gemini</code> 或 <code>claude_gpt</code>（默认 <code>gemini</code>）。",
                helpConfigConcurrency: "<code>max_concurrency</code>: 探测并发 HTTP 请求数上限（默认 <code>6</code>）。",
                helpConfigMinChange: "<code>min_change</code>: 优先级写回变动最小阈值，变动小于该值则跳过写入以减少 IO（默认 <code>1</code>）。",
                helpConfigCachePath: "<code>state_cache_path</code>: 状态缓存与自适应学习率持久化文件路径（默认 <code>data/antigravity-priority-cache.json</code>）。",
                helpConfigRulesEnabled: "<code>priority_rules.enabled</code>: 是否启用自定义优先级规则（默认 <code>true</code>；关闭时强制使用内置默认值）。",
                helpConfigBoostStart: "<code>priority_rules.boost_start_priority</code>: 动态提权区间的起始基准优先级（默认 <code>999</code>）。",
                helpConfigNormalStart: "<code>priority_rules.normal_start_priority</code>: 常规健康凭证的起始基准优先级（默认 <code>100</code>）。",
                btnClose: "关闭",
                btnApplyNow: "立即写回",
                previewTitle: "试运行计划预览 (Dry-Run)",
                applyTitle: "写回执行结果 (Apply)",
                noChanges: "本次调度无优先级或禁用状态变化",
                statusActive: "正常活跃",
                statusBoosted: "🚀 动态 Boost",
                statusWeeklyDepleted: "周额度耗尽",
                statusShortDepleted: "5h短窗口耗尽",
                statusFailed: "探测失败",
                shortWindow: "5h 短窗口",
                longWindow: "7d 周窗口",
                resetIn: "重置倒计时",
                priority: "优先级",
                urgencyLabel: "紧迫度",
                burnLabel: "燃烧率",
                running: "执行中...",
                keyModalTitle: "CPA 管理密钥认证",
                keyModalDesc: "CPA 原生管理界面下请输入 config.yaml 中的 Management Key；若在 CPA-Plus 等增强面板中，请输入 CPA-Plus 登录密码（通常为 cpamp_... 格式）。",
                btnSaveKey: "保存并验证"
            },
            "en-US": {
                title: "Antigravity Priority",
                tabOverview: "Overview & Meters",
                tabHistory: "Run History",
                tabDiagnostics: "Diagnostics",
                tabHelp: "Help & Rules",
                kpiTotal: "Total Credentials",
                kpiBoosted: "🚀 Boosted",
                kpiBoostedDesc: "High quota abundance",
                kpiDepleted: "Depleted / Down",
                kpiDepletedDesc: "Awaiting window reset",
                kpiLastAudit: "Latest Audit",
                labelModelGroup: "Model Group:",
                optGemini: "Gemini Models",
                optClaudeGPT: "Claude & GPT Models",
                btnRefresh: "Refresh",
                btnKey: "Key",
                btnDryRun: "🔍 Dry-Run",
                btnApply: "⚡ Apply Now",
                btnReset: "🔄 Reset Default",
                confirmReset: "Are you sure you want to reset all Antigravity credential priorities to default unset state?",
                resetSuccess: "All credential priorities have been reset to default unset state",
                loading: "Loading credentials & quota...",
                noCreds: "No Antigravity credentials found",
                historyTitle: "Execution History (Last 10)",
                noHistory: "No execution history yet",
                diagnosticsTitle: "System Diagnostics & Scheduler",
                helpTitle: "Antigravity Priority Mechanics & Rules",
                helpP1: "Tailored for Google Antigravity in CLIProxyAPI with double-window adaptive scheduling and intelligent write-back:",
                help5hDesc: "<strong>5-Hour Short Window (5h)</strong>: Monitors burst quota and reset countdowns to prevent 429 rate limits.",
                help7dDesc: "<strong>7-Day Weekly Window (7d)</strong>: Tracks weekly remaining balance and reset progress to prevent early depletion.",
                helpBurnDesc: "<strong>Adaptive Burn Rate (C_cycle)</strong>: Incrementally learns real consumption rates to calculate required burn horizons.",
                helpUrgencyDesc: "<strong>Weekly Urgency Index</strong>: Measures unit-time quota pressure to smoothly rotate accounts throughout the cycle.",
                helpBoostDesc: "<strong>🚀 Dynamic Boost Tier</strong>: Elevates credentials with large remaining balances to top priority (900-999) before weekly reset.",
                helpConfigTitle: "Configuration Field Details",
                helpConfigAutoApply: "<code>auto_apply</code>: Enable periodic scheduled priority sorting and write-back (default <code>false</code>; manual runs remain available).",
                helpConfigInterval: "<code>interval</code>: Automated probing and scheduling interval, e.g. <code>15m</code>, <code>30m</code>, <code>1h</code> (default <code>15m</code>).",
                helpConfigGroup: "<code>antigravity_model_group</code>: Primary quota model group, options: <code>gemini</code> or <code>claude_gpt</code> (default <code>gemini</code>).",
                helpConfigConcurrency: "<code>max_concurrency</code>: Maximum concurrent quota probe HTTP requests (default <code>6</code>).",
                helpConfigMinChange: "<code>min_change</code>: Minimum priority delta threshold to trigger write-back, skipping minor changes to reduce IO (default <code>1</code>).",
                helpConfigCachePath: "<code>state_cache_path</code>: Path to persist state cache and learned metrics (default <code>data/antigravity-priority-cache.json</code>).",
                helpConfigRulesEnabled: "<code>priority_rules.enabled</code>: Enable custom priority rules (default <code>true</code>; when false, built-in defaults are used).",
                helpConfigBoostStart: "<code>priority_rules.boost_start_priority</code>: Starting priority base for boosted credentials (default <code>999</code>).",
                helpConfigNormalStart: "<code>priority_rules.normal_start_priority</code>: Starting priority base for regular healthy credentials (default <code>100</code>).",
                btnClose: "Close",
                btnApplyNow: "Apply Changes",
                previewTitle: "Dry-Run Plan Preview",
                applyTitle: "Apply Execution Result",
                noChanges: "No priority or status changes required",
                statusActive: "Active",
                statusBoosted: "🚀 Boosted",
                statusWeeklyDepleted: "Weekly Depleted",
                statusShortDepleted: "5h Depleted",
                statusFailed: "Probe Failed",
                shortWindow: "5h Window",
                longWindow: "7d Window",
                resetIn: "Resets in",
                priority: "Priority",
                urgencyLabel: "Urgency",
                burnLabel: "Burn Rate",
                running: "Running...",
                keyModalTitle: "CPA Management Key",
                keyModalDesc: "For native CPA Web UI, enter the Management Key from config.yaml; for CPA-Plus enhanced panels, enter your CPA-Plus login password (e.g. cpamp_... format).",
                btnSaveKey: "Save & Verify"
            }
        };

        let currentLang = "zh-CN";
        let latestSnapshot = null;
        let latestDiagnostics = null;
        let activeTab = "overview";
        let countdownInterval = null;

        function getManagementKey() {
            try {
                const keys = ['management_key', 'management-key', 'managementKey', 'cpa_management_key', 'admin_key', 'key'];
                for (const k of keys) {
                    const v = localStorage.getItem(k) || sessionStorage.getItem(k);
                    if (v && v.trim()) return v.trim();
                }
            } catch (_) {}

            try {
                if (window.parent && window.parent !== window) {
                    const keys = ['management_key', 'management-key', 'managementKey', 'cpa_management_key', 'admin_key', 'key'];
                    for (const k of keys) {
                        const v = window.parent.localStorage.getItem(k) || window.parent.sessionStorage.getItem(k);
                        if (v && v.trim()) return v.trim();
                    }
                    const parentParams = new URLSearchParams(window.parent.location.search);
                    const pKey = parentParams.get('key') || parentParams.get('management_key') || parentParams.get('management-key');
                    if (pKey && pKey.trim()) return pKey.trim();
                }
            } catch (_) {}

            try {
                const params = new URLSearchParams(window.location.search);
                const qKey = params.get('key') || params.get('management_key') || params.get('management-key');
                if (qKey && qKey.trim()) return qKey.trim();
            } catch (_) {}

            return "";
        }

        function setSavedKey(key) {
            try {
                if (key) {
                    localStorage.setItem('management_key', key.trim());
                    sessionStorage.setItem('management_key', key.trim());
                } else {
                    localStorage.removeItem('management_key');
                    sessionStorage.removeItem('management_key');
                }
            } catch (_) {}
        }

        function openKeyModal() {
            const input = document.getElementById("manualKeyInput");
            if (input) input.value = getManagementKey();
            const modal = document.getElementById("keyModal");
            if (modal) modal.hidden = false;
        }

        function closeKeyModal() {
            const modal = document.getElementById("keyModal");
            if (modal) modal.hidden = true;
        }

        function saveKeyAndRefresh() {
            const input = document.getElementById("manualKeyInput");
            if (input) {
                setSavedKey(input.value);
            }
            closeKeyModal();
            refreshDashboard();
        }

        function syncThemeFromParent() {
            try {
                if (window.parent && window.parent !== window && window.parent.document && window.parent.document.documentElement) {
                    const pDoc = window.parent.document.documentElement;
                    const pBody = window.parent.document.body;

                    const pTheme = pDoc.getAttribute("data-theme") || (pBody && pBody.getAttribute("data-theme"));
                    if (pTheme) {
                        document.documentElement.setAttribute("data-theme", pTheme);
                    } else {
                        document.documentElement.removeAttribute("data-theme");
                    }

                    const isDark = pDoc.classList.contains("dark") || (pBody && pBody.classList.contains("dark")) || pTheme === "dark";
                    if (isDark) {
                        document.documentElement.setAttribute("data-theme", "dark");
                    }

                    const parentStyle = window.parent.getComputedStyle(pDoc);

                    const cpaVarNames = [
                        '--bg-primary', '--bg-secondary', '--bg-tertiary', '--bg-quinary', '--bg-hover',
                        '--text-primary', '--text-secondary', '--text-tertiary', '--text-muted',
                        '--border-color', '--border-primary', '--border-secondary', '--border-hover',
                        '--primary-color', '--primary-hover', '--primary-active', '--primary-contrast',
                        '--success-color', '--warning-color', '--error-color', '--danger-color',
                        '--amber-color', '--quota-medium-color', '--floating-surface'
                    ];

                    for (const name of cpaVarNames) {
                        const val = parentStyle.getPropertyValue(name);
                        if (val && val.trim()) {
                            document.documentElement.style.setProperty(name, val.trim());
                        }
                    }

                    const sec = parentStyle.getPropertyValue('--bg-secondary') || parentStyle.getPropertyValue('--bg-primary');
                    const tert = parentStyle.getPropertyValue('--bg-tertiary');
                    if (sec && sec.trim()) {
                        document.documentElement.style.setProperty('--bg-surface', sec.trim());
                        document.documentElement.style.setProperty('--bg-card', sec.trim());
                    }
                    if (tert && tert.trim()) {
                        document.documentElement.style.setProperty('--bg-subtle', tert.trim());
                        document.documentElement.style.setProperty('--meter-bg', tert.trim());
                    }
                }
            } catch (_) {}
        }

        syncThemeFromParent();
        try {
            if (window.parent && window.parent !== window && window.parent.document) {
                const observer = new MutationObserver(function() {
                    syncThemeFromParent();
                });
                observer.observe(window.parent.document.documentElement, {
                    attributes: true,
                    attributeFilter: ["data-theme", "class", "style"]
                });
                if (window.parent.document.body) {
                    observer.observe(window.parent.document.body, {
                        attributes: true,
                        attributeFilter: ["data-theme", "class", "style"]
                    });
                }
            }
        } catch (_) {}

        function t(key) {
            return (I18N[currentLang] && I18N[currentLang][key]) || I18N["zh-CN"][key] || key;
        }

        function toggleLanguage() {
            currentLang = currentLang === "zh-CN" ? "en-US" : "zh-CN";
            applyLanguage();
            renderDashboard();
            renderHistory();
            renderDiagnostics();
        }

        function applyLanguage() {
            document.documentElement.lang = currentLang;
            document.querySelectorAll("[data-i18n]").forEach(el => {
                const key = el.getAttribute("data-i18n");
                if (key && I18N[currentLang] && I18N[currentLang][key]) {
                    el.innerHTML = I18N[currentLang][key];
                }
            });
            const langLabel = document.getElementById("langLabel");
            if (langLabel) langLabel.textContent = currentLang === "zh-CN" ? "EN / 中文" : "中文 / EN";
            updateCustomSelectDisplay();
        }

        function toggleCustomSelect(event) {
            if (event) event.stopPropagation();
            const wrapper = document.getElementById("customModelGroupSelect");
            const menu = document.getElementById("customSelectMenu");
            if (!wrapper || !menu) return;
            const trigger = wrapper.querySelector(".custom-select-trigger");
            const isOpen = !menu.hidden;

            menu.hidden = isOpen;
            wrapper.classList.toggle("open", !isOpen);
            if (trigger) trigger.setAttribute("aria-expanded", String(!isOpen));
        }

        function closeCustomSelect() {
            const wrapper = document.getElementById("customModelGroupSelect");
            const menu = document.getElementById("customSelectMenu");
            if (wrapper && menu) {
                menu.hidden = true;
                wrapper.classList.remove("open");
                const trigger = wrapper.querySelector(".custom-select-trigger");
                if (trigger) trigger.setAttribute("aria-expanded", "false");
            }
        }

        function selectModelGroup(value, event) {
            if (event) event.stopPropagation();
            const select = document.getElementById("modelGroupSelect");
            if (select) select.value = value;

            document.querySelectorAll(".custom-select-option").forEach(opt => {
                const isSelected = opt.getAttribute("data-value") === value;
                opt.classList.toggle("selected", isSelected);
                opt.setAttribute("aria-selected", String(isSelected));
            });

            updateCustomSelectDisplay();
            closeCustomSelect();
        }

        function updateCustomSelectDisplay() {
            const select = document.getElementById("modelGroupSelect");
            const label = document.getElementById("customSelectLabel");
            if (select && label) {
                const selectedOpt = select.options[select.selectedIndex];
                if (selectedOpt) {
                    const key = selectedOpt.getAttribute("data-i18n");
                    label.textContent = key ? t(key) : selectedOpt.textContent;
                }
            }
        }

        document.addEventListener("click", () => {
            closeCustomSelect();
        });

        function switchTab(tabId) {
            activeTab = tabId;
            document.querySelectorAll(".tab").forEach(tab => {
                tab.classList.toggle("active", tab.dataset.tab === tabId);
            });
            document.getElementById("panelOverview").hidden = tabId !== "overview";
            document.getElementById("panelHistory").hidden = tabId !== "history";
            document.getElementById("panelDiagnostics").hidden = tabId !== "diagnostics";
            document.getElementById("panelHelp").hidden = tabId !== "help";
            if (tabId === "history") fetchDiagnostics();
            if (tabId === "diagnostics") fetchDiagnostics();
        }

        function getAuthHeader() {
            const headers = { "Content-Type": "application/json" };
            const key = getManagementKey();
            if (key) {
                headers["Authorization"] = "Bearer " + key;
                headers["X-Management-Key"] = key;
            }
            return headers;
        }

        async function apiFetch(path, options) {
            const resp = await fetch(path, {
                ...(options || {}),
                headers: { ...getAuthHeader(), ...((options && options.headers) || {}) }
            });
            if (resp.status === 401) {
                openKeyModal();
                throw new Error(currentLang === "zh-CN" ? "需要 CPA 管理密钥进行认证 (401 Unauthorized)" : "Management Key required (401 Unauthorized)");
            }
            const text = await resp.text();
            if (!resp.ok) {
                let errMessage = text || resp.statusText;
                try {
                    const parsed = JSON.parse(text);
                    if (parsed.error) errMessage = parsed.error;
                } catch (_) {}
                throw new Error(errMessage);
            }
            return text ? JSON.parse(text) : {};
        }

        async function fetchSnapshot() {
            try {
                const data = await apiFetch(SNAPSHOT_PATH);
                latestSnapshot = data;
                renderDashboard();
            } catch (err) {
                showToast(err.message, "error");
            }
        }

        async function fetchDiagnostics() {
            try {
                const data = await apiFetch(DIAGNOSTICS_PATH);
                latestDiagnostics = data;
                renderHistory();
                renderDiagnostics();
            } catch (err) {
                showToast(err.message, "error");
            }
        }

        async function refreshDashboard() {
            const btn = document.getElementById("btnRefresh");
            if (btn) btn.disabled = true;
            try {
                await Promise.all([fetchSnapshot(), fetchDiagnostics()]);
            } finally {
                if (btn) btn.disabled = false;
            }
        }

        async function triggerRun(mode) {
            if (mode === "apply") {
                const items = (latestSnapshot && latestSnapshot.items) || [];
                if (items.length === 0) {
                    showToast(currentLang === "zh-CN" ? "未发现有效凭证，无需执行写回" : "No credentials found to apply", "info");
                    return;
                }
                const changes = (latestSnapshot && latestSnapshot.changes) || [];
                if (changes.length === 0) {
                    showToast(currentLang === "zh-CN" ? "当前凭证状态与优先级已是最优，无需写回" : "All credentials in sync, no changes needed", "info");
                    return;
                }
            }

            const groupSelect = document.getElementById("modelGroupSelect");
            const group = groupSelect ? groupSelect.value : "gemini";
            const btnDry = document.getElementById("btnDryRun");
            const btnApp = document.getElementById("btnApply");

            if (btnDry) btnDry.disabled = true;
            if (btnApp) btnApp.disabled = true;

            try {
                const path = RUN_PATH + "?mode=" + encodeURIComponent(mode) + "&antigravity_model_group=" + encodeURIComponent(group);
                const result = await apiFetch(path, { method: "POST" });
                const items = (result && result.snapshot && result.snapshot.items) || (result && result.items) || [];
                if (items.length === 0) {
                    showToast(currentLang === "zh-CN" ? "未发现有效 Antigravity 凭证" : "No Antigravity credentials found", "info");
                } else {
                    showModal(mode, result);
                }
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btnDry) btnDry.disabled = false;
                if (btnApp) btnApp.disabled = false;
            }
        }

        function showModal(mode, result) {
            const modal = document.getElementById("diffModal");
            const title = document.getElementById("modalTitle");
            const summary = document.getElementById("modalSummary");
            const list = document.getElementById("modalDiffList");
            const btnApply = document.getElementById("btnModalApply");

            title.textContent = mode === "dry-run" ? t("previewTitle") : t("applyTitle");
            const changes = (result && result.changes) || (result && result.snapshot && result.snapshot.changes) || [];
            btnApply.hidden = mode === "apply" || changes.length === 0;

            summary.textContent = "Attempted: " + (result.attempted || changes.length) + ", Succeeded: " + (result.succeeded || 0) + ", Failed: " + (result.failed || 0) + ", Skipped: " + (result.skipped || 0);

            list.innerHTML = "";
            if (changes.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noChanges") + "</div>";
            } else {
                changes.forEach(c => {
                    const row = document.createElement("div");
                    row.className = "diff-card";
                    const fromP = c.current ? c.current.priority : "-";
                    const toP = c.target ? c.target.priority : (c.priority || "-");
                    const name = c.name || c.auth_index || "Credential";
                    const reason = c.reason || "";
                    const isBoost = c.is_boosted;

                    row.innerHTML = "<div>" +
                        "<div style=\"font-weight:700; font-size:14px; display:flex; align-items:center; gap:6px;\">" +
                        "<span>" + escapeHTML(name) + "</span>" +
                        (isBoost ? "<span class=\"badge badge-boost\">🚀 Boost</span>" : "") +
                        "</div>" +
                        "<div style=\"font-size:12px; color:var(--text-muted);\">" + escapeHTML(reason) + "</div>" +
                        "</div>" +
                        "<div class=\"diff-value-box\">" +
                        "<span class=\"diff-from\">" + fromP + "</span>" +
                        "<span>&rarr;</span>" +
                        "<span class=\"diff-to\">" + toP + "</span>" +
                        "</div>";
                    list.appendChild(row);
                });
            }
            modal.hidden = false;
        }

        function closeModal() {
            document.getElementById("diffModal").hidden = true;
        }

        function applyFromModal() {
            closeModal();
            triggerRun("apply");
        }

        async function triggerReset() {
            if (!confirm(t("confirmReset"))) {
                return;
            }
            const btn = document.getElementById("btnReset");
            if (btn) btn.disabled = true;
            try {
                const result = await apiFetch(RESET_PATH, { method: "POST" });
                showToast(result.message || t("resetSuccess"), "success");
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btn) btn.disabled = false;
            }
        }

        function formatCountdown(targetDateStr) {
            if (!targetDateStr) return "-";
            const target = new Date(targetDateStr).getTime();
            const now = Date.now();
            const diff = target - now;
            if (diff <= 0) return "00m 00s";

            const days = Math.floor(diff / (1000 * 60 * 60 * 24));
            const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
            const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
            const seconds = Math.floor((diff % (1000 * 60)) / 1000);

            if (days > 0) {
                return days + "d " + pad(hours) + "h " + pad(minutes) + "m";
            }
            return pad(hours) + "h " + pad(minutes) + "m " + pad(seconds) + "s";
        }

        function pad(n) { return n < 10 ? "0" + n : n; }

        function renderDashboard() {
            const container = document.getElementById("credentialsContainer");
            if (!container) return;

            const items = (latestSnapshot && latestSnapshot.items) || [];
            let boostedCount = 0;
            let depletedCount = 0;
            let activeCount = 0;

            items.forEach(item => {
                if (item.is_boosted) boostedCount++;
                if (item.target && item.target.disabled) depletedCount++;
                if (item.target && !item.target.disabled) activeCount++;
            });

            document.getElementById("valTotalCreds").textContent = items.length;
            document.getElementById("valTotalDesc").textContent = activeCount + " " + (currentLang === "zh-CN" ? "活跃可用" : "active");
            document.getElementById("valBoosted").textContent = boostedCount;
            document.getElementById("valDepleted").textContent = depletedCount;

            if (latestDiagnostics) {
                document.getElementById("valLastAudit").textContent = latestDiagnostics.latest_audit || "-";
                if (latestDiagnostics.scheduler && latestDiagnostics.scheduler.next_wait) {
                    document.getElementById("valNextProbe").textContent = (currentLang === "zh-CN" ? "下次自动调度: " : "Next run in: ") + latestDiagnostics.scheduler.next_wait;
                }
            }

            container.innerHTML = "";
            if (items.length === 0) {
                container.innerHTML = "<div class=\"empty-state\">" + t("noCreds") + "</div>";
                return;
            }

            items.forEach(item => {
                const card = document.createElement("div");
                card.className = "cred-card";

                const isBoosted = item.is_boosted;
                const r5hPercent = Math.max(0, Math.min(100, Math.round((item.r5h || 0) * 100)));
                const r7dPercent = Math.max(0, Math.min(100, Math.round((item.r7d || 0) * 100)));

                let fill5hClass = "meter-fill-healthy";
                if (r5hPercent <= 10) fill5hClass = "meter-fill-danger";
                else if (r5hPercent <= 30) fill5hClass = "meter-fill-warning";

                let fill7dClass = "meter-fill-healthy";
                if (r7dPercent <= 10) fill7dClass = "meter-fill-danger";
                else if (r7dPercent <= 30) fill7dClass = "meter-fill-warning";

                const urgency = (item.urgency || 0).toFixed(2);
                const burnRate = (item.cycle_burn_rate || 0).toFixed(2);
                const currentP = item.current ? item.current.priority : (item.priority || "-");
                const targetP = item.target ? item.target.priority : currentP;

                let statusBadge = "<span class=\"badge badge-success\">" + t("statusActive") + "</span>";
                if (item.target && item.target.disabled) {
                    statusBadge = "<span class=\"badge badge-danger\">" + t("statusWeeklyDepleted") + "</span>";
                } else if (isBoosted) {
                    statusBadge = "<span class=\"badge badge-boost\">" + t("statusBoosted") + "</span>";
                }

                card.innerHTML =
                    "<div class=\"cred-info\">" +
                        "<div class=\"cred-name\">" +
                            "<span>" + escapeHTML(item.name || item.account || item.auth_index || "Credential") + "</span>" +
                        "</div>" +
                        "<div class=\"cred-meta\">ID: " + escapeHTML(item.auth_index || "-") + " · " + escapeHTML(item.plan_type || "Antigravity") + "</div>" +
                        "<div>" + statusBadge + "</div>" +
                    "</div>" +

                    "<div class=\"cred-metrics\">" +
                        "<div class=\"metric-pill\">" + t("urgencyLabel") + "<strong>" + urgency + "</strong></div>" +
                        "<div class=\"metric-pill\">" + t("burnLabel") + "<strong>" + burnRate + "</strong></div>" +
                    "</div>" +

                    "<div class=\"meter-container\">" +
                        "<div class=\"meter-label-row\">" +
                            "<span>" + t("shortWindow") + " (" + r5hPercent + "%)</span>" +
                            "<span class=\"meter-countdown\" data-reset-time=\"" + (item.short_window_reset_at || "") + "\">" + formatCountdown(item.short_window_reset_at) + "</span>" +
                        "</div>" +
                        "<div class=\"meter-track\">" +
                            "<div class=\"meter-fill " + fill5hClass + "\" style=\"width: " + r5hPercent + "%\"></div>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"meter-container\">" +
                        "<div class=\"meter-label-row\">" +
                            "<span>" + t("longWindow") + " (" + r7dPercent + "%)</span>" +
                            "<span class=\"meter-countdown\" data-reset-time=\"" + (item.long_window_reset_at || "") + "\">" + formatCountdown(item.long_window_reset_at) + "</span>" +
                        "</div>" +
                        "<div class=\"meter-track\">" +
                            "<div class=\"meter-fill " + fill7dClass + "\" style=\"width: " + r7dPercent + "%\"></div>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"cred-priority\">" +
                        "<div style=\"font-size:12px; color:var(--text-muted);\">" + t("priority") + "</div>" +
                        "<div class=\"priority-score\">" + targetP + "</div>" +
                        "<div style=\"font-size:11px; color:var(--text-secondary);\">" + escapeHTML(item.reason || "") + "</div>" +
                    "</div>";

                container.appendChild(card);
            });
        }

        function renderHistory() {
            const list = document.getElementById("historyList");
            if (!list) return;

            const entries = (latestDiagnostics && latestDiagnostics.run_history) || [];
            list.innerHTML = "";
            if (entries.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noHistory") + "</div>";
                return;
            }

            entries.forEach(entry => {
                const item = document.createElement("div");
                item.className = "history-item";
                const dateStr = entry.at ? new Date(entry.at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                const kind = entry.kind || "run";
                const msg = entry.message || "";

                item.innerHTML =
                    "<div class=\"history-head\">" +
                        "<div style=\"display:flex; align-items:center; gap:8px;\">" +
                            "<span class=\"badge badge-subtle\">" + escapeHTML(kind.toUpperCase()) + "</span>" +
                            "<span style=\"font-size:13px; font-weight:600;\">" + escapeHTML(dateStr) + "</span>" +
                        "</div>" +
                        "<div class=\"history-stats\">" +
                            "<span class=\"badge badge-success\">Succeeded: " + (entry.succeeded || 0) + "</span>" +
                            "<span class=\"badge badge-danger\">Failed: " + (entry.failed || 0) + "</span>" +
                            "<span class=\"badge badge-subtle\">Skipped: " + (entry.skipped || 0) + "</span>" +
                        "</div>" +
                    "</div>" +
                    (msg ? "<div style=\"font-size:12px; color:var(--text-muted); font-family:monospace;\">" + escapeHTML(msg) + "</div>" : "");
                list.appendChild(item);
            });
        }

        function renderDiagnostics() {
            const raw = document.getElementById("rawDiagnostics");
            const sched = document.getElementById("schedulerInfo");
            if (raw && latestDiagnostics) {
                raw.textContent = JSON.stringify(latestDiagnostics, null, 2);
            }
            if (sched && latestDiagnostics && latestDiagnostics.scheduler) {
                sched.innerHTML = "<strong>Scheduler:</strong> Interval: " + (latestDiagnostics.scheduler.interval || "-") + " · Active: " + (latestDiagnostics.scheduler.worker_active ? "Yes" : "No") + " · Next Wait: " + (latestDiagnostics.scheduler.next_wait || "-");
            }
        }

        function updateAllCountdowns() {
            document.querySelectorAll(".meter-countdown[data-reset-time]").forEach(el => {
                const resetTime = el.getAttribute("data-reset-time");
                if (resetTime) {
                    el.textContent = formatCountdown(resetTime);
                }
            });
        }

        function showToast(msg, type) {
            const root = document.getElementById("toastRoot");
            if (!root) return;
            const toast = document.createElement("div");
            toast.className = "toast " + (type === "error" ? "toast-error" : type === "info" ? "toast-info" : "toast-success");
            toast.textContent = msg;
            root.appendChild(toast);
            setTimeout(() => toast.remove(), 3000);
        }

        function escapeHTML(str) {
            return String(str || "")
                .replace(/&/g, "&amp;")
                .replace(/</g, "&lt;")
                .replace(/>/g, "&gt;")
                .replace(/"/g, "&quot;")
                .replace(/'/g, "&#039;");
        }

        // Initialize application
        applyLanguage();
        refreshDashboard();
        countdownInterval = setInterval(updateAllCountdowns, 1000);
    </script>
</body>
</html>
`
