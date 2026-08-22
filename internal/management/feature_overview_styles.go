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

`
