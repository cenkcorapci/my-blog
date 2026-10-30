---
title: Why a Vector Database Needs a Write-Ahead Log
date: 2026-10-29
tags: databases, storage, vector databases
---

# Why a Vector Database Needs a Write-Ahead Log

An in-memory vector index is easy to understand: add vectors, search them,
and keep the process running. The awkward question is what happens when the
process stops halfway through a write.

If the only copy of a recent write was in memory, it is gone. If a data file
was being changed in place, it may be partly written. A database needs a way
to recover without guessing which state is valid.

That is what a write-ahead log, or WAL, is for.

## Append first

A WAL is an append-only record of changes. Before the database treats a write
as durable, it records enough information to replay that write later. Appending
one new record is simpler to recover than altering several existing files.

In the durable path used by [Aster](/post/building-aster-a-practical-vector-database/),
a write is recorded in the WAL and placed in a memtable, an in-memory write
buffer. Later, a flush turns the memtable into an immutable SSTable segment on
disk.

On restart, Aster loads the segment manifest and its existing SSTables, then
replays the WAL. Entries already represented in a flushed segment do not need
to be reconstructed; entries that were still only in memory do.

## Why immutable files help

The flush does not update an existing segment. It creates a new immutable one.
Updates and deletes are newer records; a delete is represented by a tombstone.
Compaction later merges segments, retains the latest visible value, and drops
tombstones once it is safe to do so.

This creates more files in the short term, but gives the system a simple
recovery story:

1. A WAL contains recent writes.
2. A manifest identifies complete, immutable segments.
3. Recovery opens the segments and replays the remaining log.

The database has to decide when to synchronize its WAL to disk, so durability
is still a policy choice. But the important property is that a crash does not
leave the database trying to interpret a half-mutated search index.

## What this has to do with vector search

Vectors do not make persistence less important. A search index can often be
rebuilt from stored vectors, but rebuilds take time and may be difficult to
coordinate with incoming writes. Keeping an ordered log and immutable vector
segments makes recovery a normal storage operation rather than a special
emergency path.

For the approximate-search side of the design, see
[HNSW for Vector Search, Briefly](/post/hnsw-for-vector-search/).
