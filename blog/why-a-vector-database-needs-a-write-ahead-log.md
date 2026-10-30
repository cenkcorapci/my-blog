---
title: Write-ahead log
date: 2026-10-29
tags: databases, storage, vector databases
---

# Write-ahead log

A write-ahead log, or WAL, is the part of a database that makes a recent write
recoverable after a crash.

Without it, an in-memory write is gone when the process stops. Updating a data
file in place is not much better: a power loss can leave part of the old value
and part of the new one. The database needs an ordered record of changes that
it can replay.

## Record format

A WAL is append-only. Every record needs enough framing to find the next record
and enough validation to reject a torn or corrupted one.

```cpp
enum class RecordType : uint8_t {
  kUpsert,
  kDelete,
};

struct RecordHeader {
  uint32_t length;      // Number of bytes in the encoded payload.
  uint32_t checksum;    // CRC32C of type + sequence + payload.
  RecordType type;
  uint64_t sequence;    // Monotonically increasing per database.
};
```

The payload for an upsert normally contains an ID, timestamp or version,
vector, metadata, and tags. A delete contains an ID and a version. Length and
checksum let recovery stop safely when the last append was only partly written.

Sequence numbers are useful even when rows already have timestamps. They make
the log order explicit and help detect duplicate replay after a crash.

Encode these fields explicitly in a fixed byte order. Writing a native C++
struct directly to disk makes the format depend on padding, alignment, and
endianness.

## Write path

The rule is in the name: write the log before treating the change as durable.

```cpp
Status Database::Upsert(Row row) {
  Record record = EncodeUpsert(row, next_sequence_++);

  RETURN_IF_ERROR(wal_.Append(record));
  RETURN_IF_ERROR(wal_.Sync());

  memtable_.Upsert(std::move(row));
  return Status::Ok();
}
```

`Sync` asks the operating system to persist the appended bytes. Calling it for
every write gives a clear durability boundary but costs latency. Group commit
is a common alternative: append several records, sync once, then acknowledge
the group. A database may also offer a weaker asynchronous mode, but it should
say clearly that a crash can lose acknowledged writes.

Do not acknowledge the write before the selected durability policy has been
met. Also make replay idempotent: applying the same upsert twice should leave
the same visible row. Last-write-wins versions or sequence numbers are a
straightforward way to do that.

## Memtable and flush

Writing every row directly into its final sorted file would turn random writes
into random disk I/O. Instead, insert the row into a memtable. When it reaches
a size or time threshold, flush it as a new immutable segment.

```cpp
Status Database::Flush() {
  const auto snapshot = memtable_.Freeze();
  Segment segment = BuildSortedSegment(snapshot);

  const Path temporary = NewTemporarySegmentPath();
  RETURN_IF_ERROR(segment.Write(temporary));
  RETURN_IF_ERROR(FsyncFile(temporary));
  RETURN_IF_ERROR(RenameAtomically(temporary, segment.FinalPath()));
  RETURN_IF_ERROR(FsyncDirectory(data_dir_));

  RETURN_IF_ERROR(manifest_.PublishAtomically(segment.Metadata()));

  memtable_.Remove(snapshot);
  return wal_.DiscardThrough(snapshot.max_sequence());
}
```

The exact implementation differs by filesystem, but the ordering matters:

1. Write a complete new segment to a temporary path.
2. Sync it, then atomically rename it to its final path.
3. Atomically publish and sync a manifest that says the segment is live.
4. Only then discard the WAL prefix represented by that segment.

If the process fails before step 3, recovery ignores the unreferenced temporary
file and replays the WAL. If it fails after step 3, the segment is available
and its WAL records can be ignored. Never delete the log first.

## Recovery

Recovery begins from durable, immutable state and applies newer changes after
it:

```cpp
Status Database::Recover() {
  RETURN_IF_ERROR(manifest_.Load());
  RETURN_IF_ERROR(OpenReferencedSegments(manifest_));

  auto reader = wal_.Reader();
  while (auto next = reader.Next()) {
    const Record& record = *next;
    if (!ValidateChecksum(record)) {
      if (reader.AtPhysicalEnd()) {
        break;  // Ignore only a torn final append.
      }
      return Status::Corruption("bad WAL checksum");
    }
    if (record.sequence > manifest_.flushed_sequence()) {
      RETURN_IF_ERROR(memtable_.Apply(record));
    }
  }

  return reader.status();
}
```

Do not silently skip a corrupt record in the middle of a WAL. A bad checksum
after valid records at the physical end can be a normal torn final append. A
bad checksum in the middle is data corruption and should stop recovery with a
visible error.

## Deletes and compaction

An immutable segment cannot remove a row in place. A delete is a new tombstone
record with a newer version. Reads consult newer state before older segments,
so the tombstone hides the old value.

Compaction merges several segments, keeps the newest version of each ID, and
eventually drops tombstones once no older segment can still contain the deleted
row. This is also when obsolete vector data and index entries can be reclaimed.

The result is a simple recovery model: a manifest identifies complete segments
and the WAL contains ordered changes after them. The work is mostly in getting
the order right.

For the approximate index built over immutable segments, see
[HNSW](/post/hnsw-for-vector-search/).
