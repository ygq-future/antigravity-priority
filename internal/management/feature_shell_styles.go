package management

const templateStyleShell = `        body {
            background: var(--bg-primary);
            color: var(--text-primary);
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.5;
            display: flex;
            flex-direction: column;
        }

        .container {
            width: 100%;
            max-width: 1320px;
            height: 100%;
            margin: 0 auto;
            display: flex;
            flex-direction: column;
            padding: 16px 20px;
            box-sizing: border-box;
            overflow: hidden;
            position: relative;
        }

        .topbar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
            flex-wrap: wrap;
            margin-bottom: 12px;
            padding-bottom: 12px;
            border-bottom: 1px solid var(--border-subtle);
            flex-shrink: 0;
        }

        .brand-zone {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        h1 {
            margin: 0;
            font-size: 20px;
            font-weight: 750;
            letter-spacing: -0.02em;
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .version-badge {
            display: inline-flex;
            align-items: center;
            padding: 2px 8px;
            border-radius: 999px;
            background: var(--accent-blue-subtle);
            color: var(--accent-blue-text);
            font-size: 12px;
            font-weight: 700;
        }

        .topbar-actions {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-wrap: wrap;
            margin-right: 140px;
        }

        .tabs {
            display: flex;
            gap: 4px;
            padding: 3px;
            background: var(--bg-subtle);
            border: 1px solid var(--border-subtle);
            border-radius: 10px;
            margin-bottom: 12px;
            flex-shrink: 0;
        }

        .tab {
            flex: 1;
            min-height: 34px;
            background: transparent;
            color: var(--text-secondary);
            border: 0;
            border-radius: 7px;
            padding: 6px 14px;
            font-size: 13px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.15s ease;
            white-space: nowrap;
        }

        .tab.active {
            background: var(--bg-card);
            color: var(--text-primary);
            box-shadow: var(--shadow-sm);
        }

        section[id^="panel"] {
            flex: 1;
            min-height: 0;
            display: flex;
            flex-direction: column;
            overflow: hidden;
            transition: opacity 0.18s ease-out;
            position: relative;
        }

        .card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 16px;
        }

`
