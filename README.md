# CloudBucket

<p align="center">
  <img src="https://img.shields.io/badge/version-1.0.0-blue.svg" alt="Version">
  <img src="https://img.shields.io/badge/go-%3E%3D1.19-00ADD8.svg" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
  <img src="https://img.shields.io/badge/platform-windows%20%7C%20linux%20%7C%20macos-lightgrey.svg" alt="Platform">
</p>

<p align="center">
  <b>Multi-Cloud Storage Bucket Security Scanner</b><br>
  <sub>Discover misconfigured cloud storage buckets across AWS, GCP, Azure, and more</sub>
</p>

---

## Features

- **Multi-cloud support:**
  - Amazon S3 (AWS)
  - Google Cloud Storage (GCP)
  - Azure Blob Storage
  - Alibaba Cloud OSS
  - DigitalOcean Spaces

- **Comprehensive checks:**
  - Bucket existence
  - Public read access
  - Public list (directory listing)
  - Public write access (optional)
  - File enumeration

- **High performance:** Multi-threaded scanning
- **Flexible:** Scan single bucket or batch from wordlist
- **Safe defaults:** Write testing disabled by default

## Installation

### From Source
```bash
git clone https://github.com/a0x194/cloudbucket.git
cd cloudbucket
go build -o cloudbucket main.go
```

### Download Binary
Check the [Releases](https://github.com/a0x194/cloudbucket/releases) page for pre-built binaries.

## Usage

### Scan single bucket
```bash
./cloudbucket -b company-backup
```

### Scan with specific providers
```bash
./cloudbucket -b mydata -p aws,gcp
```

### Batch scan from wordlist
```bash
./cloudbucket -l buckets.txt -t 20
```

### Full scan with file listing
```bash
./cloudbucket -b company-data -files -max-files 50
```

### Check for write access (use carefully!)
```bash
./cloudbucket -b target-bucket -write
```

### Flags
| Flag | Description | Default |
|------|-------------|---------|
| `-b` | Single bucket name | - |
| `-l` | File containing bucket names | - |
| `-p` | Providers to check | all |
| `-t` | Number of threads | 10 |
| `-timeout` | Request timeout (seconds) | 10 |
| `-v` | Verbose output | false |
| `-files` | List files in accessible buckets | false |
| `-max-files` | Max files to list per bucket | 10 |
| `-write` | Check write access | false |
| `-o` | Output file | - |

## Supported Providers

| Provider | Flag Value | Regions Checked |
|----------|------------|-----------------|
| AWS S3 | `aws`, `s3` | Global |
| Google Cloud | `gcp`, `gcs`, `google` | Global |
| Azure Blob | `azure` | Global (common containers) |
| Alibaba OSS | `alibaba`, `aliyun`, `oss` | CN, HK, US |
| DigitalOcean | `do`, `digitalocean`, `spaces` | NYC, AMS, SGP, FRA, SFO |

## Example Output

```
   _____ _                 _ ____             _        _
  / ____| |               | |  _ \           | |      | |
 | |    | | ___  _   _  __| | |_) |_   _  ___| | _____| |_
 | |    | |/ _ \| | | |/ _' |  _ <| | | |/ __| |/ / _ \ __|
 | |____| | (_) | |_| | (_| | |_) | |_| | (__|   <  __/ |_
  \_____|_|\___/ \__,_|\__,_|____/ \__,_|\___|_|\_\___|\__|

  Cloud Storage Bucket Scanner v1.0.0
  Author: a0x194 | https://www.tryharder.space
  More tools: https://www.tryharder.space/tools/

[*] Scanning 1 bucket(s) across providers: all

[CRITICAL - PUBLIC WRITE] company-backup
  ├─ Provider: AWS S3
  ├─ URL: https://company-backup.s3.amazonaws.com
  ├─ Public Read: true
  ├─ Public List: true
  ├─ Public Write: true
  └─ Files found (5):
      • database_dump.sql
      • users.csv
      • config.json
      • secrets.env
      • backup.tar.gz

[*] Scan complete! Found 1 accessible bucket(s)
```

## Wordlist Generation Tips

Generate bucket names based on:
- Company name variations
- Common prefixes: `backup`, `data`, `dev`, `prod`, `staging`, `assets`
- Common suffixes: `-backup`, `-data`, `-public`, `-files`, `-assets`

Example wordlist:
```
companyname
companyname-backup
companyname-data
companyname-dev
companyname-prod
companyname-assets
company-backup
company-files
```

## Severity Levels

| Level | Condition | Risk |
|-------|-----------|------|
| 🔴 CRITICAL | Public Write | Data can be modified/deleted |
| 🔴 HIGH | Public List | Directory structure exposed |
| 🟡 MEDIUM | Public Read | Data can be accessed |
| ⚪ LOW | Exists only | Bucket name confirmed |

## Disclaimer

⚠️ **This tool is for authorized security testing only.**

- Only scan buckets you own or have explicit permission to test
- The `-write` flag creates temporary test files - use responsibly
- Unauthorized access to cloud storage is illegal

## Author

**a0x194** - [https://www.tryharder.space](https://www.tryharder.space)

More security tools: [https://www.tryharder.space/tools/](https://www.tryharder.space/tools/)

## License

MIT License - see [LICENSE](LICENSE) file for details.
