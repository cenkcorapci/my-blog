---
title: HNSW for Vector Search, Briefly
date: 2026-10-29
tags: vector databases, hnsw, machine learning
---

# HNSW for Vector Search, Briefly

Nearest-neighbour search has an obvious implementation: compare a query vector
with every stored vector and return the closest results. It is exact, simple,
and eventually expensive.

If a collection has \(N\) vectors of dimension \(d\), that scan does work
proportional to \(N \times d\) for every query. It is often fine for small
collections. It becomes a poor default when queries must stay fast as \(N\)
grows.

HNSW, short for *Hierarchical Navigable Small World*, is one way to avoid
looking everywhere.

## The graph

HNSW stores vectors in a graph. Nearby vectors are connected. A query begins
at a sparse upper layer, where long jumps get it near the right part of the
space, then descends through denser layers until it searches the bottom layer
for the best candidates.

The result is approximate. HNSW may miss a true nearest neighbour, but it
usually needs far fewer distance calculations than an exhaustive scan.

Two parameters matter most:

- `M` limits how many neighbours a node keeps. It affects graph size and
  quality.
- `ef_search` controls how many candidates a query considers at the dense
  layer. Raising it usually improves recall and increases query cost.

There is no setting that is correct for every application. A product-matching
workflow may spend more CPU to avoid missing candidates. A recommendation
endpoint may accept lower recall for a tighter latency budget.

## Why one graph per segment

A single mutable HNSW graph is inconvenient for a write-heavy database. Adding
a vector changes graph links; deleting one leaves holes; recovery after an
interrupted graph update needs careful handling.

The design used by [Aster](/post/building-aster-a-practical-vector-database/)
is different: a flushed, immutable storage segment owns its own graph. New
writes are handled separately. A search gathers candidates from each segment,
reconciles newer values and deletes, then returns the best remaining results.
Compaction merges old segments and builds a new graph for the merged data.

This keeps graph construction away from the write path and makes immutable
storage do some useful work. It also means the number of segments matters:
too many segments make one query search too many graphs, so compaction is part
of search performance.

## The status in Aster

Exact search is Aster's correctness baseline. Its segmented-HNSW
implementation is still awaiting its recall and latency verification gate.
That is the right time to be precise about what HNSW provides: not a guarantee
of the exact top results, but a measurable recall/latency trade-off.

For the surrounding storage and recovery model, see
[Why a Vector Database Needs a Write-Ahead Log](/post/why-a-vector-database-needs-a-write-ahead-log/).
