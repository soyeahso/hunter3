# MCP GitHub CLI Plugin

An MCP (Model Context Protocol) server that provides tools for interacting with GitHub via the `gh` CLI.

## Overview

This plugin wraps the GitHub CLI (`gh`) command to provide comprehensive GitHub operations through MCP. It supports repository management, issues, pull requests, workflows, releases, gists, authentication, and more.

## Prerequisites

- GitHub CLI (`gh`) must be installed and configured
- Authenticate with `gh auth login` before using this plugin

## Installation

Build the plugin:
```bash
make mcp-gh
```

Register with Claude CLI:
```bash
claude mcp add --transport stdio mcp-gh -- $(pwd)/dist/mcp-gh
```

Or build and register all plugins:
```bash
make mcp-all
make mcp-register
```

## Environment Variables

- `HUNTER3_GH_ALLOWED_PATHS`: Comma-separated list of allowed directories for gh operations (defaults to `$HOME`)

Example:
```bash
export HUNTER3_GH_ALLOWED_PATHS="/home/user/projects,/home/user/repos"
```

## Available Tools

### Repository Operations

- **gh_repo_view** - View repository information
- **gh_repo_clone** - Clone a repository locally
- **gh_repo_create** - Create a new repository
- **gh_repo_fork** - Fork a repository
- **gh_repo_list** - List repositories for a user or organization

### Issue Operations

- **gh_issue_list** - List issues in a repository
- **gh_issue_view** - View an issue
- **gh_issue_create** - Create a new issue
- **gh_issue_close** - Close an issue
- **gh_issue_reopen** - Reopen an issue

### Pull Request Operations

- **gh_pr_list** - List pull requests in a repository
- **gh_pr_view** - View a pull request
- **gh_pr_create** - Create a pull request
- **gh_pr_checkout** - Check out a pull request locally
- **gh_pr_merge** - Merge a pull request
- **gh_pr_close** - Close a pull request
- **gh_pr_review** - Add a review to a pull request
- **gh_pr_diff** - View changes in a pull request

### Workflow/Actions Operations

- **gh_run_list** - List workflow runs
- **gh_run_view** - View a workflow run
- **gh_run_rerun** - Rerun a workflow run
- **gh_workflow_list** - List workflows in a repository
- **gh_workflow_run** - Trigger a workflow run

### Release Operations

- **gh_release_list** - List releases in a repository
- **gh_release_view** - View a release
- **gh_release_create** - Create a new release
- **gh_release_download** - Download release assets

### Gist Operations

- **gh_gist_list** - List gists
- **gh_gist_view** - View a gist
- **gh_gist_create** - Create a new gist

### Authentication Operations

- **gh_auth_status** - View authentication status
- **gh_auth_login** - Authenticate with GitHub

### Search Operations

- **gh_search_repos** - Search for repositories
- **gh_search_issues** - Search for issues and pull requests

### API Operations

- **gh_api** - Make an authenticated GitHub API request

## Usage Examples

### View a Repository
```json
{
  "name": "gh_repo_view",
  "arguments": {
    "repo": "owner/repo"
  }
}
```

### Create an Issue
```json
{
  "name": "gh_issue_create",
  "arguments": {
    "repository_path": "/path/to/repo",
    "title": "Bug report",
    "body": "Description of the bug",
    "label": ["bug", "urgent"]
  }
}
```

### List Pull Requests
```json
{
  "name": "gh_pr_list",
  "arguments": {
    "repository_path": "/path/to/repo",
    "state": "open",
    "limit": 10
  }
}
```

### Create a Pull Request
```json
{
  "name": "gh_pr_create",
  "arguments": {
    "repository_path": "/path/to/repo",
    "title": "New feature",
    "body": "Description of changes",
    "base": "main",
    "head": "feature-branch"
  }
}
```

### Merge a Pull Request
```json
{
  "name": "gh_pr_merge",
  "arguments": {
    "repository_path": "/path/to/repo",
    "number": "123",
    "merge_method": "squash",
    "delete_branch": "true"
  }
}
```

### List Workflow Runs
```json
{
  "name": "gh_run_list",
  "arguments": {
    "repository_path": "/path/to/repo",
    "workflow": "CI",
    "limit": 20
  }
}
```

### Search Repositories
```json
{
  "name": "gh_search_repos",
  "arguments": {
    "query": "mcp in:name language:go",
    "limit": 10
  }
}
```

### Make API Request
```json
{
  "name": "gh_api",
  "arguments": {
    "endpoint": "/repos/owner/repo/issues",
    "method": "GET"
  }
}
```

## Response Format

All tools return a JSON result with the following structure:

```json
{
  "command": "gh command that was executed",
  "success": true,
  "stdout": "output from gh command",
  "stderr": "error output (if any)",
  "error": "error message (if failed)"
}
```

## Security

- All repository paths are validated against `HUNTER3_GH_ALLOWED_PATHS`
- The plugin respects GitHub CLI authentication and permissions
- Commands are executed with the permissions of the authenticated GitHub user

## Logging

Logs are written to `~/.hunter3/logs/mcp-gh.log`

To tail the logs:
```bash
tail -f ~/.hunter3/logs/mcp-gh.log
```

Or use the convenience target:
```bash
make tail_logs
```

## Common Use Cases

### Working with PRs
1. List open PRs: `gh_pr_list`
2. View a specific PR: `gh_pr_view`
3. Review the PR: `gh_pr_review`
4. Check out locally: `gh_pr_checkout`
5. Merge when ready: `gh_pr_merge`

### Managing Issues
1. List issues: `gh_issue_list`
2. Create new issue: `gh_issue_create`
3. View issue details: `gh_issue_view`
4. Close when resolved: `gh_issue_close`

### CI/CD Workflows
1. List workflows: `gh_workflow_list`
2. Trigger a workflow: `gh_workflow_run`
3. Check run status: `gh_run_list`
4. View run details: `gh_run_view`
5. Rerun if needed: `gh_run_rerun`

### Release Management
1. List releases: `gh_release_list`
2. Create new release: `gh_release_create`
3. Download assets: `gh_release_download`

## Troubleshooting

### Authentication Issues
If you get authentication errors:
```bash
gh auth status
gh auth login
```

### Permission Issues
Check that your repository path is within allowed directories:
```bash
echo $HUNTER3_GH_ALLOWED_PATHS
```

### Command Not Found
Ensure GitHub CLI is installed:
```bash
gh --version
```

Install if needed:
- macOS: `brew install gh`
- Linux: See https://github.com/cli/cli#installation
- Windows: See https://github.com/cli/cli#installation

## Contributing

To add new gh commands:
1. Add the tool definition in `handleListTools()`
2. Add the tool handler function
3. Add the case in `handleCallTool()`
4. Update this README

## License

Part of the Hunter3 project. See LICENSE in the root directory.
