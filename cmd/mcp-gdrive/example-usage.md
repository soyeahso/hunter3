# Google Drive MCP Plugin - Example Usage

This document provides practical examples of using the Google Drive MCP plugin.

## Basic File Operations

### List all files in your Drive

**Request:**
```
List the files in my Google Drive
```

**Tool call:**
```json
{
  "name": "list_files",
  "arguments": {}
}
```

### List files with filters

**Request:**
```
Show me all PDF files in my Drive
```

**Tool call:**
```json
{
  "name": "list_files",
  "arguments": {
    "query": "mimeType = 'application/pdf'",
    "max_results": "50"
  }
}
```

### List files in a specific folder

**Request:**
```
List files in the folder with ID 1ABC...XYZ
```

**Tool call:**
```json
{
  "name": "list_files",
  "arguments": {
    "folder_id": "1ABC...XYZ"
  }
}
```

## Searching Files

### Search by filename

**Request:**
```
Find all files with "budget" in the name
```

**Tool call:**
```json
{
  "name": "search_files",
  "arguments": {
    "query": "name contains 'budget'"
  }
}
```

### Search by content

**Request:**
```
Search for files containing the text "quarterly report"
```

**Tool call:**
```json
{
  "name": "search_files",
  "arguments": {
    "query": "fullText contains 'quarterly report'"
  }
}
```

### Search recently modified files

**Request:**
```
Show me files modified in the last week
```

**Tool call:**
```json
{
  "name": "search_files",
  "arguments": {
    "query": "modifiedTime > '2024-02-01'"
  }
}
```

### Complex search queries

**Request:**
```
Find large PDF files created this year
```

**Tool call:**
```json
{
  "name": "search_files",
  "arguments": {
    "query": "mimeType = 'application/pdf' and size > 5000000 and createdTime > '2024-01-01'"
  }
}
```

## File Information

### Get detailed file info

**Request:**
```
Get information about the file with ID 1ABC...XYZ
```

**Tool call:**
```json
{
  "name": "get_file_info",
  "arguments": {
    "file_id": "1ABC...XYZ"
  }
}
```

## Downloading Files

### Download and view a text file

**Request:**
```
Download and show me the content of file 1ABC...XYZ
```

**Tool call:**
```json
{
  "name": "download_file",
  "arguments": {
    "file_id": "1ABC...XYZ"
  }
}
```

### Download and save a file

**Request:**
```
Download file 1ABC...XYZ and save it to /tmp/document.pdf
```

**Tool call:**
```json
{
  "name": "download_file",
  "arguments": {
    "file_id": "1ABC...XYZ",
    "output_path": "/tmp/document.pdf"
  }
}
```

## Uploading Files

### Simple upload

**Request:**
```
Upload the file /home/user/report.pdf to my Google Drive
```

**Tool call:**
```json
{
  "name": "upload_file",
  "arguments": {
    "file_path": "/home/user/report.pdf"
  }
}
```

### Upload with custom name

**Request:**
```
Upload /tmp/data.csv to Drive as "Sales Data Q1"
```

**Tool call:**
```json
{
  "name": "upload_file",
  "arguments": {
    "file_path": "/tmp/data.csv",
    "name": "Sales Data Q1"
  }
}
```

### Upload to specific folder

**Request:**
```
Upload /tmp/report.pdf to the folder with ID 1ABC...XYZ
```

**Tool call:**
```json
{
  "name": "upload_file",
  "arguments": {
    "file_path": "/tmp/report.pdf",
    "folder_id": "1ABC...XYZ",
    "description": "Monthly sales report"
  }
}
```

## Folder Management

### Create a folder

**Request:**
```
Create a folder called "Work Projects" in my Drive
```

**Tool call:**
```json
{
  "name": "create_folder",
  "arguments": {
    "name": "Work Projects",
    "description": "All work-related projects"
  }
}
```

### Create a subfolder

**Request:**
```
Create a folder called "2024" inside folder 1ABC...XYZ
```

