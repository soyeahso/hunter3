# MCP iCloud Mail Plugin Summary

## Overview
An MCP (Model Context Protocol) server plugin that provides email functionality for iCloud Mail accounts using standard IMAP/SMTP protocols.

## Key Features
- ✉️ List messages from any mailbox
- 📖 Read full email content
- 📤 Send emails with attachments, CC, BCC, HTML support
- 🔍 Search emails using IMAP criteria
- 📁 List all available mailboxes

## Technology Stack
- **Protocol**: IMAP (reading) and SMTP (sending)
- **Authentication**: App-Specific Passwords (no OAuth)
- **Go Libraries**:
  - `github.com/emersion/go-imap` - IMAP client
  - `github.com/emersion/go-message` - Email parsing
  - `net/smtp` - SMTP sending (standard library)

## Configuration
Simple JSON configuration file:
```json
{
  "email": "your@icloud.com",
  "password": "app-specific-password"
}
```

Or environment variables:
- `ICLOUD_EMAIL`
- `ICLOUD_PASSWORD`

## iCloud Mail Servers
- **IMAP**: imap.mail.me.com:993 (TLS)
- **SMTP**: smtp.mail.me.com:587 (STARTTLS)

## Security
- Requires App-Specific Password from appleid.apple.com
- Credentials stored locally in `~/.hunter3/icloud-mail.json`
- No OAuth flow required (simpler than Gmail)
- Each app should use its own App-Specific Password

## Tools Provided
1. **list_messages** - Browse inbox or any mailbox
2. **read_message** - Read full email content
3. **send_message** - Send emails with full formatting
4. **search_messages** - Search using IMAP queries
5. **list_mailboxes** - Show all folders

## Comparison with Gmail Plugin
| Feature | iCloud Mail | Gmail |
|---------|-------------|-------|
| Protocol | IMAP/SMTP | REST API |
| Auth | App Password | OAuth2 |
| Setup | Simple | Complex (GCP) |
| Message ID | Sequence # | Message ID |
| Search | IMAP queries | Gmail syntax |
| API Limits | IMAP standard | API quotas |

## Use Cases
- Personal email automation for iCloud users
- Simple email integration without OAuth complexity
- Direct protocol access (no API rate limits)
- Works with any IMAP/SMTP provider (easily adaptable)

## Future Enhancements
- Support for other IMAP providers (Gmail via IMAP, Outlook, etc.)
- Attachment download capability
- Email drafts management
- Label/folder operations
- Batch operations
