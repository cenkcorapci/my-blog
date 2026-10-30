---
title: Building Aster: A Vector Database for the Rest of Us
date: 2026-10-30
tags: vector databases, databases, c++, machine learning
---

# Building Aster: A Vector Database for the Rest of Us

Most applications that use embeddings eventually need to answer a simple
question: which stored item is most similar to this one? The input can be a
product title, a support request, an image, or a paragraph from a document.
Once it is represented as a vector, the question is the same.

The usual answer is to reach for a distributed system with a long configuration
file, several containers, and resource requirements that make sense for a large
team. That is a good answer for some problems. But most teams do not need that
level of machinery. They need a small, reliable component that can store
vectors, survive a restart, and be simple to install and operate.

That is why I am building [Aster](https://github.com/cenkcorapci/Aster), a
CPU-first vector database written in C++20. Its target is the part of the
market where simplicity and low resource usage matter more than a feature
checklist: embedded applications, local tools, edge devices, and small
services.

## A database should not require a platform team

The first useful workflow should be boring. Install Aster, start it with a
data directory, and use an HTTP API:

```bash
curl -fsSL https://raw.githubusercontent.com/cenkcorapci/Aster/main/scripts/install.sh | sh

aster serve --data-dir "$HOME/.local/share/aster" --port 8080
```

Creating a collection, writing a document, and searching it is intentionally
just a few requests:

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

This is not meant to replace every large-scale vector platform. It is meant to
remove the point at which a team is forced to adopt one before its problem
actually demands it.

## Small does not mean disposable

Keeping every vector in memory and comparing a query with all of them is a
perfectly reasonable starting point. But it does not help with the things that
make a database dependable: surviving a restart, removing old data, observing
the system, and recovering cleanly from an interrupted write.

Aster combines a vector index with a familiar durable storage lifecycle:

1. A write enters a memtable and, when a data directory is configured, is
   persisted to a write-ahead log.
2. A flush creates an immutable SSTable segment and its vector index.
3. Compaction merges segments and removes obsolete entries and tombstones.

This is the kind of unglamorous work that matters. The aim is to bring the
operational robustness associated with systems such as Cassandra to a much
smaller deployment model: durable logs, immutable files, explicit compaction,
and recovery instead of an in-memory index that has to be reconstructed after
every restart.

The longer-term distributed direction is eventual consistency with the same
resource-conscious mindset. Aster should be able to make a pragmatic
availability and consistency trade-off when a workload needs more than one
node, without making clustering the price of getting started. Today, the
stable release focuses on durable single-node storage; replication is a planned
step rather than a feature to pretend already exists.

## Run it where the application runs

Sometimes HTTP is the right boundary. Sometimes it is an unnecessary process
and an unnecessary deployment. Aster also exposes a stable embedded C++ API,
so an application can own the database in-process:

```cpp
#include "aster/db/db.h"

aster::Db::Options options;
options.dimension = 384;
options.metric = aster::Metric::kCosine;
options.data_dir = "/var/lib/my-app/aster";

auto opened = aster::Db::Open(options);
if (!opened.ok()) {
  return ReportError(opened.status());
}

std::unique_ptr<aster::Db> db = std::move(opened.value());
```

The same codebase has a tiny in-memory profile for constrained targets, smaller
edge defaults for single-board computers, and a full server profile. The goal
is not to claim that an Arduino should host a huge semantic-search corpus. It
is to make the core database small enough to run there when an application only
needs a small local index.

## Search is a trade-off, not a magic trick

Exact search compares a query vector with every stored vector. It provides the
reference answer, but becomes expensive as collections grow. For larger
collections, Aster uses per-segment HNSW indexes. HNSW navigates a graph of
likely neighbours rather than comparing every vector, trading a small amount
of recall for much lower search cost.

There is no universally correct setting. A product-matching workflow may
prefer higher recall because missing a viable candidate is costly. A
recommendation feature may prioritize latency. Aster makes that trade-off
visible through request-level controls such as `top_k` and `ef_search`, rather
than hiding it behind a fixed deployment decision.

```json
{
  "vector": [0.1, 0.2, 0.3],
  "top_k": 10,
  "ef_search": 128
}
```

## Memory is also a product decision

Vectors are deceptively large. Two hundred thousand 512-dimensional float32
vectors need roughly 410 MiB just for their values, before considering the
index, metadata, or the rest of the application. This is manageable on a
server, but it changes the conversation for an edge device.

Aster keeps the write path as float32, then offers compact encodings for
sealed segments:

| Encoding | Approximate payload at 200k × 512 |
| --- | ---: |
| float32 | 409.6 MiB |
| float16 | 204.8 MiB |
| int8 scalar quantization | 103.2 MiB |
| binary RaBitQ-style encoding | 13.6 MiB |

These are not free savings. Lower-precision representations affect recall and
may require a higher search effort. The important part is that the trade-off is
available to the application:

```cpp
aster::Db::Options options;
options.dimension = 512;
options.vector_encoding = aster::VectorEncoding::kInt8Sq;
```

A cold collection with a strict memory budget has different needs from a small,
high-value collection where fidelity is more important. Aster should let a team
choose that boundary without changing databases.

## What comes next

Building a vector database is less about inventing a new distance function and
more about making many ordinary decisions fit together: storage formats,
recovery, compaction, index quality, API behaviour, and memory use. Aster is
at its first stable release with durable single-node storage, HTTP and Thrift
servers, HNSW search, and an embedded C++ API.

There is still a lot to do, especially client package publishing and
replication. Keeping those boundaries visible is more useful than pretending a
database is complete because it can return a nearest neighbour.

You can find the source code, examples, and design documentation in the
[Aster repository](https://github.com/cenkcorapci/Aster).
