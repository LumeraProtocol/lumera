---
id: security-auth
name: Security & Auth
triggers:
  keywords: [auth, oauth, jwt, encryption, owasp, session, token, password, rbac]
  feature_patterns: ["*auth*", "*login*", "*permission*", "*security*", "*encrypt*"]
applies_to: [architect, coder, debugger]
---

## When to apply

This specialist is relevant when the project involves authentication, authorization, session management, encryption, or any security-sensitive functionality. Look for features involving login, signup, permissions, roles, tokens, or data protection.

## Guidance for architect

- Choose an auth strategy (session-based, JWT, OAuth2) based on the deployment model and client types
- Separate authentication (who are you?) from authorization (what can you do?) in the architecture
- Define a permission model early: RBAC, ABAC, or simple role checks — document it
- Plan token lifecycle: issuance, refresh, revocation, expiration
- Store secrets (API keys, signing keys) in environment variables or a secrets manager — never in code
- Design rate limiting for authentication endpoints to prevent brute force
- Plan for audit logging of security-relevant events (login, permission changes, data access)

## Guidance for coder

- Hash passwords with bcrypt, scrypt, or argon2 — never store plaintext or use fast hashes (MD5, SHA)
- Validate and sanitize all user inputs — OWASP Top 10 applies to every endpoint
- Set secure cookie flags: HttpOnly, Secure, SameSite
- Implement CSRF protection for state-changing requests
- Never expose stack traces, internal paths, or database errors to clients
- Use constant-time comparison for token/password verification to prevent timing attacks
- Rotate secrets and tokens on a defined schedule
- Log authentication events (success, failure, lockout) without logging credentials

## Guidance for debugger

- When investigating auth issues, check token expiration and clock skew first
- Verify that the permission model is applied consistently — check both API and UI enforcement
- Look for broken access control: can user A access user B's resources by changing an ID?
- Check for information leakage in error messages (different errors for "user not found" vs. "wrong password")
- Verify that rate limiting is actually enforced, not just configured

## Review checklist

- [ ] Passwords hashed with a strong algorithm (bcrypt/scrypt/argon2)
- [ ] All inputs validated and sanitized
- [ ] CSRF protection for state-changing requests
- [ ] Secure cookie flags set (HttpOnly, Secure, SameSite)
- [ ] Rate limiting on auth endpoints
- [ ] No secrets in code or logs
- [ ] Audit logging for security events
- [ ] Broken access control tested (horizontal and vertical)
