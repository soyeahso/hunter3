# MCP Google Drive Plugin

A Model Context Protocol (MCP) server that provides Google Drive integration for file management operations.

## Features

- **List Files**: Browse files and folders in Google Drive with optional filtering
- **File Information**: Get detailed metadata about files and folders
- **Download Files**: Download files from Google Drive to local storage
- **Upload Files**: Upload files from local storage to Google Drive
- **Create Folders**: Create new folders in Google Drive
- **Delete Files**: Delete files and folders (moves to trash)
- **Search Files**: Search for files using Google Drive's query syntax
- **Share Files**: Share files with specific users or make them publicly accessible

## Setup

### 1. Create Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Drive API for your project

### 2. Create OAuth 2.0 Credentials

1. Go to APIs & Services > Credentials
2. Click "Create Credentials" > "OAuth client ID"
3. Select "Desktop app" as the application type
4. Download the credentials JSON file
5. Save it as `~/.hunter3/gdrive-credentials.json`

### 3. Build and Run

```bash
# Build the plugin
make mcp-gdrive

# Register with Claude CLI (run once)
make mcp-register
```

### 4. First-Time Authentication

The first time you run the plugin, it will:
1. Open a browser window for Google authentication
2. Ask you to grant permissions to access your Google Drive
3. Save the authentication token to `~/.hunter3/gdrive-token.json`

## Environment Variables

- `GDRIVE_CREDENTIALS_FILE`: Path to the OAuth credentials file (default: `~/.hunter3/gdrive-credentials.json`)

## Available Tools

### list_files

List files and folders in Google Drive.

**Parameters:**
- `query` (optional): Search query using Google Drive query syntax
- `max_results` (optional): Maximum number of files to return (default: 20, max: 100)
- `folder_id` (optional): List files in a specific folder

**Examples:**
```
List all files: {}
List PDFs only: {"query": "mimeType = 'application/pdf'"}
List files in a folder: {"folder_id": "1ABC...XYZ"}
List recent files: {"query": "modifiedTime > '2024-01-01'"}
```

### get_file_info

Get detailed information about a specific file or folder.

**Parameters:**
- `file_id` (required): The ID of the file or folder

**Example:**
```json
{"file_id": "1ABC...XYZ"}
```

### download_file

Download a file from Google Drive.

**Parameters:**
- `file_id` (required): The ID of the file to download
- `output_path` (optional): Local path to save the file

**Examples:**
```
Download text file (view content): {"file_id": "1ABC...XYZ"}
Download and save: {"file_id": "1ABC...XYZ", "output_path": "/tmp/file.pdf"}
```

### upload_file

Upload a file to Google Drive.

**Parameters:**
- `file_path` (required): Local path to the file to upload
- `name` (optional): Name for the file in Google Drive
- `folder_id` (optional): ID of the folder to upload to
- `description` (optional): Description for the file

**Example:**
```json
{
  "file_path": "/tmp/report.pdf",
  "name": "Monthly Report",
  "folder_id": "1ABC...XYZ",
  "description": "Q1 2024 Report"
}
```

### create_folder

Create a new folder in Google Drive.

**Parameters:**
- `name` (required): Name of the folder
- `parent_id` (optional): ID of the parent folder
- `description` (optional): Description for the folder

**Example:**
```json
{
  "name": "Projects",
  "parent_id": "1ABC...XYZ",
  "description": "Work projects folder"
}
```

### delete_file

Delete a file or folder (moves to trash).

**Parameters:**
- `file_id` (required): The ID of the file or folder to delete

**Example:**
```json
{"file_id": "1ABC...XYZ"}
```

### search_files

Search for files using advanced query syntax.

**Parameters:**
- `query` (required): Search query
- `max_results` (optional): Maximum number of results (default: 20, max: 100)

**Examples:**
```
Search by content: {"query": "fullText contains 'meeting notes'"}
Search by name: {"query": "name contains 'budget'"}
Search large files: {"query": "mimeType = 'application/pdf' and size > 1000000"}
```

### share_file

Share a file or folder with specific users or make it publicly accessible.

**Parameters:**
- `file_id` (required): The ID of the file or folder to share
- `email` (optional): Email address to share with
- `role` (optional): Permission role (reader, writer, commenter, owner; default: reader)
- `type` (optional): Permission type (user, group, domain, anyone; default: user)

**Examples:**
```
Share with user: {"file_id": "1ABC...XYZ", "email": "user@example.com", "role": "writer"}
Make public: {"file_id": "1ABC...XYZ", "type": "anyone", "role": "reader"}
```

## Google Drive Query Syntax

The plugin supports Google Drive's advanced query syntax:

- `name = 'filename'` - Exact name match
- `name contains 'text'` - Name contains text
- `mimeType = 'application/pdf'` - Filter by MIME type
- `fullText contains 'text'` - Search file content
- `modifiedTime > '2024-01-01'` - Files modified after date
- `trashed = false` - Exclude trashed files
- `'parentID' in parents` - Files in specific folder

Combine queries with `and`, `or`, and `not`:
```
name contains 'report' and mimeType = 'application/pdf' and not trashed
```

## Troubleshooting

### Authentication Issues

If you get authentication errors:
1. Delete `~/.hunter3/gdrive-token.json`
2. Run the plugin again to re-authenticate

### Permission Errors

Make sure your OAuth credentials have the following scopes enabled:
- `https://www.googleapis.com/auth/drive`
- `https://www.googleapis.com/auth/drive.file`
- `https://www.googleapis.com/auth/drive.metadata.readonly`

### File Not Found

When referencing files, always use their Google Drive ID (not the name).
You can get the ID from the file's URL or by using `list_files` or `search_files`.

## Logs

Logs are written to `~/.hunter3/logs/mcp-gdrive.log`

View logs:
```bash
tail -f ~/.hunter3/logs/mcp-gdrive.log
```

## Security Notes

- OAuth credentials and tokens are stored locally in `~/.hunter3/`
- Tokens are automatically refreshed when they expire
- Never commit credentials to version control
- The plugin only requests the permissions it needs
