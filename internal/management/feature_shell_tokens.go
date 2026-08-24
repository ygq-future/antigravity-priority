package management

const templateStyleTokens = `
        :root {
            color-scheme: light dark;
            --bg-primary: #f8fafc;
            --bg-secondary: #f1f5f9;
            --bg-tertiary: #e2e8f0;
            --bg-surface: #ffffff;
            --bg-card: #ffffff;
            --bg-subtle: #f1f5f9;
            --bg-hover: #e2e8f0;
            --bg-overlay: rgba(15, 23, 42, 0.45);
            --border-color: #e2e8f0;
            --border-subtle: #f1f5f9;
            --border-focus: #3b82f6;
            --text-primary: #0f172a;
            --text-secondary: #475569;
            --text-muted: #94a3b8;
            --text-inverse: #ffffff;
            --accent-blue: #2563eb;
            --accent-blue-hover: #1d4ed8;
            --accent-blue-subtle: #eff6ff;
            --accent-blue-text: #1d4ed8;
            --accent-green: #10b981;
            --accent-green-subtle: #ecfdf5;
            --accent-green-text: #047857;
            --accent-yellow: #f59e0b;
            --accent-yellow-subtle: #fffbeb;
            --accent-yellow-text: #b45309;
            --accent-red: #ef4444;
            --accent-red-subtle: #fef2f2;
            --accent-red-text: #b91c1c;
            --accent-purple: #7c3aed;
            --accent-purple-subtle: #faf5ff;
            --accent-purple-text: #6d28d9;
            --meter-bg: #e2e8f0;
            --meter-track-bg: #cbd5e1;
            --meter-track-border: #94a3b8;
            --meter-fill: #10b981;
            --meter-warn: #f59e0b;
            --meter-danger: #ef4444;
            --badge-boost-bg: #fdf4ff;
            --badge-boost-border: #f5d0fe;
            --badge-boost-text: #9333ea;
            --diff-from-bg: #fee2e2;
            --diff-from-text: #991b1b;
            --diff-to-bg: #dcfce7;
            --diff-to-text: #166534;
            --shadow-sm: 0 1px 3px 0 rgba(0, 0, 0, 0.04), 0 1px 2px -1px rgba(0, 0, 0, 0.03);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.03);
            --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.06), 0 4px 6px -4px rgba(0, 0, 0, 0.03);
        }

        @media (prefers-color-scheme: dark) {
            :root:not([data-theme="light"]) {
                --bg-primary: #121214;
                --bg-secondary: #18181b;
                --bg-tertiary: #27272a;
                --bg-surface: #18181b;
                --bg-card: #1f1f23;
                --bg-subtle: #27272a;
                --bg-hover: #3f3f46;
                --bg-overlay: rgba(0, 0, 0, 0.7);
                --border-color: #27272a;
                --border-subtle: #27272a;
                --border-focus: #a1a1aa;
                --text-primary: #f4f4f5;
                --text-secondary: #a1a1aa;
                --text-muted: #71717a;
                --text-inverse: #0f172a;
                --accent-blue: #3b82f6;
                --accent-blue-hover: #60a5fa;
                --accent-blue-subtle: #1e293b;
                --accent-blue-text: #60a5fa;
                --accent-green: #22c55e;
                --accent-green-subtle: rgba(20, 83, 45, 0.25);
                --accent-green-text: #4ade80;
                --accent-yellow: #f59e0b;
                --accent-yellow-subtle: rgba(120, 53, 15, 0.25);
                --accent-yellow-text: #fbbf24;
                --accent-red: #ef4444;
                --accent-red-subtle: rgba(127, 29, 29, 0.25);
                --accent-red-text: #f87171;
                --accent-purple: #a855f7;
                --accent-purple-subtle: rgba(88, 28, 135, 0.25);
                --accent-purple-text: #c084fc;
                --meter-bg: #27272a;
                --meter-track-bg: #3f3f46;
                --meter-track-border: #52525b;
                --meter-fill: #3b82f6;
                --meter-warn: #f59e0b;
                --meter-danger: #ef4444;
                --badge-boost-bg: rgba(74, 4, 78, 0.35);
                --badge-boost-border: #a21caf;
                --badge-boost-text: #f0abfc;
                --diff-from-bg: rgba(127, 29, 29, 0.35);
                --diff-from-text: #fca5a5;
                --diff-to-bg: rgba(20, 83, 45, 0.35);
                --diff-to-text: #86efac;
                --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.4);
                --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.4), 0 2px 4px -2px rgba(0, 0, 0, 0.4);
                --shadow-lg: 0 10px 18px -3px rgba(0, 0, 0, 0.5), 0 4px 6px -4px rgba(0, 0, 0, 0.5);
            }
        }

        :root[data-theme="dark"] {
            --bg-primary: #121214;
            --bg-secondary: #18181b;
            --bg-tertiary: #27272a;
            --bg-surface: #18181b;
            --bg-card: #1f1f23;
            --bg-subtle: #27272a;
            --bg-hover: #3f3f46;
            --bg-overlay: rgba(0, 0, 0, 0.7);
            --border-color: #27272a;
            --border-subtle: #27272a;
            --border-focus: #a1a1aa;
            --text-primary: #f4f4f5;
            --text-secondary: #a1a1aa;
            --text-muted: #71717a;
            --text-inverse: #0f172a;
            --accent-blue: #3b82f6;
            --accent-blue-hover: #60a5fa;
            --accent-blue-subtle: #1e293b;
            --accent-blue-text: #60a5fa;
            --accent-green: #22c55e;
            --accent-green-subtle: rgba(20, 83, 45, 0.25);
            --accent-green-text: #4ade80;
            --accent-yellow: #f59e0b;
            --accent-yellow-subtle: rgba(120, 53, 15, 0.25);
            --accent-yellow-text: #fbbf24;
            --accent-red: #ef4444;
            --accent-red-subtle: rgba(127, 29, 29, 0.25);
            --accent-red-text: #f87171;
            --accent-purple: #a855f7;
            --accent-purple-subtle: rgba(88, 28, 135, 0.25);
            --accent-purple-text: #c084fc;
            --meter-bg: #27272a;
            --meter-track-bg: #3f3f46;
            --meter-track-border: #52525b;
            --meter-fill: #3b82f6;
            --meter-warn: #f59e0b;
            --meter-danger: #ef4444;
            --badge-boost-bg: rgba(74, 4, 78, 0.35);
            --badge-boost-border: #a21caf;
            --badge-boost-text: #f0abfc;
            --diff-from-bg: rgba(127, 29, 29, 0.35);
            --diff-from-text: #fca5a5;
            --diff-to-bg: rgba(20, 83, 45, 0.35);
            --diff-to-text: #86efac;
            --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.4);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.4), 0 2px 4px -2px rgba(0, 0, 0, 0.4);
            --shadow-lg: 0 10px 18px -3px rgba(0, 0, 0, 0.5), 0 4px 6px -4px rgba(0, 0, 0, 0.5);
        }

        :root[data-theme="light"] {
            --bg-primary: #f8fafc;
            --bg-secondary: #f1f5f9;
            --bg-tertiary: #e2e8f0;
            --bg-surface: #ffffff;
            --bg-card: #ffffff;
            --bg-subtle: #f1f5f9;
            --bg-hover: #e2e8f0;
            --bg-overlay: rgba(15, 23, 42, 0.45);
            --border-color: #e2e8f0;
            --border-subtle: #f1f5f9;
            --border-focus: #3b82f6;
            --text-primary: #0f172a;
            --text-secondary: #475569;
            --text-muted: #94a3b8;
            --text-inverse: #ffffff;
            --accent-blue: #2563eb;
            --accent-blue-hover: #1d4ed8;
            --accent-blue-subtle: #eff6ff;
            --accent-blue-text: #1d4ed8;
            --accent-green: #10b981;
            --accent-green-subtle: #ecfdf5;
            --accent-green-text: #047857;
            --accent-yellow: #f59e0b;
            --accent-yellow-subtle: #fffbeb;
            --accent-yellow-text: #b45309;
            --accent-red: #ef4444;
            --accent-red-subtle: #fef2f2;
            --accent-red-text: #b91c1c;
            --accent-purple: #7c3aed;
            --accent-purple-subtle: #faf5ff;
            --accent-purple-text: #6d28d9;
            --meter-bg: #e2e8f0;
            --meter-track-bg: #cbd5e1;
            --meter-track-border: #94a3b8;
            --meter-fill: #10b981;
            --meter-warn: #f59e0b;
            --meter-danger: #ef4444;
            --badge-boost-bg: #fdf4ff;
            --badge-boost-border: #f5d0fe;
            --badge-boost-text: #9333ea;
            --diff-from-bg: #fee2e2;
            --diff-from-text: #991b1b;
            --diff-to-bg: #dcfce7;
            --diff-to-text: #166534;
            --shadow-sm: 0 1px 3px 0 rgba(0, 0, 0, 0.04), 0 1px 2px -1px rgba(0, 0, 0, 0.03);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.03);
            --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.06), 0 4px 6px -4px rgba(0, 0, 0, 0.03);
        }

        * { box-sizing: border-box; }
        html, body {
            height: 100%;
            margin: 0;
            padding: 0;
            overflow: hidden;
        }

`
