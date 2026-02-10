# iCloud Mail MCP Plugin - Example Usage

## Setup

First, create your configuration:

```bash
mkdir -p ~/.hunter3
cat > ~/.hunter3/icloud-mail.json << 'EOF'
{
  "email": "yourname@icloud.com",
  "password": "abcd-efgh-ijkl-mnop"
}
EOF
chmod 600 ~/.hunter3/icloud-mail.json
```

Build and start the plugin:

```bash
make all
./dist/mcp-imail
```

## Example Commands

### 1. List Recent Emails

**Natural Language:**
> List my recent iCloud emails

**What it does:**
- Fetches the last 10 messages from INBOX
- Shows sender, subject, date
- Indicates read/unread status

**Output:**
```
Found 10 message(s):

1. Seq: 145, UID: 3421
   From: alice@example.com
   Subject: Meeting Tomorrow
   Date: 2024-01-15 10:30:00

2. [READ] Seq: 144, UID: 3420
   From: bob@company.com
   Subject: Project Update
   Date: 2024-01-15 09:15:00
...
```

### 2. List From Sent Folder

**Natural Language:**
> Show my sent emails from iCloud

**Parameters:**
```json
{
  "mailbox": "Sent Messages",
  "limit": "5"
}
```

### 3. Read a Specific Email

**Natural Language:**
> Read iCloud email with sequence number 145

**Output:**
```
=== Email Message ===

From: alice@example.com
To: yourname@icloud.com
Subject: Meeting Tomorrow
Date: 2024-01-15 10:30:00

=== Body ===
Hi,

Can we meet tomorrow at 2pm to discuss the project?

Thanks,
Alice
```

### 4. Search for Unread Messages

**Natural Language:**
> Search for unread iCloud emails

**Parameters:**
```json
{
  "query": "UNSEEN"
}
```

### 5. Search by Sender

**Natural Language:**
> Search iCloud emails FROM alice@example.com

**Parameters:**
```json
{
  "query": "FROM alice@example.com",
  "limit": "20"
}
```

### 6. Search by Subject

**Natural Language:**
> Find iCloud emails about "project update"

**Parameters:**
```json
{
  "query": "SUBJECT project update"
}
```

### 7. Send a Simple Email

**Natural Language:**
> Send an email via iCloud to bob@example.com with subject "Quick Question" and body "Hey Bob, can you send me that file?"

**Result:**
```
Email sent successfully to bob@example.com
```

### 8. Send Email with CC

**Natural Language:**
> Send an email via iCloud to alice@example.com, CC bob@example.com, subject "Team Meeting" and body "Team meeting at 3pm today"

**Parameters:**
```json
{
  "to": "alice@example.com",
  "cc": "bob@example.com",
  "subject": "Team Meeting",
  "body": "Team meeting at 3pm today"
}
```

### 9. Send HTML Email

**Natural Language:**
> Send an HTML email via iCloud to alice@example.com with subject "Newsletter" and body "<h1>Hello!</h1><p>Check out our <b>new features</b>!</p>"

**Parameters:**
```json
{
  "to": "alice@example.com",
  "subject": "Newsletter",
  "body": "<h1>Hello!</h1><p>Check out our <b>new features</b>!</p>",
  "is_html": "true"
}
```

### 10. Send Email with Attachment

**Natural Language:**
> Send an email via iCloud to alice@example.com with subject "Report" and body "Please see attached report" and attach /home/user/report.pdf

**Parameters:**
```json
{
  "to": "alice@example.com",
  "subject": "Report",
  "body": "Please see attached report",
  "attachment_paths": "/home/user/report.pdf"
}
```

### 11. Send Email with Multiple Attachments

**Natural Language:**
> Send an email via iCloud to team@example.com with subject "Documents" and body "Here are the files" and attach /home/user/doc1.pdf,/home/user/doc2.xlsx

**Parameters:**
```json
{
  "to": "team@example.com",
  "subject": "Documents",
  "body": "Here are the files",
  "attachment_paths": "/home/user/doc1.pdf,/home/user/doc2.xlsx"
}
```

### 12. List All Mailboxes

**Natural Language:**
> Show my iCloud mailboxes

**Output:**
```
Available mailboxes:

- INBOX
- Sent Messages
- Drafts
- Trash
- Junk
- Archive
- Notes
```

## Advanced IMAP Search Examples

### Search Read Messages
```json
{
  "query": "SEEN"
}
```

### Search by Date Range
```json
{
  "query": "SINCE 01-Jan-2024"
}
```

### Complex Text Search
```json
{
  "query": "invoice"
}
```
This searches for "invoice" in the entire message (subject, body, etc.)

## Common Workflows

### Morning Email Check
```
1. Search for unread iCloud emails
2. Read iCloud email with sequence number X (for each interesting one)
3. Reply by sending an email via iCloud
```

### Email Cleanup
```
1. List my iCloud mailboxes
2. Search iCloud emails FROM spam@example.com
3. Delete or move messages (future feature)
```

### Send Weekly Report
```
1. Send an email via iCloud to team@company.com
   - Subject: "Weekly Report"
   - Body: HTML formatted report
   - Attach: report.pdf, data.xlsx
   - CC: manager@company.com
```

## Tips

1. **Sequence Numbers Change**: When emails are deleted, sequence numbers shift. Use UID if you need stable identifiers.

2. **Mailbox Names**: Use quotes if mailbox names have spaces:
   - `"Sent Messages"`
   - `"Deleted Items"`

3. **Search Syntax**: IMAP search is case-insensitive for keywords (UNSEEN, SEEN, FROM, SUBJECT)

4. **Attachments**: Use absolute paths for attachment files

5. **HTML Emails**: For HTML content, set `is_html: "true"`

6. **Multiple Recipients**: Separate with commas: `"alice@example.com,bob@example.com"`

## Troubleshooting

### "Login failed"
- Check you're using an App-Specific Password, not your regular password
- Verify the password in `~/.hunter3/icloud-mail.json`

### "Mailbox not found"
- Run "List my iCloud mailboxes" to see available names
- Use exact names (case-sensitive)

### "Message not found"
- Sequence numbers change when messages are deleted
- List messages again to get current sequence numbers

### Can't Send Email
- Verify SMTP settings (smtp.mail.me.com:587)
- Check App-Specific Password has necessary permissions
- Ensure recipients are valid email addresses
