# MCP Gmail Plugin - Example Usage

This document provides practical examples of using the MCP Gmail plugin.

## Example 1: List Recent Unread Emails

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_messages",
    "arguments": {
      "query": "is:unread",
      "max_results": "10"
    }
  }
}
```

## Example 2: Read a Specific Email

First, get the message ID from listing messages, then:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "read_message",
    "arguments": {
      "message_id": "18d4f2c3a1b2c3d4"
    }
  }
}
```

## Example 3: Send a Simple Email

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "send_message",
    "arguments": {
      "to": "colleague@example.com",
      "subject": "Meeting Notes",
      "body": "Hi team,\n\nHere are the notes from today's meeting:\n\n- Topic 1\n- Topic 2\n- Topic 3\n\nBest regards"
    }
  }
}
```

## Example 4: Send HTML Email with Attachment

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "send_message",
    "arguments": {
      "to": "manager@example.com",
      "cc": "team@example.com",
      "subject": "Q4 Report",
      "body": "<html><body><h1>Q4 Report</h1><p>Please find the quarterly report attached.</p><ul><li>Revenue: $1M</li><li>Growth: 25%</li></ul></body></html>",
      "is_html": "true",
      "attachment_paths": "/home/user/reports/q4-report.pdf"
    }
  }
}
```

## Example 5: Send Email with Multiple Attachments

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "send_message",
    "arguments": {
      "to": "client@example.com",
      "subject": "Project Deliverables",
      "body": "Dear Client,\n\nPlease find attached the project deliverables as discussed.\n\nThank you.",
      "attachment_paths": "/home/user/projects/design.pdf,/home/user/projects/code.zip,/home/user/projects/documentation.docx"
    }
  }
}
```

## Example 6: Search for Emails from Specific Sender

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "search_messages",
    "arguments": {
      "query": "from:boss@company.com is:unread",
      "max_results": "5"
    }
  }
}
```

## Example 7: Search for Large Attachments

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "search_messages",
    "arguments": {
      "query": "has:attachment larger:5M",
      "max_results": "20"
    }
  }
}
```

## Example 8: Search by Date Range

```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "search_messages",
    "arguments": {
      "query": "after:2024/01/01 before:2024/02/01 subject:invoice",
      "max_results": "50"
    }
  }
}
```

## Example 9: List Starred Messages

```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "tools/call",
  "params": {
    "name": "list_messages",
    "arguments": {
      "query": "is:starred",
      "max_results": "15"
    }
  }
}
```

## Example 10: Send to Multiple Recipients

```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "send_message",
    "arguments": {
      "to": "person1@example.com,person2@example.com,person3@example.com",
      "cc": "supervisor@example.com",
      "bcc": "archive@example.com",
      "subject": "Team Update",
      "body": "Hi everyone,\n\nThis is an update for the entire team.\n\nBest regards"
    }
  }
}
```

## Testing the Plugin

To test the plugin, you can pipe these JSON-RPC messages to it:

```bash
# Initialize the connection
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./bin/mcp-gmail

# List tools
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ./bin/mcp-gmail

# List unread messages
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_messages","arguments":{"query":"is:unread","max_results":"5"}}}' | ./bin/mcp-gmail
```

## Common Gmail Search Operators

- `from:sender@example.com` - From specific sender
- `to:recipient@example.com` - To specific recipient
- `subject:keyword` - Subject contains keyword
- `is:unread` - Unread messages
- `is:read` - Read messages
- `is:starred` - Starred messages
- `is:important` - Important messages
- `has:attachment` - Has attachments
- `filename:pdf` - Specific file type
- `larger:5M` - Larger than 5MB
- `smaller:1M` - Smaller than 1MB
- `after:2024/01/01` - After date
- `before:2024/12/31` - Before date
- `category:primary` - Primary inbox
- `category:social` - Social category
- `category:promotions` - Promotions category
- `label:work` - Has label "work"

## Combining Search Operators

You can combine multiple operators:

```
from:boss@company.com is:unread has:attachment after:2024/01/01
```

This searches for unread emails from your boss with attachments sent after January 1, 2024.
