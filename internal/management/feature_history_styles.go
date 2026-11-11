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

        .req-status-cell {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            line-height: 1;
        }

        .req-status-ok {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            line-height: 1;
            font-weight: 650;
            font-size: 11.5px;
            color: var(--accent-green-text);
        }

        .req-status-429 {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            line-height: 1;
            font-weight: 650;
            font-size: 11.5px;
            color: var(--accent-yellow-text);
        }

        .req-status-fail {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            line-height: 1;
            font-weight: 650;
            font-size: 11.5px;
            color: var(--accent-red-text);
        }

        .history-limit-control {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            font-size: 12px;
            color: var(--text-secondary);
            white-space: nowrap;
        }

        .history-limit-input {
            width: 72px !important;
            min-width: 72px !important;
            max-width: 72px !important;
            height: 30px !important;
            min-height: 30px !important;
            text-align: center !important;
            padding: 4px 8px !important;
            box-sizing: border-box !important;
            -moz-appearance: textfield;
            appearance: textfield;
        }

        .history-limit-input::-webkit-outer-spin-button,
        .history-limit-input::-webkit-inner-spin-button {
            -webkit-appearance: none;
            margin: 0;
        }

        .history-limit-save {
            height: 30px;
            min-height: 30px;
            padding: 0 10px;
            font-size: 12px;
        }

        .req-error-info {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            flex: 0 0 auto;
            width: 16px;
            height: 16px;
            border: 1px solid currentColor;
            border-radius: 50%;
            color: var(--accent-red-text);
            font-size: 10px;
            font-weight: 800;
            line-height: 1;
            cursor: help;
            outline: none;
        }

        .req-error-info:hover,
        .req-error-info:focus-visible {
            background: var(--accent-red-subtle);
        }

        .req-error-tooltip {
            position: fixed;
            z-index: 10000;
            max-width: min(440px, calc(100vw - 16px));
            max-height: min(320px, 50vh);
            overflow-y: auto;
            padding: 9px 12px;
            border: 1px solid var(--border-color);
            border-radius: 8px;
            background: var(--bg-card);
            color: var(--text-primary);
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-size: 11.5px;
            line-height: 1.55;
            white-space: pre-wrap;
            overflow-wrap: anywhere;
            user-select: text;
            cursor: text;
            opacity: 0;
            visibility: hidden;
            pointer-events: none;
            transform: translateY(4px);
            transition: opacity 0.15s ease, transform 0.15s ease, visibility 0s linear 0.15s;
        }

        .req-error-tooltip[data-placement="top"] {
            transform: translateY(-4px);
        }

        .req-error-tooltip.visible {
            opacity: 1;
            visibility: visible;
            pointer-events: auto;
            transform: translateY(0);
            transition: opacity 0.15s ease, transform 0.15s ease, visibility 0s;
        }
`
