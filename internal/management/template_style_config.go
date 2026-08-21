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

`
