package management

const templateStyleHistory = `        /* Request History Table Styling */
        #historyList {
            width: 100% !important;
            height: 100% !important;
            box-sizing: border-box !important;
            overflow-y: auto !important;
            overflow-x: auto !important;
            scrollbar-width: thin;
            scrollbar-color: var(--border-color) transparent;
        }

        #historyList::-webkit-scrollbar {
            width: 5px;
            height: 5px;
        }

        #historyList::-webkit-scrollbar-track {
            background: transparent;
        }

        #historyList::-webkit-scrollbar-thumb {
            background: var(--border-color);
            border-radius: 999px;
        }

        .request-history-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 12px;
            text-align: left;
        }

        .request-history-table th {
            position: sticky;
            top: 0;
            background: var(--bg-card);
            border-bottom: 2px solid var(--border-color);
            padding: 8px 12px;
            font-size: 11.5px;
            font-weight: 700;
            color: var(--text-secondary);
            white-space: nowrap;
            z-index: 2;
        }

        .request-history-table td {
            padding: 9px 12px;
            border-bottom: 1px solid var(--border-color);
            color: var(--text-primary);
            white-space: nowrap;
            vertical-align: middle;
        }

        .request-history-table tr:hover td {
            background: rgba(255, 255, 255, 0.02);
        }

        .req-time {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 11.5px;
            color: var(--text-secondary);
        }

        .req-model-badge {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 2px 7px;
            border-radius: 6px;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 11.5px;
            font-weight: 650;
            background: var(--bg-subtle);
            border: 1px solid var(--border-color);
        }

        .req-model-gemini {
            color: var(--accent-blue-text);
            border-color: rgba(59, 130, 246, 0.3);
            background: rgba(59, 130, 246, 0.08);
        }

        .req-model-argon {
            color: #06b6d4;
            border-color: rgba(6, 182, 212, 0.3);
            background: rgba(6, 182, 212, 0.08);
        }

        .req-model-claude {
            color: var(--accent-purple-text);
            border-color: rgba(168, 85, 247, 0.3);
            background: rgba(168, 85, 247, 0.08);
        }

        .req-cred {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 12px;
            font-weight: 600;
            color: var(--text-primary);
        }

        .req-tokens {
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 11.5px;
            color: var(--text-primary);
        }

        .req-tokens-total {
            font-weight: 750;
            color: var(--accent-green-text);
            margin-left: 4px;
        }

        .req-status-ok {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            font-weight: 650;
            font-size: 11.5px;
            color: var(--accent-green-text);
        }

        .req-status-429 {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            font-weight: 650;
            font-size: 11.5px;
            color: var(--accent-yellow-text);
        }

        .req-status-fail {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            font-weight: 650;
            font-size: 11.5px;
            color: var(--accent-red-text);
        }
`
