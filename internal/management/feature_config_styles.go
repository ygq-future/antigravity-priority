package management

const templateStyleConfig = `        /* Config Center - Two columns with horizontal space optimization */
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

        /* Prompt Guard Editor & Dry-Run */
        .pg-toolbar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
            flex-wrap: wrap;
            margin-top: 4px;
        }
        .pg-card-header {
            display: flex;
            align-items: flex-start;
            justify-content: space-between;
            gap: 12px;
            margin-bottom: 6px;
            flex-wrap: nowrap;
        }
        .pg-card-heading {
            display: flex;
            flex-direction: column;
            gap: 2px;
            min-width: 0;
        }
        .pg-card-header .config-card-title {
            margin: 0;
        }
        .pg-card-subtitle {
            margin: 0;
            font-size: 11.5px;
            line-height: 1.4;
            color: var(--text-muted);
        }
        .pg-remote-link {
            color: var(--accent-blue-text);
            text-decoration: none;
            font-weight: 500;
        }
        .pg-remote-link:hover {
            text-decoration: underline;
        }
        .pg-copy-bisect-btn {
            height: 28px;
            padding: 0 10px;
            font-size: 12px;
            white-space: nowrap;
            flex-shrink: 0;
        }
        .pg-toolbar-left {
            display: flex;
            align-items: center;
            gap: 8px;
            flex: 1;
            min-width: 220px;
        }
        .pg-toolbar-right {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-shrink: 0;
        }
        .pg-search-input {
            flex: 1;
            min-width: 160px;
            max-width: 320px;
            height: 30px;
            padding: 0 10px;
            border-radius: 7px;
            border: 1px solid var(--border-color);
            background: var(--bg-surface);
            color: var(--text-primary);
            font-size: 12px;
        }
        .pg-count-pill {
            font-size: 11px;
            font-weight: 700;
            color: var(--text-secondary);
            background: var(--bg-subtle);
            border: 1px solid var(--border-color);
            border-radius: 999px;
            padding: 2px 8px;
            white-space: nowrap;
        }
        .pg-rules-list {
            display: flex;
            flex-direction: column;
            gap: 6px;
            max-height: 360px;
            overflow-y: auto;
            padding-right: 2px;
        }
        .pg-rule-row {
            display: grid;
            grid-template-columns: auto auto minmax(0, 1fr) auto minmax(0, 1fr) 54px 26px;
            align-items: center;
            gap: 6px;
            padding: 6px 8px;
            border-radius: 8px;
            border: 1px solid var(--border-color);
            background: var(--bg-secondary);
        }
        .pg-source-badge {
            font-size: 10px;
            padding: 1px 4px;
            border-radius: 4px;
            line-height: 1.2;
            white-space: nowrap;
            text-align: center;
            user-select: none;
        }
        .pg-source-custom {
            background: rgba(14, 165, 233, 0.15);
            color: var(--accent-color, #0ea5e9);
            border: 1px solid rgba(14, 165, 233, 0.3);
        }
        .pg-source-remote {
            background: rgba(156, 163, 175, 0.15);
            color: var(--text-muted, #9ca3af);
            border: 1px solid rgba(156, 163, 175, 0.3);
        }
        .pg-rule-row.pg-rule-disabled {
            opacity: 0.62;
        }
        .pg-mini-toggle {
            position: relative;
            display: inline-block;
            width: 32px;
            height: 18px;
            flex-shrink: 0;
        }
        .pg-mini-toggle input {
            opacity: 0;
            width: 0;
            height: 0;
            position: absolute;
        }
        .pg-mini-slider {
            position: absolute;
            cursor: pointer;
            inset: 0;
            background: var(--meter-bg);
            border-radius: 18px;
            transition: 0.2s;
        }
        .pg-mini-slider:before {
            position: absolute;
            content: "";
            height: 12px;
            width: 12px;
            left: 3px;
            bottom: 3px;
            background: #ffffff;
            border-radius: 50%;
            transition: 0.2s;
        }
        .pg-mini-toggle input:checked + .pg-mini-slider {
            background: var(--accent-green);
        }
        .pg-mini-toggle input:checked + .pg-mini-slider:before {
            transform: translateX(14px);
        }
        .pg-rule-input {
            width: 100%;
            min-width: 0;
            height: 28px;
            padding: 0 8px;
            border-radius: 6px;
            border: 1px solid var(--border-color);
            background: var(--bg-surface);
            color: var(--text-primary);
            font-size: 12px;
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
            box-sizing: border-box;
        }
        .pg-rule-input:focus {
            outline: none;
            border-color: var(--border-focus);
        }
        .pg-rule-input.pg-input-invalid {
            border-color: var(--accent-red);
            background: var(--accent-red-subtle);
        }
        .pg-arrow {
            color: var(--text-muted);
            font-size: 12px;
            font-weight: 700;
            user-select: none;
        }
        .pg-pill-btn {
            width: 100%;
            height: 26px;
            padding: 0 8px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            text-align: center;
            border-radius: 999px;
            border: 1px solid var(--border-color);
            background: var(--bg-surface);
            color: var(--text-secondary);
            font-size: 11px;
            font-weight: 650;
            cursor: pointer;
            white-space: nowrap;
            box-sizing: border-box;
            transition: all 0.15s ease;
        }
        .pg-pill-btn:hover {
            border-color: var(--border-focus);
            color: var(--text-primary);
        }
        .pg-pill-btn.active-all {
            background: var(--accent-purple-subtle);
            border-color: var(--accent-purple);
            color: var(--accent-purple-text);
        }
        .pg-pill-btn.active-regex {
            background: var(--accent-blue-subtle);
            border-color: var(--accent-blue);
            color: var(--accent-blue-text);
        }
        .pg-delete-btn {
            height: 26px;
            width: 26px;
            padding: 0;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            border-radius: 6px;
            border: 1px solid transparent;
            background: transparent;
            color: var(--text-muted);
            cursor: pointer;
        }
        .pg-delete-btn:hover {
            color: var(--accent-red-text);
            background: var(--accent-red-subtle);
            border-color: var(--accent-red);
        }
        .pg-empty-box {
            padding: 16px;
            text-align: center;
            font-size: 12px;
            color: var(--text-muted);
            border: 1px dashed var(--border-color);
            border-radius: 8px;
        }
        .pg-dryrun-box {
            margin-top: 6px;
            padding-top: 10px;
            border-top: 1px solid var(--border-subtle);
            display: flex;
            flex-direction: column;
            gap: 8px;
        }
        .pg-dryrun-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
            flex-wrap: wrap;
        }
        .pg-dryrun-title {
            font-size: 12.5px;
            font-weight: 700;
            color: var(--text-primary);
        }
        .pg-dryrun-controls {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-wrap: wrap;
        }
        .pg-textarea {
            width: 100%;
            min-height: 82px;
            padding: 8px 10px;
            border-radius: 8px;
            border: 1px solid var(--border-color);
            background: var(--bg-surface);
            color: var(--text-primary);
            font-size: 12px;
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
            resize: vertical;
            box-sizing: border-box;
        }
        .pg-preview-result {
            display: flex;
            flex-direction: column;
            gap: 6px;
            padding: 10px;
            border-radius: 8px;
            border: 1px solid var(--border-color);
            background: var(--bg-secondary);
        }
        .pg-preview-summary {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
            flex-wrap: wrap;
            font-size: 12px;
            font-weight: 650;
        }
        .pg-preview-copy-btn {
            min-height: 24px;
            height: 24px;
            padding: 0 8px;
            font-size: 11px;
        }
        .pg-preview-warnings {
            display: flex;
            flex-wrap: wrap;
            gap: 6px;
        }
        .pg-preview-matches {
            display: flex;
            flex-direction: column;
            gap: 6px;
        }
        .pg-match-card {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            padding: 8px 12px;
            border-radius: 8px;
            background: var(--bg-surface);
            border: 1px solid var(--border-color);
        }
        .pg-match-diff {
            display: flex;
            flex-direction: column;
            gap: 3px;
            min-width: 0;
            flex: 1;
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
            font-size: 12px;
            line-height: 1.4;
            word-break: break-word;
        }
        .pg-match-find {
            color: var(--text-muted);
            text-decoration: line-through;
        }
        .pg-match-replace {
            color: var(--accent-blue-text);
        }
        .pg-match-empty {
            font-style: italic;
            opacity: 0.7;
        }
        .pg-match-count {
            flex-shrink: 0;
            font-size: 12px;
            font-weight: 600;
            color: var(--text-muted);
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
        }
        .pg-warn-chip {
            font-size: 11px;
            padding: 2px 8px;
            border-radius: 6px;
            background: var(--accent-red-subtle);
            color: var(--accent-red-text);
            border: 1px solid var(--accent-red);
        }
        .pg-preview-output {
            margin: 0;
            width: 100%;
            min-height: 120px;
            max-height: 240px;
            overflow: auto;
            padding: 8px 10px;
            border-radius: 6px;
            border: 1px solid var(--border-color);
            background: var(--bg-surface);
            color: var(--text-primary);
            font-size: 11.5px;
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
            line-height: 1.45;
            resize: vertical;
            box-sizing: border-box;
        }
        @media (max-width: 820px) {
            .pg-rule-row {
                grid-template-columns: auto 1fr auto 1fr;
            }
        }

`
