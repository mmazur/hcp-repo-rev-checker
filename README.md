# repo-rev-checker

Extracts `ARO_HCP_REPO_REVISION` values and commit dates from pipelines repository branches.

## Usage

```bash
./repo-rev-checker.exe <repo_directory>
```

### Options

- `--quick, -q`: Skip git fetch/reset operations and use repository as-is. This is faster but uses the current state of the repository without pulling latest changes from remote.

## Example Output

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
