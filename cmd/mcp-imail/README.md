# MCP iCloud Mail Plugin

A Model Context Protocol (MCP) plugin that provides email functionality for iCloud Mail accounts via IMAP/SMTP protocols.

## Features

- **List Messages**: Browse messages from any mailbox (INBOX, Sent, Drafts, etc.)
- **Read Messages**: Read full email content including headers and body
- **Send Messages**: Send emails with support for:
  - Multiple recipients (To, CC, BCC)
  - HTML and plain text formats
  - File attachments
- **Search Messages**: Search for emails using IMAP search criteria
- **List Mailboxes**: View all available mailboxes in your account

## Prerequisites

1. An iCloud Mail account
2. An **App-Specific Password** (required for third-party app access)

### Generating an App-Specific Password

1. Go to [appleid.apple.com](https://appleid.apple.com)
2. Sign in with your Apple ID
3. Go to "Security" section
4. Under "App-Specific Passwords", click "Generate Password"
5. Enter a label (e.g., "Hunter3 MCP")
6. Copy the generated password (format: xxxx-xxxx-xxxx-xxxx)

## Installation

1. Build the plugin:
```bash
cd /path/to/hunter3
make mcp-imail
```

2. Create configuration file:
```bash
mkdir -p ~/.hunter3
cat > ~/.hunter3/icloud-mail.json << 'EOF'
{
  "email": "your-email@icloud.com",
  "password": "your-app-specific-password"
}
EOF
chmod 600 ~/.hunter3/icloud-mail.json
```

Alternatively, you can set environment variables:
```bash
export ICLOUD_EMAIL="your-email@icloud.com"
export ICLOUD_PASSWORD="your-app-specific-password"
```

3. Register with Claude CLI:
```bash
claude mcp add --transport stdio mcp-imail -- /path/to/hunter3/dist/mcp-imail
```

Or add manually to `~/.claude/config.json`:
```json
{
  "mcpServers": {
    "imail": {
      "command": "/path/to/hunter3/dist/mcp-imail",
      "args": [],
      "env": {}
    }
  }
}
```

## Usage

### List Messages

List recent messages from your inbox:
```
List my recent iCloud emails
```

List messages from a specific mailbox:
```
List emails from my Sent folder
```

### Read a Message

After listing messages, you'll see sequence numbers. Use them to read full content:
```
Read iCloud email with sequence number 5
```

### Send an Email

Send a simple text email:
```
Send an email via iCloud to user@example.com with subject "Hello" and body "This is a test"
```

Send with CC and attachments:
```
Send an email via iCloud to user@example.com, CC manager@example.com, subject "Report" 
with body "Please see attached report" and attach /path/to/report.pdf
```

Send HTML email:
```
Send an HTML email via iCloud to user@example.com with subject "Newsletter" 
and body "<h1>Hello!</h1><p>This is HTML content</p>"
```

### Search Messages

Search for unread messages:
```
Search for unread messages in my iCloud inbox
```

Search by sender:
```
Search iCloud emails FROM user@example.com
```

Search by subject:
```
Search iCloud emails with SUBJECT meeting
```

### List Mailboxes

See all available mailboxes:
```
List my iCloud mailboxes
```

## Available Tools

### list_messages
List email messages from a mailbox.

**Arguments:**
- `mailbox` (optional): Mailbox name (default: INBOX)
- `limit` (optional): Max messages to return (default: 10, max: 100)

### read_message
Read full content of a specific message.

**Arguments:**
- `mailbox` (optional): Mailbox name (default: INBOX)
- `seq_num` (required): Sequence number of the message

### send_message
Send an email message.

**Arguments:**
- `to` (required): Recipient email(s), comma-separated
- `subject` (required): Email subject
- `body` (required): Email body content
- `cc` (optional): CC recipients, comma-separated
- `bcc` (optional): BCC recipients, comma-separated
- `is_html` (optional): "true" for HTML content (default: "false")
- `attachment_paths` (optional): Comma-separated file paths

### search_messages
Search for messages using IMAP criteria.

**Arguments:**
- `mailbox` (optional): Mailbox to search (default: INBOX)
- `query` (required): Search query (e.g., "FROM user@example.com", "SUBJECT meeting", "UNSEEN")
- `limit` (optional): Max results (default: 10, max: 100)

### list_mailboxes
List all available mailboxes in the account.

**Arguments:** None

## Search Query Examples

- `UNSEEN` - Unread messages
- `SEEN` - Read messages
- `FROM user@example.com` - From specific sender
- `SUBJECT meeting` - Subject contains "meeting"
- Text search - Search in message content

## Common Mailboxes

- `INBOX` - Main inbox
- `Sent Messages` - Sent items
- `Drafts` - Draft messages
- `Trash` - Deleted items
- `Junk` - Spam folder
- `Archive` - Archived messages

## Troubleshooting

### Authentication Errors

If you get authentication errors:
1. Verify you're using an **App-Specific Password**, not your regular iCloud password
2. Check that two-factor authentication is enabled on your Apple ID
3. Ensure your email and password are correctly set in the config file or environment variables

### Connection Issues

iCloud Mail servers:
- IMAP: `imap.mail.me.com:993` (TLS)
- SMTP: `smtp.mail.me.com:587` (STARTTLS)

### Logs

Check logs for debugging:
```bash
tail -f ~/.hunter3/logs/mcp-imail.log
```

## Security Notes

1. **Never commit your App-Specific Password to version control**
2. Store credentials securely in `~/.hunter3/icloud-mail.json` with 600 permissions
3. App-Specific Passwords can be revoked at any time from appleid.apple.com
4. Each app should use its own App-Specific Password

## Differences from Gmail Plugin

- Uses IMAP/SMTP protocols instead of REST API
- Requires App-Specific Password (no OAuth flow)
- Messages identified by sequence number (not message ID)
- Search uses IMAP criteria (not Gmail's advanced search syntax)
- Simpler setup (no Google Cloud Console configuration)

## References

- [iCloud Mail IMAP/SMTP Settings](https://support.apple.com/en-us/HT202304)
- [App-Specific Passwords](https://support.apple.com/en-us/HT204397)
- [IMAP Protocol RFC 3501](https://tools.ietf.org/html/rfc3501)