**Tool call:**
```json
{
  "name": "create_folder",
  "arguments": {
    "name": "2024",
    "parent_id": "1ABC...XYZ"
  }
}
```

## Sharing Files

### Share with a specific user

**Request:**
```
Share file 1ABC...XYZ with john@example.com as a reader
```

**Tool call:**
```json
{
  "name": "share_file",
  "arguments": {
    "file_id": "1ABC...XYZ",
    "email": "john@example.com",
    "role": "reader"
  }
}
```

### Share with edit permissions

**Request:**
```
Share file 1ABC...XYZ with jane@example.com and let her edit it
```

**Tool call:**
```json
{
  "name": "share_file",
  "arguments": {
    "file_id": "1ABC...XYZ",
    "email": "jane@example.com",
    "role": "writer"
  }
}
```

### Make a file public

**Request:**
```
Make file 1ABC...XYZ publicly accessible
```

**Tool call:**
```json
{
  "name": "share_file",
  "arguments": {
    "file_id": "1ABC...XYZ",
    "type": "anyone",
    "role": "reader"
  }
}
```

## Deleting Files

### Delete a file

**Request:**
```
Delete the file with ID 1ABC...XYZ
```

**Tool call:**
```json
{
  "name": "delete_file",
  "arguments": {
    "file_id": "1ABC...XYZ"
  }
}
```

**Note:** This moves the file to trash. It can still be recovered from the Google Drive trash.

## Advanced Workflows

### Backup workflow

1. List important files:
```json
{"name": "search_files", "arguments": {"query": "name contains 'important'"}}
```

2. Download each file:
```json
{"name": "download_file", "arguments": {"file_id": "...", "output_path": "/backup/file1.pdf"}}
```

### Organization workflow

1. Create folder structure:
```json
{"name": "create_folder", "arguments": {"name": "Archive 2024"}}
```

2. Find old files:
```json
{"name": "search_files", "arguments": {"query": "modifiedTime < '2024-01-01'"}}
```

3. Move files (share to new folder, then delete from old location)

### Collaboration workflow

1. Create project folder:
```json
{"name": "create_folder", "arguments": {"name": "Team Project"}}
```

2. Upload project files:
```json
{"name": "upload_file", "arguments": {"file_path": "/tmp/project.pdf", "folder_id": "..."}}
```

3. Share with team:
```json
{"name": "share_file", "arguments": {"file_id": "...", "email": "team@example.com", "role": "writer"}}
```

## MIME Types Reference

Common MIME types for filtering:

- PDF: `application/pdf`
- Word: `application/vnd.openxmlformats-officedocument.wordprocessingml.document`
- Excel: `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
- PowerPoint: `application/vnd.openxmlformats-officedocument.presentationml.presentation`
- Image: `image/jpeg`, `image/png`, `image/gif`
- Text: `text/plain`
- Folder: `application/vnd.google-apps.folder`
- Google Docs: `application/vnd.google-apps.document`
- Google Sheets: `application/vnd.google-apps.spreadsheet`
- Google Slides: `application/vnd.google-apps.presentation`

## Query Operators

- `=` : Equals
- `!=` : Not equals
- `<` : Less than
- `<=` : Less than or equal to
- `>` : Greater than
- `>=` : Greater than or equal to
- `contains` : Contains substring
- `in` : Contains (for arrays)
- `and` : Logical AND
- `or` : Logical OR
- `not` : Logical NOT

## Tips and Best Practices

1. **Get File IDs First**: Use `list_files` or `search_files` to find file IDs before downloading or sharing.

2. **Use Filters**: When listing files, use queries to narrow down results and improve performance.

3. **Check File Types**: Use `get_file_info` to check the file type before downloading.

4. **Organize with Folders**: Create a clear folder structure for better organization.

5. **Test Queries**: Start with simple queries and gradually make them more complex.

6. **Monitor Quotas**: Be aware of Google Drive API quotas when performing bulk operations.

7. **Use Descriptive Names**: When uploading files or creating folders, use clear, descriptive names.

8. **Back Up Important Files**: Regularly download critical files to local storage.
