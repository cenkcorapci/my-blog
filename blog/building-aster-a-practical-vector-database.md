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

### What the numbers look like

#### Aster 1.0.3: another 10-million-vector improvement

[Aster 1.0.3](https://github.com/cenkcorapci/Aster/releases/tag/v1.0.3)
converts the query to double once per segment instead of once per vector in
optimized ARM64 int8 L2 exact scans. The calculation still scans every vector
and accumulates distances in double precision.

On the same Apple M4 host and 10-million-vector workload described below,
optimized builds of 1.0.2 and 1.0.3 each ran 30 sequential top-10 searches
after reopen, including the first query:

| Metric | 1.0.2 | 1.0.3 |
| --- | ---: | ---: |
| Search p50 (ms) | 252.060 | 214.753 |
| Searches per second | 3.9 | 4.7 |
| Peak process RSS (MiB) | 5880.5 | 5574.1 |

That is **14.8% lower median latency** in this run. Repeating the comparison
in reverse order gave 270.200 ms versus 219.680 ms (18.7% lower), with
3.7 versus 4.5 searches per second. Both comparisons had zero reported
search errors. Bars are scaled separately for each metric; peak RSS includes
loading and caching and does not demonstrate a memory reduction.

These are quantized exhaustive scans with HNSW disabled, not ANN or HTTP
benchmarks. The host was shared and 30 queries per run are too few to
characterize tail latency or production capacity. The benchmark did not
measure recall. See the [benchmark notes](https://github.com/cenkcorapci/Aster/blob/v1.0.3/docs/ram-efficiency.md#103-prepared-query-for-packed-exact-scans)
for the settings and reproduction command.

#### Aster 1.0.2: 10-million-vector comparison

After profiling the exact scan, [Aster 1.0.2](https://github.com/cenkcorapci/Aster/releases/tag/v1.0.2)
added an ARM64 NEON kernel for packed int8 L2 scoring and bounded parallel
scanning for large, unfiltered searches across multiple exact int8 segments.
The scan still scores every vector; it is not an approximate index.

The following is a like-for-like comparison of optimized (`-c opt`) builds
on the same Apple M4 with 32 GiB RAM and macOS 26.6.2. Each run used 10 million
synthetic normalized 384-dimensional vectors, `int8_sq`, L2, cold mmap, WAL
sync disabled, and HNSW background building off. After reopening, the harness
ran 20 sequential top-10 searches, including the first query.

| Metric | 1.0.1 | 1.0.2 |
| --- | ---: | ---: |
| Search p50 (ms) | 2100.474 | 256.051 |
| Searches per second | 0.5 | 3.9 |
| Peak process RSS (MiB) | 5790.0 | 6073.7 |

The median was **8.2× lower** in this workload, with higher peak process RSS
and up to four scan workers trading CPU concurrency for latency. The bar
lengths are scaled separately for each metric. The shared machine was not
isolated, and 20 queries are too few for reliable tail-latency estimates. This
is a quantized exhaustive scan, not an ANN, HTTP, recall, or production
capacity benchmark. Filtered searches, HNSW, other encodings, and small scans
do not use the parallel fan-out. Different floating-point reduction order can
slightly change scores at ties.

The command and full conditions are in the
[benchmark notes](https://github.com/cenkcorapci/Aster/blob/v1.0.2/docs/ram-efficiency.md#large-exact-scan-benchmark-2026-11-20).

#### Historical fastbuild measurements by index size

These are measurements, not capacity promises. I ran the same single-node
durable stress harness at different collection sizes on an Apple M4 with
32 GiB RAM and macOS 26.6.2 on November 20, 2026. Each run used
synthetic normalized 384-dimensional vectors, L2 distance, `int8_sq`, cold
mmap, WAL sync disabled, and exact top-10 searches after reopening. HNSW
background building was **off**.
This was the existing Bazel `fastbuild` binary, not an optimized production
build, from Aster revision
[`1710c88`](https://github.com/cenkcorapci/Aster/tree/1710c88).
These historical charts are not the optimized 1.0.2 results above and should
not be used to calculate the speedup.
The search loop starts after reopen and includes its first query;
these are not separate cold-start measurements. The machine was not isolated
from other work.

The three smaller runs used 100 sequential queries each; the million-vector
runs used only 10 each and ran at lower process priority. Their latency and
throughput figures are rough observations, not stable tail-latency estimates.
The horizontal axis is logarithmic so small and large collections fit on the
same chart; the tables contain the exact plotted values.

#### Peak process RSS by index size

| Vectors | Peak RSS (MiB) |
| --- | ---: |
| 10,000 | 64.6 |
| 25,000 | 123.2 |
| 50,000 | 135.7 |
| 1,000,000 | 667.7 |
| 2,000,000 | 1178.9 |
| 5,000,000 | 2711.0 |
| 10,000,000 | 5581.0 |

This is the process high-water mark over loading and searching, not steady
state, anonymous RSS, or just the index. It includes the memtable, metadata,
and resident mapped pages. The harness reported 0 bytes of anonymous vector
heap for the first sealed store after reopen at each size; it does not check
all segments and is not a claim that the whole process uses no RAM.

#### Exact search latency by index size

| Vectors | p50 (ms) |
| --- | ---: |
| 10,000 | 7.618 |
| 25,000 | 19.129 |
| 50,000 | 38.151 |
| 1,000,000 | 889.748 |
| 2,000,000 | 1541.784 |
| 5,000,000 | 3934.726 |
| 10,000,000 | 7685.991 |

These medians describe sequential exact scans, not ANN or HTTP latency.
At 10,000, 25,000, and 50,000 vectors respectively, p95 was 10.529, 19.902,
and 38.823 ms; p99 was 20.573, 20.042, and 42.017 ms. Those are based on
100 queries per size. Ten queries at the larger sizes are too few to make a
useful p95 or p99 claim.

#### Exact search throughput by index size

| Vectors | Searches per second |
| --- | ---: |
| 10,000 | 121.1 |
| 25,000 | 52.1 |
| 50,000 | 26.0 |
| 1,000,000 | 1.1 |
| 2,000,000 | 0.6 |
| 5,000,000 | 0.3 |
| 10,000,000 | 0.1 |

Each size was a separate run; throughput is measured over the same search
loop as the latency chart and rounded to one decimal place. The 10-million
run took 544.38 seconds to load and flush and 25.497 seconds to reopen.
The harness reported no search or write errors, but did not measure recall.
Different datasets, indexing modes, hardware, and concurrency will produce
different results.

To reproduce, build the benchmark and run each size in a **disposable
directory**: the harness deletes the directory passed to `--root`. The larger
runs take substantial time and memory; the 10-million-vector run used over
4 GiB of disk.

```bash
bazel build //aster/bench:ram_stress_bench
bench_dir="$(mktemp -d)"
for n in 10000 25000 50000; do
  bazel-bin/aster/bench/ram_stress_bench --single-only --single-n "$n" \
    --dim 384 --encoding int8_sq --searches 100 --root "$bench_dir/$n"
done
for n in 1000000 2000000 5000000 10000000; do
  nice -n 10 bazel-bin/aster/bench/ram_stress_bench --single-only --single-n "$n" \
    --dim 384 --encoding int8_sq --searches 10 --root "$bench_dir/$n"
done
```

More on the harness, residency metrics, and limitations is in the
[RAM efficiency notes](https://github.com/cenkcorapci/Aster/blob/main/docs/ram-efficiency.md).

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
