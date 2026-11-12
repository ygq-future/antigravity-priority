package management

const templateStyleDiagnostics = `        /* System Diagnostics Panel Styles */
        .diag-scroll-container {
            flex: 1;
            min-height: 0;
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 12px;
            padding-bottom: 20px;
            box-sizing: border-box;
            width: 100%;
        }

        .diag-top-bar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 10px;
            flex-wrap: wrap;
            padding: 2px 0 4px 0;
        }

        .diag-title-group {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-wrap: wrap;
            min-width: 0;
        }

        .diag-title {
            margin: 0;
            font-size: 15px;
            font-weight: 700;
            color: var(--text-primary);
        }

        .diag-health-text {
            font-size: 12px;
            color: var(--text-secondary);
        }

        .diag-copy-btn {
            min-height: 30px;
            height: 30px;
            padding: 0 12px;
            font-size: 12px;
        }

        .diag-kpi-grid {
            display: grid;
            grid-template-columns: repeat(3, minmax(0, 1fr));
            gap: 12px;
            width: 100%;
            box-sizing: border-box;
        }

        .diag-kpi-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 14px 16px;
            display: flex;
            flex-direction: column;
            gap: 6px;
            box-shadow: var(--shadow-sm);
            min-width: 0;
        }

        .diag-kpi-head {
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 8px;
        }

        .diag-kpi-title {
            font-size: 13px;
            font-weight: 700;
            color: var(--text-secondary);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .diag-kpi-value {
            font-size: 20px;
            font-weight: 800;
            color: var(--text-primary);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            line-height: 1.2;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .diag-kpi-desc {
            font-size: 12px;
            color: var(--text-muted);
            line-height: 1.3;
            min-height: 16px;
        }

        .diag-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 14px 16px;
            display: flex;
            flex-direction: column;
            gap: 10px;
            box-shadow: var(--shadow-sm);
            min-width: 0;
        }

        .diag-card-title {
            margin: 0;
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

        .diag-table-wrap {
            width: 100%;
            overflow-x: auto;
            min-width: 0;
        }

        .diag-table-scroll {
            max-height: 280px;
            overflow-y: auto;
        }

        .diag-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 12px;
        }

        .diag-table th {
            position: sticky;
            top: 0;
            z-index: 1;
            background: var(--bg-card);
            text-align: left;
            font-size: 11px;
            font-weight: 700;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.4px;
            padding: 6px 8px;
            border-bottom: 1px solid var(--border-color);
            white-space: nowrap;
        }

        .diag-table td {
            padding: 6px 8px;
            border-bottom: 1px solid var(--border-subtle);
            vertical-align: middle;
        }

        .diag-table tbody tr:last-child td {
            border-bottom: none;
        }

        .diag-mono {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
        }

        .diag-cred {
            display: flex;
            align-items: center;
            gap: 6px;
            min-width: 0;
        }

        .diag-cred-name {
            font-weight: 600;
            color: var(--text-primary);
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            max-width: 180px;
        }

        .diag-cred-pool {
            font-size: 10px;
            flex: 0 0 auto;
        }

        .diag-reason {
            max-width: 240px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            color: var(--text-secondary);
        }

        .diag-empty {
            text-align: center;
            color: var(--text-muted);
            padding: 14px;
        }

        .diag-lower-grid {
            display: grid;
            grid-template-columns: minmax(0, 1fr) minmax(0, 1.5fr);
            gap: 12px;
            align-items: stretch;
        }

        .diag-pool-grid {
            grid-template-columns: minmax(0, 1.7fr) minmax(0, 1fr);
        }

        .diag-storm-banner {
            display: flex;
            flex-direction: column;
            gap: 2px;
            padding: 8px 10px;
            background: var(--accent-red-subtle);
            border: 1px solid var(--accent-red);
            border-radius: 8px;
            color: var(--accent-red-text);
            font-size: 12px;
        }

        .diag-storm-banner[hidden] {
            display: none;
        }

        .diag-health-block {
            display: flex;
            flex-direction: column;
            gap: 6px;
            min-width: 0;
        }

        .diag-subhead {
            font-size: 11px;
            font-weight: 700;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.4px;
        }

        .diag-empty-line {
            font-size: 12px;
            color: var(--text-muted);
            padding: 4px 0;
        }

        .diag-mini-list {
            display: flex;
            flex-direction: column;
            gap: 5px;
            max-height: 200px;
            overflow-y: auto;
        }

        .diag-mini-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
            padding: 6px 8px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            font-size: 12px;
            min-width: 0;
        }

        .diag-mini-main {
            display: flex;
            flex-direction: column;
            gap: 2px;
            min-width: 0;
        }

        .diag-mini-meta {
            font-size: 11px;
            color: var(--text-muted);
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .diag-mini-countdown {
            display: flex;
            flex-direction: column;
            align-items: flex-end;
            gap: 1px;
            font-size: 11px;
            color: var(--text-muted);
            white-space: nowrap;
        }

        .diag-mini-countdown strong {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            color: var(--accent-yellow-text);
        }

        .diag-cooldown-empty {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 10px 12px;
            background: var(--accent-green-subtle);
            border: 1px solid var(--accent-green);
            border-radius: 8px;
            color: var(--accent-green-text);
            font-size: 12px;
            font-weight: 600;
        }

        .diag-timeline {
            display: flex;
            flex-direction: column;
            gap: 4px;
            max-height: 260px;
            overflow-y: auto;
        }

        .diag-timeline-item {
            display: grid;
            grid-template-columns: 56px minmax(0, auto) minmax(0, 1fr) auto auto;
            align-items: center;
            gap: 8px;
            padding: 6px 8px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            font-size: 12px;
            text-align: left;
            color: var(--text-primary);
            cursor: pointer;
            font-family: inherit;
        }

        .diag-timeline-item:hover {
            background: var(--bg-hover);
        }

        .diag-timeline-time {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            color: var(--text-muted);
            font-size: 11px;
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .diag-timeline-kind {
            font-weight: 600;
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .diag-timeline-msg {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            color: var(--text-secondary);
        }

        .diag-timeline-outcome {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            text-align: right;
        }

        /* Share one column grid across rows so every row aligns even when the
           kind or message length differs; the base rules above stay as the
           per-row fallback for engines without subgrid. */
        @supports (grid-template-columns: subgrid) {
            .diag-timeline {
                display: grid;
                grid-template-columns: 64px max-content minmax(0, 1fr) max-content max-content;
                column-gap: 8px;
                row-gap: 4px;
                padding: 0 8px;
            }

            .diag-timeline-item {
                grid-column: 1 / -1;
                grid-template-columns: subgrid;
                gap: 8px;
                padding: 6px 0;
            }

            .diag-timeline-time {
                padding-left: 8px;
            }

            .diag-timeline-item .badge {
                margin-right: 8px;
            }
        }

        .diag-storage-list {
            display: flex;
            flex-direction: column;
            gap: 6px;
        }

        .diag-storage-item {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            padding: 7px 10px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            min-width: 0;
        }

        .diag-storage-info {
            display: flex;
            flex-direction: column;
            gap: 2px;
            min-width: 0;
            flex: 1 1 auto;
        }

        .diag-storage-role {
            font-size: 12.5px;
            font-weight: 700;
            color: var(--text-primary);
        }

        .diag-storage-path {
            font-size: 11px;
            color: var(--text-muted);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .diag-storage-size {
            flex: 0 0 auto;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
        }

        .diag-storage-warning {
            font-size: 11.5px;
            color: var(--accent-red-text);
            background: var(--accent-red-subtle);
            border: 1px solid var(--accent-red);
            border-radius: 8px;
            padding: 6px 10px;
            line-height: 1.35;
            word-break: break-word;
        }

`
