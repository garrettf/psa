# psa
A networked pubsub library for Go. Making use of lock-free concurrent data structures, with our [lockless linked list component](https://github.com/garrettf/slll).

Currently a work in progress.

## Preliminary benchmarking
AKA reasons to not benchmark on Darwin.

On a Macbook Pro (Ivy Bridge i7-3615QM) with 4 physical cores, 8 logical units:
![screen shot 2015-06-06 at 5 06 33 am](https://cloud.githubusercontent.com/assets/480618/8019611/64425776-0c0c-11e5-8855-653c29cf7318.png)

And a VM on the same machine:
![screen shot 2015-06-06 at 5 07 06 am](https://cloud.githubusercontent.com/assets/480618/8019619/842a652e-0c0c-11e5-8ac2-9e3f10237dc7.png)

## Tradeoffs

### Connection Handling
Since goroutines have a small memory footprint and the Go runtime uses an
_O(1)_ scheduler, [the Go community
suggests](https://groups.google.com/forum/#!topic/golang-nuts/TSf14CJyA2s) to
use one goroutine per network connection. This implementation works fine. CPU
profiling suggests that very little time is spent in the scheduler. 

For many (>1k) connections, it may be useful to spawn a fixed number of worker
goroutines in order to limit potential stress on the scheduler.

### Synchronization
The problem of dumping the same message to thousands of connections essentially
boils down to synchronization. The most idiomatic, Go-like solution involves
assigning each connection a channel. The publisher must iterate over all of the
channels, placing a message into every connection's buffer.

There are many problems with this implementation, including but not limited to:

1. To guarantee success, the publish function must block on an _O(num
   connections)_ operation.
2. Slow connections have the potential to fill their channel buffer, thus
   causing the publish operation to block on I/O. This could be ameliorated
   with a variation of [stacked
   channels](http://gowithconfidence.tumblr.com/post/31426832143/stacked-channels),
   but this ultimately leads to hemorrhaging memory.
3. Go's policy for concurrency is "do not communicate by sharing memory;
   instead, share memory by communicating." This is fairly sound advise in most
   cases, but when you intend to communicate the same message to a few thousand
   clients, the overhead of duplicating that message and signaling a few
   thousand semaphores becomes overwhelming.

Instead, I got creative. Introducing: lock-free data structures. My
implementation uses a lock-free singly-linked list and a condition variable.
I use the list as a master log. Each call to the publish function appends to
the log and broadcasts on the condition variable. Each subscriber goroutine
sends every list entry until it reaches the end of the list. Then the
subscriber waits on the condition variable again. 

There is a small amount of overhead due to the use of a mutex with the
condition variable. In this case, there's no need for a mutex. We'd be better
off with a traditional semaphore, but Go doesn't provide any other primitive
capable of safely repeatedly broadcasting. As a compromise, I immediately
release the mutex whenever it is given. CPU profiling suggests that this does
not incur a significant penalty.

The lock-free implementation heavily outperformed a regular linked list
protected by an RWMutex, by nearly an order of magnitude.

## TODO
* Garbage collection of the log
* Handling the last-message-received subscriber parameter
