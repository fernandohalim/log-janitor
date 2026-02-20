# log janitor

a lightweight go microservice to archive and clean up old log files on windows servers. 

this tool uses a dual-retention system:
1. keeps recent logs untouched.
2. zips and archives logs that pass the first age threshold.
3. permanently deletes logs (even from inside the zip backup) once they pass the second age threshold.



## features
* **dual retention:** separate rules for archiving vs. permanent deletion.
* **smart archiving:** merges old logs into existing backup zips and automatically purges expired logs from within the zip.
* **json configured:** update target directories and retention periods without recompiling.
* **zero dependencies:** runs as a single compiled `.exe`, perfect for windows task scheduler.

## installation & build
1. clone the repository.
2. compile the executable:
```bash
go build -o log-janitor.exe main.go