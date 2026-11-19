# Security Policy

Report vulnerabilities privately through GitHub private vulnerability reporting. Do not open public issues containing credentials, tenant names, schedules, alert payloads or infrastructure identifiers.

Opslink integrates with Incident.io, Slack, Google Workspace, Google Sheets and Gemini. Use separate least-privilege credentials for each integration, rotate them regularly and keep them outside the repository.

The Helm chart is an example. Before deployment, review namespace isolation, Secret access, network controls, schedules and resource limits. Run commands with `--dry-run` while validating configuration.
