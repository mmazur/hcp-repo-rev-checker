# repo-rev-checker

Extracts `ARO_HCP_REPO_REVISION` values and commit dates from pipelines repository branches.

## Usage

```bash
./repo-rev-checker.exe <repo_directory>
```

## Example Output

JSON with revision and commit date for main, staging, and production branches:

```json
{
  "int": {
    "repo_tip": "526f70d3d81f",
    "repo_tip_commit_date": "2025-09-24 02:55:10 +0000"
  },
  "stg": {
    "repo_tip": "526f70d3d81f",
    "repo_tip_commit_date": "2025-09-23 15:22:15 +0000"
  },
  "prod": {
    "repo_tip": "5e5a1bf7d9c0",
    "repo_tip_commit_date": "2025-09-23 17:28:32 +0200"
  }
}
```
