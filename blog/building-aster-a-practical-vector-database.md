---
title: Building Aster: Segments, Search, and RAM That Does Not Track the Corpus
date: 2026-10-30
tags: vector databases, databases, machine learning
---

# Building Aster: Segments, Search, and RAM That Does Not Track the Corpus

I am building [Aster](https://github.com/cenkcorapci/Aster) for applications
that need vector search but do not need a data platform. The useful case is
ordinary: documents, product titles, support tickets, or images become
vectors, and the application needs the nearest ones for a query. If that
collection fits on one machine and does not need multi-node replication, a
cluster is usually extra work rather than an advantage.

A local HTTP API is easy to show. The design bets that matter are elsewhere:
how writes become immutable segments, how search stays correct while an
approximate index is still building, and how sealed vectors avoid forcing
anonymous RAM to grow with the corpus.

Aster is intended to run next to the application, as a local HTTP service or
in-process. The same tree also builds constrained profiles, from a tiny
in-memory exact index to edge and server builds. The small profile is useful
for an embedded local index; it is not a claim that an Arduino should host a
large semantic-search corpus.

## Architecture: immutable segments instead of one mutable graph

With a data directory, Aster uses an LSM-style storage path:

1. It records the write in a write-ahead log (WAL) and adds it to an in-memory
   memtable. Both stay float32.
2. A flush writes the memtable as an immutable SSTable segment (`.ast`).
3. Compaction merges segments and eventually removes values superseded by
   updates or deletes.

The reason for this shape is not novelty. It makes the crash story
manageable: on startup, Aster reads its segment manifest and replays WAL
entries that have not yet been flushed. The
[write-ahead log primer](/post/why-a-vector-database-needs-a-write-ahead-log/)
covers recovery in more detail.

Immutability also decides how indexing works. Aster does not maintain one
live HNSW graph that mutates on every upsert and delete. Each sealed segment
gets its own graph. Deletes are tombstones; superseded rows disappear when
compaction rebuilds segments and their indexes. That is closer to how Lucene
handles segment-local structures than to a single always-mutable ANN heap.

A local vector index is useful. A local vector index that can lose recent
writes after a restart is much less useful. Durability of the write path is
still the part of the project that matters most to me.

## Search: exact until READY, then per-segment HNSW

Exact search compares a query with every vector. It is the reference answer,
a good fit for small collections, and Aster's baseline: every approximate
index is measured against it.

For larger collections, each immutable segment can carry an HNSW graph.
While that graph is still pending or building, the segment remains searchable
with an exact scan. Once the graph is READY, search uses HNSW on that
segment. There is no window where a flushed row is invisible because the
approximate index is not finished yet.

At query time Aster searches the memtable and each segment, merges the
candidates, reconciles last-write-wins versions, and can recompute final
scores from the reconciled full-precision rows. Approximate search is
candidate generation; the merge path still has a precise notion of which row
won.

HNSW trades a small amount of recall for fewer distance calculations. The
[HNSW primer](/post/hnsw-for-vector-search/) explains the graph and its
`ef_search` trade-off. The segmented HNSW implementation is in the tree, and
there is a recall CI floor (recall@10 ≥ 0.95 at `ef_search=128` versus
exact). The project's own recall and latency gate is still pending
verification. That distinction matters: the single-node durable storage path
is useful today; treat published ANN latency numbers as unfinished until that
gate lands.

## RAM: encoding and residency are different knobs

A 512-dimensional float32 vector uses 2 KiB. That sounds small until there
are 200,000 of them: the vector payload alone is about 410 MB before
metadata and indexing. Aster's docs use that 200k × 512 workload as a sizing
example:

| Encoding | Bytes per vector | ≈ 200k × 512 payload |
| --- | --- | ---: |
| float32 | `4D` | ≈ 410 MB |
| float16 | `2D` | ≈ 205 MB |
| int8 scalar quantization | `D + 4` | ≈ 103 MB |
| binary (RaBitQ-inspired) | `⌈D/8⌉ + 4` | ≈ 14 MB |

Those encodings apply when a segment is sealed or compacted. The WAL and
memtable stay float32. int8 is per-vector asymmetric scalar quantization with
no training set. The binary path is a RaBitQ-inspired 1-bit-per-dimension
packing with a scale factor, not a full multi-bit RaBitQ or Faiss port.

Compression alone is not enough. If the sealed packed payload still lives on
the heap after flush or reopen, corpus size still tracks anonymous RSS. Aster
treats that as a second, independent choice: *residency*.

- **cold** (default): mmap the sealed vector block; demand-paged
- **cached**: mmap and prefault when latency matters more than cold RSS
- **pinned**: keep a heap copy of the packed payload

Encoding chooses how many bytes a vector occupies. Residency chooses whether
those bytes must stay in anonymous RAM. Upper HNSW layers and entry points
are small navigation state and can stay pinned; the bulk vector block is the
plane that should be reclaimable. READY HNSW search scores from the packed or
mapped store rather than keeping a second float32 copy of the corpus beside
the segment.

On a documented stress run (Apple M4, 50k × 384, `int8_sq`, cold mmap, HNSW
background build off), sealed anonymous vector heap was 0 B after reopen and
process RSS peaked around 136 MB. That run is an exact-scan floor for
correctness and residency, not an HNSW latency claim.

The practical choice depends on the collection. A cold archive with a fixed
memory budget may accept int8 or binary. A small, high-value collection may
keep float16 or float32 and still use cold residency so the process does not
need to pin the whole payload.

## What using it looks like

The current single-node HTTP workflow is short:

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

## What Aster is not yet

The intended destination is an eventually consistent, peer-to-peer database
with tunable replication. Parts of that design exist as in-process
coordinator and gossip libraries, and as formal specs. They are not the
current product boundary: there is no production multi-node HTTP or RPC
cluster to operate today.

Use Aster when one durable node, local deployment, or an embedded index is
the problem. Do not select it for production replication, multi-node
failover, or published client packages; those areas are still in development.
Keeping that distinction explicit is more useful than calling an unfinished
feature a platform.

The code, API examples, encoding notes, and implementation status are in the
[Aster repository](https://github.com/cenkcorapci/Aster).
