package management

const templateStyleResponsive = `        @media (max-width: 1100px) {
            .diag-pool-grid { grid-template-columns: 1fr; }
        }

        @media (max-width: 960px) {
            .summary-grid { grid-template-columns: repeat(2, 1fr); }
            .config-form-grid { grid-template-columns: 1fr; }
            .diag-kpi-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
            .diag-lower-grid { grid-template-columns: 1fr; }
        }

        @media (max-width: 600px) {
            .container { padding: 10px; }
            .summary-grid { grid-template-columns: 1fr; }
            .diag-kpi-grid { grid-template-columns: 1fr; }
            .diag-timeline { display: flex; flex-direction: column; padding: 0; }
            .diag-timeline-item { grid-template-columns: 46px minmax(0, 1fr) auto auto; padding: 6px 8px; gap: 8px; }
            .diag-timeline-msg { display: none; }
            .topbar { flex-direction: column; align-items: flex-start; }
            .topbar-actions { width: 100%; justify-content: space-between; margin-right: 0; }
        }
`
