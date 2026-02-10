# Google Drive MCP Plugin - Quick Start

Get started with the Google Drive MCP plugin in 5 minutes!

## Step 1: Enable Google Drive API

1. Visit the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or use an existing one)
3. Go to **APIs & Services** > **Library**
4. Search for "Google Drive API"
5. Click **Enable**

## Step 2: Create OAuth Credentials

1. Go to **APIs & Services** > **Credentials**
2. Click **Create Credentials** > **OAuth client ID**
3. If prompted, configure the OAuth consent screen:
   - User Type: **External** (for personal use)
   - App name: "Hunter3 MCP Google Drive"
   - Add your email as a test user
4. Application type: **Desktop app**
5. Name: "Hunter3 MCP"
6. Click **Create**
7. Click **Download JSON**

## Step 3: Install Credentials

```bash
# Create the hunter3 config directory
mkdir -p ~/.hunter3

# Copy your downloaded credentials file
cp ~/Downloads/client_secret_*.json ~/.hunter3/gdrive-credentials.json
```

## Step 4: Build and Register

```bash
# Build the plugin
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
make mcp-gdrive

# Register with Claude CLI
claude mcp add --transport stdio mcp-gdrive -- $(pwd)/dist/mcp-gdrive
```

## Step 5: Test It

Start Claude CLI and try:

```
Can you list the files in my Google Drive?
```

On first use, you'll be prompted to:
1. Visit a URL in your browser
2. Grant permissions to the app
3. Copy the authorization code
4. Paste it into the terminal

The token will be saved to `~/.hunter3/gdrive-token.json` for future use.

## Quick Examples

### List files
```
Show me all PDF files in my Drive
```

### Download a file
```
Download the file with ID abc123xyz and save it to /tmp/document.pdf
```

### Upload a file
```
Upload /tmp/report.pdf to my Google Drive
```

### Create a folder
```
Create a folder called "Work Projects" in my Google Drive
```

### Search files
```
Search my Drive for files containing "meeting notes"
```

### Share a file
```
Share the file with ID abc123xyz with john@example.com as a reader
```

## Troubleshooting

### "Credentials file not found"
- Make sure you copied the credentials to `~/.hunter3/gdrive-credentials.json`
- Check the file path with: `ls -la ~/.hunter3/gdrive-credentials.json`

### "Access blocked: This app's request is invalid"
- Make sure you configured the OAuth consent screen
- Add yourself as a test user in the OAuth consent screen settings

### "Invalid authentication"
- Delete the token: `rm ~/.hunter3/gdrive-token.json`
- Try again - you'll be prompted to re-authenticate

## Next Steps

- Read the [full README](README.md) for advanced features
- Check out [example-usage.md](example-usage.md) for more examples
- View logs: `tail -f ~/.hunter3/logs/mcp-gdrive.log`

## Getting File IDs

To work with specific files, you need their Google Drive ID:

**Method 1: From URL**
```
https://drive.google.com/file/d/1ABC...XYZ/view
                              ^^^^^^^^^^^
                              This is the file ID
```

**Method 2: Using list_files**
```
List all my PDF files
```
The output will show each file's ID.
