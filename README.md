tag-monger
===

Utility for "retiring" eups distrib tags from `EUPS_PKGROOTS` contained in an
s3 bucket.

Docker Image
---

[`docker.io/lsstsqre/tag-monger`](https://hub.docker.com/r/lsstsqre/tag-monger/)

Usage
---

```
error: Usage:
  tag-monger

Application Options:
  -v, --verbose   Show verbose debug information [$TAG_MONGER_VERBOSE]
  -p, --pagesize= page size of s3 object listing (default: 100) [$TAG_MONGER_PAGESIZE]
  -m, --max=      maximum number of s3 object to list (default: 1000) [$TAG_MONGER_MAX]
  -b, --bucket=   name of s3 bucket [$TAG_MONGER_BUCKET]
  -d, --days=     Expire tags older than N days (default: 30) [$TAG_MONGER_DAYS]
  -n, --noop      Do not make any changes [$TAG_MONGER_NOOP]

Help Options:
  -h, --help      Show this help message
```
