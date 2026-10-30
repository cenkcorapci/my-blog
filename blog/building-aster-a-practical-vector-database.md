---
title: Building Aster: A Vector Database for the Rest of Us
date: 2026-10-30
tags: vector databases, databases, machine learning
---

# Building Aster: A Vector Database for the Rest of Us

There is a familiar pattern in software. A team has a small problem, finds a
tool built for a much bigger problem, and then spends the next year operating
the tool.

Vector search seems especially prone to this. Suppose you have a few hundred
thousand product titles, support tickets, or documents. You embed them, then
you want to find the nearest ones for each new item. It is tempting to begin
with a distributed vector database, a Kubernetes cluster, and a long list of
things to tune. Sometimes that is the right answer. Usually it is not.

I am building [Aster](https://github.com/cenkcorapci/Aster) for the other
cases: applications that need vector search, but do not need a data platform.
It is a CPU-first vector database that can run as a small HTTP service or
inside an application. The intended experience is closer to adding a local
database than adopting a new piece of infrastructure.

The basic setup should look like this:

```bash
curl -fsSL https://raw.githubusercontent.com/cenkcorapci/Aster/main/scripts/install.sh | sh

aster serve --data-dir "$HOME/.local/share/aster" --port 8080
```

Then create a collection, write a vector, and search it:

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

That is not a clever interface. It is deliberately not clever.

## The thing I do not want to give up

Simple does not have to mean disposable. A process that holds an HNSW index in
memory can be very fast, but it is not yet a database. It needs to survive a
restart. It needs a reasonable answer to an interrupted write. It needs to
remove deleted values eventually, rather than collecting them forever.

Aster uses a write-ahead log, memtables, immutable SSTable segments, and
compaction. The sequence is not particularly original:

1. A write is added to a memtable and persisted to a WAL when a data directory
   is configured.
2. A flush turns that mutable state into an immutable segment with its vector
   index.
3. Compaction merges segments and removes values that are no longer current.

This is the part of databases that I find more interesting than the search
algorithm. HNSW is useful, but a database mostly earns its place on the system
diagram by behaving sensibly at inconvenient times.

The resilience target is informed by systems such as Cassandra: append durable
data first, use immutable files, and make recovery and cleanup explicit. I do
not mean that a small local vector database should be a drop-in replacement for
Cassandra. It should borrow the habits that make Cassandra boring in the good
sense.

## Where the small part matters

Most vector databases are comfortable assuming a server with plenty of RAM.
That is fine until the vector store is part of an application that runs on a
single-board computer, a local tool, or a constrained edge machine.

The same Aster codebase has a tiny in-memory profile, smaller edge defaults,
and a full server profile. The tiny profile can run on an Arduino-class target.
This does not mean an Arduino is now a semantic-search cluster. It means a
small local index can be treated as a normal part of an embedded application
instead of an excuse to add a separate service.

HTTP is useful when there are multiple consumers. In-process storage is useful
when there are not. Neither should force the other.

## Vectors take more space than they look like

A 512-dimensional float32 vector is only 2 KiB. That sounds small. Two hundred
thousand of them are roughly 410 MiB before the index, metadata, or the rest
of the application. This is the point at which a simple choice starts having
real consequences.

Aster writes float32 vectors to its active write path, then supports compact
representations in sealed segments:

| Encoding | Approximate payload at 200k × 512 |
| --- | ---: |
| float32 | 409.6 MiB |
| float16 | 204.8 MiB |
| int8 scalar quantization | 103.2 MiB |
| binary RaBitQ-style encoding | 13.6 MiB |

The trade-off is ordinary: less memory means less precision. There is no
encoding that makes that go away. But it is useful to be able to make the
choice at the collection boundary rather than changing the whole system.

Search has the same shape. Exact search checks every vector and is the clean
reference answer. HNSW checks a promising part of the collection and is much
cheaper at scale, but approximate. Applications can decide how hard to search:

```json
{
  "vector": [0.1, 0.2, 0.3],
  "top_k": 10,
  "ef_search": 128
}
```

For product matching, missing a good candidate might be more expensive than a
few extra milliseconds. For a recommendation widget, the opposite might be
true. The database should expose that decision instead of pretending it does
not exist.

## One node first, more when it is needed

The long-term direction for Aster is eventual consistency across nodes without
turning a small deployment into a complicated one. That is a goal, not a claim
about the current release. Today, the stable surface is durable single-node
storage, HTTP and Thrift servers, HNSW search, and an embedded API.

Replication and client package publishing are still ahead. I would rather keep
that boundary clear than call the project distributed because it contains a
ring implementation.

The project is small on purpose. The best outcome is not that every team uses
Aster. It is that a team with a modest vector-search problem can use it without
first acquiring the operational burden of a much larger system.

The code, API examples, and design documents are in the
[Aster repository](https://github.com/cenkcorapci/Aster).
