# MCP Gmail Plugin

A Model Context Protocol (MCP) server that provides Gmail integration capabilities including reading, sending, and searching emails with attachment support.

## Features

- **List Messages**: Browse your inbox with optional filtering
- **Read Messages**: Read full email content including headers and body
- **Send Messages**: Send emails with support for:
  - Plain text and HTML content
  - Multiple recipients (To, CC, BCC)
  - File attachments
- **Search Messages**: Advanced search using Gmail's query syntax

## Setup

### 1. Enable Gmail API

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Gmail API:
   - Navigate to "APIs & Services" > "Library"
   - Search for "Gmail API"
   - Click "Enable"

### 2. Create OAuth 2.0 Credentials

1. Go to "APIs & Services" > "Credentials"
2. Click "Create Credentials" > "OAuth client ID"
3. Choose "Desktop app" as the application type
4. Name it "Hunter3 MCP Gmail"
5. Download the credentials JSON file

### 3. Install Credentials

Save the downloaded credentials file to:
```
~/.hunter3/gmail-credentials.json
```

Or set a custom path using the environment variable:
```bash
export GMAIL_CREDENTIALS_FILE=/path/to/credentials.json
```

### 4. First Run Authentication

On first run, the plugin will:
1. Display an authentication URL
2. Open your browser to authorize the application
3. Save the authorization token to `~/.hunter3/gmail-token.json`

The token will be automatically refreshed as needed.

## Required OAuth Scopes

The plugin requires the following Gmail API scopes:
- `https://www.googleapis.com/auth/gmail.readonly` - Read emails
- `https://www.googleapis.com/auth/gmail.send` - Send emails
- `https://www.googleapis.com/auth/gmail.compose` - Create drafts and compose emails
- `https://www.googleapis.com/auth/gmail.modify` - Modify email labels and properties

## Usage

### List Messages

```json
{
  "name": "list_messages",
  "arguments": {
    "query": "is:unread",
    "max_results": "20"
  }
}
```

Query examples:
- `is:unread` - Unread messages
- `from:user@example.com` - From specific sender
- `subject:meeting` - Contains "meeting" in subject
- `has:attachment` - Has attachments
- `after:2024/01/01` - After specific date

### Read Message

```json
{
  "name": "read_message",
  "arguments": {
    "message_id": "18d4f2c3a1b2c3d4"
  }
}
```

### Send Message

Simple email:
```json
{
  "name": "send_message",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Hello from Hunter3",
    "body": "This is a test message."
  }
}
```

Email with CC, BCC, and attachments:
```json
{
  "name": "send_message",
  "arguments": {
    "to": "recipient@example.com",
    "cc": "copy@example.com",
    "bcc": "blind@example.com",
    "subject": "Report",
    "body": "<h1>Monthly Report</h1><p>Please find attached.</p>",
    "is_html": "true",
    "attachment_paths": "/path/to/report.pdf,/path/to/data.xlsx"
  }
}
```

### Search Messages

```json
{
  "name": "search_messages",
  "arguments": {
    "query": "has:attachment larger:10M",
    "max_results": "50"
  }
}
```

## Gmail Search Query Syntax

Common operators:
- `from:` - Sender email
- `to:` - Recipient email
- `subject:` - Subject line
- `is:unread` / `is:read` - Read status
- `is:starred` - Starred messages
- `has:attachment` - Has attachments
- `filename:` - Attachment filename
- `larger:` / `smaller:` - Size filters (e.g., `larger:10M`)
- `after:` / `before:` - Date filters (e.g., `after:2024/01/01`)
- `category:primary` / `category:social` / `category:promotions` - Gmail categories

## Building

```bash
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
go build -o bin/mcp-gmail ./cmd/mcp-gmail
```

## Running

```bash
./bin/mcp-gmail
```

The server communicates via JSON-RPC over stdin/stdout.

## Logging

Logs are written to:
```
~/.hunter3/logs/mcp-gmail.log
```

## Security Notes

- Never commit credentials files to version control
- The OAuth token is stored locally and provides access to your Gmail account
- File attachments are read from the local filesystem - ensure proper file permissions
- The plugin validates file paths for attachments to prevent unauthorized access

## Troubleshooting

### "unable to read credentials file"
Ensure `~/.hunter3/gmail-credentials.json` exists with valid OAuth 2.0 credentials from Google Cloud Console.

### "failed to initialize Gmail service"
Check that:
1. Gmail API is enabled in your Google Cloud project
2. OAuth credentials are correctly configured for "Desktop app"
3. Required scopes are included

### "authorization code" prompt appears repeatedly
Delete the token file and re-authenticate:
```bash
rm ~/.hunter3/gmail-token.json
```

### Attachment errors
Verify:
- File paths are absolute or relative to the current working directory
- Files exist and are readable
- File sizes are within Gmail's attachment limits (25MB)
