---
title: Write-ahead log
date: 2026-10-29
tags: databases, storage, vector databases
---

# Write-ahead log

An in-memory index is easy to understand until the process stops halfway
through a write.

If a recent write exists only in memory, it is lost. If a data file is being
changed in place, it may be partly written. A database needs a way to recover
without guessing which state is valid.

A write-ahead log, or WAL, is the usual answer.

## Append first

A WAL is an append-only sequence of changes. Before the database treats a
write as durable, it records enough information to replay that write later.

The basic write path looks like this:

```cpp
Status Database::Upsert(Row row) {
  const Record record = Encode(row);

  RETURN_IF_ERROR(wal_.Append(record));
  RETURN_IF_ERROR(wal_.Sync());

  memtable_.Upsert(std::move(row));
  return Status::Ok();
}
```

The exact synchronization policy is a trade-off. Calling `Sync` for every
write gives a stronger durability boundary but costs more. Grouping several
writes before syncing improves throughput but can lose the most recent group
after a crash.

The important part is the order: the database does not rely on the in-memory
memtable as its only record of an acknowledged write.

## Flush

The memtable cannot grow forever. A flush writes its current rows to a new,
immutable segment:

```cpp
Status Database::Flush() {
  Segment segment = BuildSegment(memtable_.Snapshot());

  RETURN_IF_ERROR(segment.Write(data_dir_));
  RETURN_IF_ERROR(manifest_.Add(segment));

  memtable_.Clear();
  return wal_.Truncate();
}
```

The ordering details depend on the database, but the idea is consistent:
publish a complete immutable segment before discarding the log entries that it
contains.

Updates and deletes are newer records. A delete is usually represented by a
tombstone. Compaction later merges segments, retains the latest visible value,
and removes tombstones when older copies can no longer be returned.

## Recovery

After a crash, the database opens complete segments first, then replays the
WAL entries that were not yet flushed:

```cpp
Status Database::Recover() {
  RETURN_IF_ERROR(manifest_.Load());
  RETURN_IF_ERROR(OpenSegments(manifest_));

  for (const Record& record : wal_.ReadAll()) {
    memtable_.Apply(record);
  }
  return Status::Ok();
}
```

This is why append-only logs and immutable files fit together well. A crash
does not leave the database trying to interpret a partly changed search index.
It has complete segments and an ordered record of newer changes.

For the approximate-search side, see
[HNSW](/post/hnsw-for-vector-search/).
