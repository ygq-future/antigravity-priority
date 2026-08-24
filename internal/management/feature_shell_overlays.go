package management

const templateStyleOverlays = `        /* Modals */
        .modal-backdrop {
            position: fixed;
            inset: 0;
            display: grid;
            place-items: center;
            background: var(--bg-overlay);
            padding: 20px;
            z-index: 100;
            animation: modalBgIn 200ms ease forwards;
        }

        .modal {
            width: min(720px, 100%);
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 14px;
            padding: 20px;
            box-shadow: var(--shadow-lg);
            display: flex;
            flex-direction: column;
            gap: 14px;
            max-height: 85vh;
            animation: modalIn 260ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
        }

        .modal-body {
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 8px;
            padding-right: 4px;
            max-height: 60vh;
        }

        .diff-card {
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 8px;
            padding: 10px 12px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            flex-wrap: wrap;
        }

        .probe-history-group {
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .probe-history-group-title {
            padding: 4px 2px 0;
            color: var(--text-secondary);
            font-size: 12px;
            font-weight: 700;
        }

        .diff-value-box {
            display: flex;
            align-items: center;
            gap: 8px;
            font-family: SFMono-Regular, Consolas, Menlo, monospace;
            font-weight: 700;
            font-size: 13px;
        }

        .diff-from {
            padding: 2px 6px;
            border-radius: 5px;
            background: var(--diff-from-bg);
            color: var(--diff-from-text);
        }

        .diff-to {
            padding: 2px 6px;
            border-radius: 5px;
            background: var(--diff-to-bg);
            color: var(--diff-to-text);
        }

        .confirm-title {
            margin: 0;
            font-size: 17px;
            font-weight: 700;
            display: flex;
            align-items: center;
            gap: 6px;
        }

        .confirm-message {
            font-size: 13px;
            color: var(--text-secondary);
            margin: 0;
            line-height: 1.5;
        }

        .confirm-actions {
            display: flex;
            justify-content: flex-end;
            gap: 8px;
            margin-top: 6px;
        }

        /* Toast Notifications */
        .toast-root {
            position: fixed;
            top: 70px;
            right: 20px;
            z-index: 200;
            display: flex;
            flex-direction: column;
            gap: 8px;
            max-width: 360px;
        }

        .toast {
            border-radius: 8px;
            padding: 10px 14px;
            font-size: 13px;
            font-weight: 600;
            box-shadow: var(--shadow-md);
            border-left: 4px solid;
            background: var(--bg-card);
            color: var(--text-primary);
            animation: toastIn 240ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
        }
        .toast-success { border-color: var(--accent-green); }
        .toast-error   { border-color: var(--accent-red); }
        .toast-info    { border-color: var(--accent-blue); }
        .toast.toast-exit { animation: toastOut 200ms ease forwards; }

        @keyframes toastIn {
            from { transform: translateX(30px) scale(0.95); opacity: 0; }
            to   { transform: translateX(0) scale(1); opacity: 1; }
        }
        @keyframes toastOut {
            from { opacity: 1; transform: translateY(0); }
            to   { opacity: 0; transform: translateY(-8px); }
        }
        @keyframes modalBgIn {
            from { opacity: 0; }
            to   { opacity: 1; }
        }
        @keyframes modalIn {
            from { transform: translateY(12px) scale(0.96); opacity: 0; }
            to   { transform: translateY(0) scale(1); opacity: 1; }
        }

        [hidden] { display: none !important; }

`
