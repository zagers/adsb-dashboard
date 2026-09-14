# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please use [GitHub Security Advisories](https://github.com/zagers/adsb-dashboard/security/advisories/new) to privately report the issue.

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment:** Within 48 hours
- **Initial assessment:** Within 1 week
- **Fix or mitigation:** Depends on severity, typically within 2 weeks

## Scope

### In Scope

- The Go server application (`adsb-dashboard/`)
- HTTP server and SSE endpoints
- File parsing (stats.json, aircraft.json, /proc reads)
- systemd service configuration

### Out of Scope

- The `dump1090-fa` application itself
- Raspberry Pi OS or hardware vulnerabilities
- Issues requiring authentication beyond localhost access

## Security Considerations

This application is designed to run on localhost only (`--addr 127.0.0.1` by default). If you expose it beyond localhost:

- Enable TLS/HTTPS
- Add authentication
- Configure firewall rules

The application performs no filesystem writes and no network requests beyond serving the dashboard.
