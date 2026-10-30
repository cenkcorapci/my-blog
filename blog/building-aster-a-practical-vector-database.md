---
title: Building Aster: A Practical Vector Database
date: 2026-10-30
tags: vector databases, databases, c++, machine learning
---

# Building Aster: A Practical Vector Database

Most applications that use embeddings eventually need to answer a simple
question: which of these thousands, millions, or perhaps billions of items is
most similar to this one? The input can be a product title, a support request,
an image, or a paragraph from a document. Once it is represented as a vector,
the question is the same.

The simple answer is to keep every vector in memory and compare the query with
all of them. It is also a good answer for small datasets. But it becomes
expensive quickly, and it does not help much with the parts that make a system
useful in production: surviving a restart, deleting old data, observing what
is happening, and running on something smaller than a large server.

This is the problem I wanted to work on with [Aster](https://github.com/cenkcorapci/Aster), a CPU-first vector database written in C++20.

## What “practical” means here

Aster can run as an embedded library inside a C++ application, as a local HTTP
service, or as a server. It has a durable write-ahead log and SSTables, while
also keeping a vector index for similarity search. In other words, the vector
index is not treated as a temporary cache that has to be rebuilt from an
external database after every restart.

The write path follows a familiar pattern:

1. A write first goes into a memtable and, when a data directory is configured,
   is persisted to the WAL.
2. A flush creates an immutable SSTable segment together with its vector index.
3. Compaction merges segments and removes obsolete entries and tombstones.

This is not a new storage idea. LSM trees have been used for a long time
because they give a useful balance between write throughput and durable,
ordered files. The interesting part is applying the same lifecycle to vectors
and their indexes. A search may need to look at multiple segments, but the
database can still keep writes, flushes, and compactions predictable.

## Exact search is useful, approximate search is necessary

For a query vector \(q\), exact search calculates its distance to every stored
vector \(x\). Depending on the collection, Aster can use cosine distance or
other supported metrics. This gives us a clean reference point: exact results
are what we want, but they are not always what we can afford.

For larger collections, Aster uses per-segment HNSW indexes. HNSW is an
approximate nearest-neighbour algorithm. Instead of comparing a query with
every vector, it traverses a graph and investigates the most promising
neighbourhoods. The important word is *approximate*: we gain speed by accepting
that a result may not be the mathematically closest vector.

There is no universally correct setting for this trade-off. A product matching
workflow may prefer higher recall because missing a viable candidate is costly.
A recommendation feature may prioritize latency. That is why search parameters
such as `top_k` and `ef_search` are request-level controls in Aster rather than
fixed deployment choices.

## Memory is part of the design

Vectors are deceptively large. Two hundred thousand 512-dimensional float32
vectors need roughly 410 MiB just for their values, before considering the
index, metadata, or the rest of the application. This is manageable on a
server, but it changes the conversation for an edge device or an embedded
deployment.

Aster keeps the write path as float32, then offers compact encodings for
sealed segments:

| Encoding | Approximate payload at 200k × 512 |
| --- | ---: |
| float32 | 409.6 MiB |
| float16 | 204.8 MiB |
| int8 scalar quantization | 103.2 MiB |
| binary RaBitQ-style encoding | 13.6 MiB |

These are not free savings. Lower-precision representations affect recall and
may require a higher search effort. The point is to make the trade-off
explicit. A cold collection with a strict memory budget has different needs
than a small, high-value collection where fidelity is more important.

## A small interface, but not a toy

The first useful workflow should not require operating a cluster. Aster can
start a local HTTP server, create a collection with a chosen dimension and
metric, accept vectors, and search them. It also exposes collection statistics,
health information, and Prometheus metrics because a database that cannot be
observed is difficult to trust.

For applications that do not need a separate service, the stable
`aster::Db` interface can be linked directly into a C++ application. This
matters for local tools and deployments where running another process would be
unnecessary overhead.

The project also has profiles for different targets: a tiny in-memory build
with exact search, smaller edge defaults, and a full server profile. The goal
is not to claim that one binary is optimal everywhere. It is to keep the core
model useful when the hardware and operational constraints are different.

## What comes next

Building a vector database is less about inventing a new distance function and
more about making many ordinary decisions fit together: storage formats,
recovery, compaction, index quality, API behaviour, and memory use. Aster is
now at its first stable release, with durable single-node storage, HTTP and
Thrift servers, HNSW search, and an embedded C++ API.

There is still a lot to do. Client package publishing, replication, and
production-ready object-store authentication are deliberately separate pieces
of work. Keeping these boundaries visible is more useful than pretending a
database is complete because it can return a nearest neighbour.

You can find the source code, examples, and design documentation in the
[Aster repository](https://github.com/cenkcorapci/Aster).
