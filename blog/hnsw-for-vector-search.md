---
title: HNSW
date: 2026-10-29
tags: vector databases, hnsw, machine learning
---

# HNSW

HNSW is an approximate nearest-neighbor index. It answers: given a query
vector, which stored vectors are probably closest to it?

The exact answer is easy to define. Calculate a distance from the query to
every vector, sort, and keep the first `k` values. That is also the right
baseline for testing an approximate index. It becomes expensive when the
collection is large because every query touches every vector.

HNSW, short for *Hierarchical Navigable Small World*, avoids most of those
distance calculations by arranging vectors in a graph.

## What is stored

Each vector is a node. A node has one neighbour list for each layer it belongs
to. Layer zero contains every node. Higher layers contain progressively fewer
nodes, so they act as shortcuts.

```cpp
using NodeId = uint32_t;

struct Node {
  std::vector<float> vector;
  std::vector<std::vector<NodeId>> neighbors;
  // neighbors[0] is the dense bottom layer.
};

struct Hnsw {
  std::vector<Node> nodes;
  NodeId entry_point;
  int top_layer = 0;
  std::size_t M = 16;
};
```

The graph stores node IDs, not copied vectors. Keep vectors in a contiguous
store if possible; distance calculation is usually the hot path.

The metric must be fixed for an index. With cosine similarity, normalize
vectors before inserting and querying, then maximize the dot product. With
Euclidean distance, minimize squared L2 distance; there is no need to take the
square root just to compare results.

```cpp
float SquaredL2(std::span<const float> a, std::span<const float> b) {
  float sum = 0.0F;
  for (std::size_t i = 0; i < a.size(); ++i) {
    const float d = a[i] - b[i];
    sum += d * d;
  }
  return sum;
}
```

## Building the layers

Each new node gets a random highest layer. Most nodes only exist in layer zero.
Few exist in upper layers. The usual distribution is exponential:

```cpp
int RandomLevel(std::mt19937& rng, float level_multiplier) {
  std::uniform_real_distribution<float> uniform(0.0F, 1.0F);
  const float value = std::max(uniform(rng),
                               std::numeric_limits<float>::min());
  return static_cast<int>(-std::log(value) * level_multiplier);
}
```

For HNSW, `level_multiplier` is commonly `1 / log(M)`. This is not arbitrary:
it keeps upper layers sparse enough to be useful as shortcuts without making
the graph too large.

When inserting a vector:

1. Draw its highest layer.
2. Start from the current entry point at the top layer.
3. Greedily walk each layer down to the node closest to the new vector.
4. At every layer the new node belongs to, run a wider local search.
5. Connect it to a bounded number of selected neighbours.
6. If it has the highest layer seen so far, make it the new entry point.

The broad shape is:

```cpp
void Insert(Hnsw& index, std::vector<float> vector, std::mt19937& rng) {
  const int level = RandomLevel(rng, 1.0F / std::log(index.M));
  const NodeId id = AddNode(index, std::move(vector), level);
  NodeId current = index.entry_point;

  for (int layer = index.top_layer; layer > level; --layer) {
    current = GreedySearch(index, index.nodes[id].vector, current, layer);
  }

  for (int layer = std::min(level, index.top_layer); layer >= 0; --layer) {
    auto candidates = SearchLayer(index, index.nodes[id].vector, {current},
                                  layer, /*ef=*/200);
    auto selected = SelectNeighbors(index, id, candidates, index.M);
    ConnectBidirectionally(index, id, selected, layer, index.M);
    if (!selected.empty()) {
      current = selected.front();
    }
  }

  if (level > index.top_layer) {
    index.entry_point = id;
    index.top_layer = level;
  }
}
```

The example omits empty-index handling and error paths, but those belong in a
real implementation. In particular, initialize the first node as the entry
point before searching.

`SelectNeighbors` should not simply keep the `M` closest candidates. A common
heuristic rejects a candidate if it is closer to an already selected neighbor
than it is to the new node. That preserves links in different directions and
avoids making all edges point into one dense cluster. Keep degrees bounded for
old neighbours too; otherwise the graph grows without limit.

`ef_construction` is the candidate count used while building. Higher values
usually improve graph quality and make insertion slower. In a real API,
validate that it and `ef_search` are nonzero, and that `ef_search >= k`.

## Searching

Search has two phases. In upper layers, greedily follow the one neighbour that
improves the distance. At layer zero, use a best-first search with two heaps:

- a min-heap of candidates still worth visiting;
- a max-heap of the best `ef_search` candidates found so far.

```cpp
std::vector<NodeId> SearchLayer(
    const Hnsw& index,
    std::span<const float> query,
    std::vector<NodeId> entry_points,
    int layer,
    std::size_t ef_search) {
  MinHeap candidates;  // smallest distance first
  MaxHeap best;        // largest distance first
  std::unordered_set<NodeId> visited;

  for (NodeId id : entry_points) {
    const float distance = SquaredL2(query, index.nodes[id].vector);
    candidates.push({distance, id});
    best.push({distance, id});
    visited.insert(id);
  }

  while (!candidates.empty()) {
    const auto [distance, id] = candidates.pop();
    if (best.size() == ef_search && distance > best.top().distance) {
      break;
    }

    for (NodeId next : index.nodes[id].neighbors[layer]) {
      if (!visited.insert(next).second) {
        continue;
      }

      const float next_distance =
          SquaredL2(query, index.nodes[next].vector);
      if (best.size() < ef_search ||
          next_distance < best.top().distance) {
        candidates.push({next_distance, next});
        best.push({next_distance, next});
        if (best.size() > ef_search) {
          best.pop();
        }
      }
    }
  }

  return best.sorted_by_distance();
}
```

The stopping rule is important. Once the closest unvisited candidate is worse
than the worst result in a full `best` heap, exploring farther candidates
cannot improve that local result set.

At query time, `ef_search` must be at least `k`. Raising it normally increases
recall and CPU use:

```cpp
SearchRequest request;
request.vector = query;
request.top_k = 10;
request.ef_search = 128;

auto hits = index.Search(request);
```

There is no globally correct value. Measure recall and latency against the
actual data distribution.

## Testing it

Keep an exact scan available for tests. For each query, calculate the exact
top `k`, calculate the HNSW top `k`, and compare the IDs:

```text
recall@k = |approximate_results ∩ exact_results| / k
```

Test random vectors, but also test clustered vectors, duplicate vectors,
updates, deletes, empty indexes, and dimensions that do not match. Random
uniform data is usually too kind to an approximate index.

For a database, do not mutate a shared graph on every write unless the locking
and crash story are clear. A common design is one immutable graph per storage
segment: new writes go to a small mutable buffer, a flush builds a graph, and
compaction rebuilds graphs for merged segments. That trades one huge mutable
problem for several smaller, immutable ones.

For the recovery path behind those segments, see
[Write-ahead log](/post/why-a-vector-database-needs-a-write-ahead-log/).
