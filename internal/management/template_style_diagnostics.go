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
        }

        .diag-kpi-value {
            font-size: 20px;
            font-weight: 800;
            color: var(--text-primary);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            line-height: 1.2;
        }

        .diag-kpi-desc {
            font-size: 12px;
            color: var(--text-muted);
            line-height: 1.3;
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

        .diag-detail-grid {
            display: grid;
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: 6px 20px;
        }

        .diag-detail-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 6px 0;
            border-bottom: 1px solid var(--border-subtle);
            gap: 12px;
        }

        .diag-detail-label {
            font-size: 12.5px;
            font-weight: 600;
            color: var(--text-secondary);
        }

        .diag-detail-val {
            font-size: 12.5px;
            font-weight: 700;
            color: var(--text-primary);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            text-align: right;
        }

        .diag-window-box {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            padding: 10px 14px;
            margin-top: 2px;
        }

        .diag-cooldown-empty {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 12px 14px;
            background: var(--accent-green-subtle);
            border: 1px solid var(--accent-green);
            border-radius: 8px;
            color: var(--accent-green-text);
            font-size: 13px;
            font-weight: 600;
        }

        .diag-cooldown-list {
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .diag-cooldown-item {
            background: var(--accent-yellow-subtle);
            border: 1px solid var(--accent-yellow);
            border-radius: 8px;
            padding: 10px 14px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 12px;
            flex-wrap: wrap;
        }

        .diag-audit-box {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            padding: 10px 14px;
        }

        .diag-audit-text {
            font-size: 12px;
            color: var(--text-primary);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            word-break: break-all;
            line-height: 1.4;
        }

        .diag-pills-row {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
            align-items: center;
        }

`
