package management

const templateStyleResponsive = `        @media (max-width: 960px) {
            .summary-grid { grid-template-columns: repeat(2, 1fr); }
            .config-form-grid { grid-template-columns: 1fr; }
            .diag-kpi-grid { grid-template-columns: 1fr; }
            .diag-detail-grid { grid-template-columns: 1fr; }
        }

        @media (max-width: 600px) {
            .container { padding: 10px; }
            .summary-grid { grid-template-columns: 1fr; }
            .topbar { flex-direction: column; align-items: flex-start; }
            .topbar-actions { width: 100%; justify-content: space-between; margin-right: 0; }
        }
`
