# repo-rev-checker

Extracts `ARO_HCP_REPO_REVISION` values and commit dates from pipelines repository branches.

## Usage

```bash
./repo-rev-checker.exe <repo_directory>
```

## Example Output

JSON with arrays for main, staging, and production branches. Each array contains objects with digest and commit_date fields (dates in UTC):

```json
{
  "int": [
    {
      "digest": "526f70d3d81f",
      "commit_date": "2025-09-24 02:55:10 +0000"
    }
  ],
  "stg": [
    {
      "digest": "526f70d3d81f",
      "commit_date": "2025-09-23 13:22:15 +0000"
    }
  ],
  "prod": [
    {
      "digest": "5e5a1bf7d9c0",
      "commit_date": "2025-09-23 15:28:32 +0000"
    }
  ]
}
```
