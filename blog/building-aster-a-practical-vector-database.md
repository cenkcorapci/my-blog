---
title: Building Aster: Local Vector Search Without a Platform
date: 2026-10-30
tags: vector databases, databases, machine learning
---

# Building Aster: Local Vector Search Without a Platform

I am building [Aster](https://github.com/cenkcorapci/Aster) for applications
that need vector search but do not need a data platform.

The useful case is fairly ordinary. An application has documents, product
titles, support tickets, or images. It turns them into vectors and needs to
find the nearest ones for a query. If the collection fits on one machine and
the application does not need multi-node replication, adding a cluster is
usually extra work rather than an advantage.

Aster is intended to run next to the application: as a local HTTP service or
in-process. It is also designed for constrained targets, from a small
in-memory profile to edge and server profiles. The small profile is useful for
an embedded local index; it is not a claim that an Arduino should host a large
semantic-search corpus.

## What using it looks like

The current single-node HTTP workflow is deliberately short:

```bash
curl -fsSL https://raw.githubusercontent.com/cenkcorapci/Aster/main/scripts/install.sh | sh

aster serve --data-dir "$HOME/.local/share/aster" --port 8080
```

Create a collection with a fixed vector dimension and metric, then add and
search documents:

```bash
curl -X PUT http://localhost:8080/v1/collections/docs \
  -H 'Content-Type: application/json' \
  -d '{"dimension":3,"metric":"cosine"}'

curl -X PUT http://localhost:8080/v1/collections/docs/docs/getting-started \
  -H 'Content-Type: application/json' \
  -d '{"vector":[0.1,0.2,0.3],"metadata":"hello"}'

curl -X POST http://localhost:8080/v1/collections/docs/search \
  -H 'Content-Type: application/json' \
  -d '{"vector":[0.1,0.2,0.3],"top_k":5}'
```

Cosine is generally the appropriate metric when the embedding model produces
normalized vectors. The collection dimension must match the embedding model's
output. Those two choices are made once for a collection, so they are worth
deciding before loading data.

## What happens to a write

With a data directory, Aster uses an LSM-style storage path:

1. It records the write in a write-ahead log (WAL) and adds it to an in-memory
   memtable.
2. A flush writes the memtable as an immutable SSTable segment.
3. Compaction merges segments and eventually removes values superseded by
   updates or deletes.

The reason for this shape is not novelty. It makes the crash story manageable:
on startup, Aster reads its segment manifest and replays WAL entries that have
not yet been flushed. The [write-ahead log primer](/post/why-a-vector-database-needs-a-write-ahead-log/)
explains the terms and the recovery path in more detail.

This is the part of the project that matters most to me. A local vector index
is useful. A local vector index that can lose recent writes after a restart is
much less useful.

## How search works

Exact search compares a query with every vector. It is the reference answer
and is a good fit for small collections. It is also Aster's baseline: every
approximate index is measured against exact search.

For larger collections, Aster's design uses one HNSW graph per immutable
segment. HNSW trades a small amount of recall for fewer distance calculations.
The [HNSW primer](/post/hnsw-for-vector-search/) explains the graph and its
`ef_search` trade-off without assuming that it is magic.

The HNSW implementation work is recorded as complete, but its recall and
latency gate is still pending verification. That is an important distinction:
the single-node storage path is useful today; HNSW should not be treated as a
finished performance promise until those measurements are published.

## Where resource use enters the picture

A 512-dimensional float32 vector uses 2 KiB. That sounds small until there
are 200,000 of them: the vector values alone then require about 410 MB,
before metadata and indexing.

Aster can compact sealed segments with lower-precision encodings. Float16,
int8 scalar quantization, and a binary RaBitQ-style encoding reduce RAM and
disk use at the cost of precision. The correct choice depends on the
collection: a cold collection with a fixed memory budget may accept lower
recall; a small, high-value collection may not.

## What Aster is not yet

The intended destination is an eventually consistent, peer-to-peer database
with tunable replication. That is not the current product boundary.

Today, use Aster when one durable node, local deployment, or an embedded index
is the problem. Do not select it for production replication, multi-node
failover, or published client packages; those areas are still in development.
Keeping that distinction explicit is more useful than calling an unfinished
feature a platform.

The code, API examples, and implementation status are in the
[Aster repository](https://github.com/cenkcorapci/Aster).
