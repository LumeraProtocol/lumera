# Security Rules

- No secrets, API keys, or credentials in code or configuration files.
- No sensitive data in production logs (PII, tokens, passwords).
- OWASP Top 10 awareness: validate inputs at system boundaries, use parameterized queries, escape output.
- Never commit `.env`, `credentials.json`, `*.pem`, or `*.key` files.
- Use environment variables for all secrets and sensitive configuration.
- Sanitize user input before rendering in HTML or executing in shell commands.
- Prefer allow-lists over deny-lists for input validation.
