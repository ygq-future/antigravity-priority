package management

const templateStyleControls = `        /* KPI Cards Grid */
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

`
