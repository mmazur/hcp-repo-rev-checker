# repo-rev-checker

Extracts `ARO_HCP_REPO_REVISION` values and commit dates from pipelines repository branches.

## Usage

```bash
./repo-rev-checker.exe <repo_directory>
```

### Options

- `--quick, -q`: Skip git fetch/reset operations and use repository as-is. This is faster but uses the current state of the repository without pulling latest changes from remote.
- `--envs, -e`: Comma-separated list of environments to analyze (int,stg,prod). If not specified, all environments are processed.
  - Examples:
    - `-e int` - Only analyze the integration environment
    - `-e int,stg` - Analyze integration and staging environments
    - `-e prod` - Only analyze the production environment
- `--days, -d`: Number of days to look back in commit history for Revision.mk changes. If 0 (default), only checks the tip commit. When specified, includes all commits that modified Revision.mk in the last N days.
  - Examples:
    - `-d 7` - Include all Revision.mk changes from the last 7 days
    - `-d 30` - Include all Revision.mk changes from the last 30 days
    - Note: The tip commit is always included as the first entry, regardless of when it was made

## Example Output

### Default behavior (tip only)
JSON with arrays for main, staging, and production branches. Each array contains objects with repo_revision and commit_date fields (dates in UTC):

```json
{
  "int": [
    {
      "repo_revision": "526f70d3d81f",
      "commit_date": "2025-09-24 02:55:10 +0000"
    }
  ],
  "stg": [
    {
      "repo_revision": "526f70d3d81f",
      "commit_date": "2025-09-23 13:22:15 +0000"
    }
  ],
  "prod": [
    {
      "repo_revision": "5e5a1bf7d9c0",
      "commit_date": "2025-09-23 15:28:32 +0000"
    }
  ]
}
```

### With --days option
When using `--days/-d`, multiple entries are included for each environment, with the tip commit always first:

```json
{
  "int": [
    {
      "repo_revision": "526f70d3d81f",
      "commit_date": "2025-09-24 02:55:10 +0000"
    },
    {
      "repo_revision": "abc123456789",
      "commit_date": "2025-09-23 10:30:45 +0000"
    },
    {
      "repo_revision": "def987654321",
      "commit_date": "2025-09-22 14:20:15 +0000"
    }
  ]
}
```
