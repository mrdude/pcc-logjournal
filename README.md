# pcc-logjournal

A library for managing `logjournal`s.

A `logjournal` is a garbage-collected, append-only list of timestamped `[]byte`'s,
split into multiple physical files in a single directory.

## Use cases
I've used this code in my central log collector; each machine in my cluster
pushes it logs to a central daemon that stores them in a `logjournal` for querying.

