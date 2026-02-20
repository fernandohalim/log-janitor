# log janitor

a lightweight, zero-dependency go microservice designed to archive and clean up old log files on windows servers. 

it scans specified directories, finds files older than a configured retention period, merges them into a zip backup, and then deletes the original files to free up disk space.

## features
* **json configured:** easily update target directories and retention days without recompiling.
* **smart archiving:** zips old logs and automatically merges them if a backup zip already exists for that folder.
* **audit logging:** writes every action (zip, delete, errors) to a local log file.
* **no background runtime:** runs purely as a compiled `.exe`, perfect for windows task scheduler.

## installation & build
1. clone the repository.
2. open your terminal in the project folder.
3. compile the executable:
```bash
go build -o log-janitor.exe main.go