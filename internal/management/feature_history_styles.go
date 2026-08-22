package management

const templateStyleHistory = `        /* History Items - 100% full width and unified height */
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

`
