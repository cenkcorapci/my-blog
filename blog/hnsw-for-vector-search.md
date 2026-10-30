---
title: HNSW
date: 2026-10-29
tags: vector databases, hnsw, machine learning
---

# HNSW

The simplest nearest-neighbour search compares a query with every vector and
returns the closest ones. It is exact. It also gets more expensive for every
vector added to the collection.

HNSW, short for *Hierarchical Navigable Small World*, is an approximate way to
search. It uses a graph instead of scanning the whole collection.

## The graph

Each vector is a node. Nodes are connected to nearby nodes. The bottom layer
contains every vector. The layers above it contain fewer vectors and act as
shortcuts across the graph.

A query starts at the top, follows the neighbour that gets it closer to the
query, and then moves down one layer. At the bottom layer it keeps a wider set
of candidates and returns the best ones.

This is a simplified version of the search:

```cpp
std::vector<Hit> Search(
    const Vector& query,
    std::size_t k,
    std::size_t ef_search) {
  NodeId current = entry_point;

  for (int layer = top_layer; layer > 0; --layer) {
    current = GreedySearch(query, current, layer);
  }

  auto candidates = SearchLayer(query, {current}, 0, ef_search);
  return TopK(candidates, k);
}
```

`GreedySearch` makes long jumps in sparse layers. `SearchLayer` examines more
neighbours in the dense bottom layer. The code hides many details, but this is
the useful mental model: get close quickly, then spend the search budget where
it matters.

## The trade-off

HNSW can miss a true nearest neighbour. In exchange, it usually calculates far
fewer distances than an exhaustive scan.

Two parameters are worth knowing:

- `M` is the maximum number of neighbours kept by a node. A larger value makes
  the graph larger and can improve its connectivity.
- `ef_search` is the number of candidates considered at search time. A larger
  value generally improves recall and increases CPU cost.

For example, a caller might choose a larger search budget for a matching
workflow where missing a candidate is costly:

```cpp
SearchRequest request;
request.vector = query;
request.top_k = 10;
request.ef_search = 128;

auto hits = index.Search(request);
```

There is no best value for every workload. The meaningful question is whether
the results are good enough at the latency and CPU cost the application can
afford.

## Segments

A single mutable HNSW graph is awkward when writes are frequent. Inserting a
vector changes graph links. Deleting a vector leaves a hole. Rebuilding the
whole graph after every change would be wasteful.

A common alternative is to build one graph for each immutable storage segment.
New writes stay in a small mutable buffer. Searches collect candidates from
each segment and merge them. Compaction merges old segments and builds a new
graph for the result.

This makes writes and search easier to reason about, but it does not remove
the work. Too many segments mean too many graphs per query, so compaction is
also part of search performance.

For the storage path behind immutable segments, see
[Write-ahead log](/post/why-a-vector-database-needs-a-write-ahead-log/).
