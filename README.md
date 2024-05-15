# CNC-MASM

**M**issing **A**ssets and **S**ervices in **M**anatee is a set of REST services for 
enhancing [KonText](https://github.com/czcorpus/kontext) installations. 
But it can be also run as a standalone service for:
- generating n-grams,
- generating and querying corpora structural metadata,
- managing Manatee-open compatible data
- querying a corpus (experimental)

Functions:

1. creating and searching in live-attributes (used by KonText)
2. generating n-grams from a vertical file
   * generating KonText query suggestion data sets
2. corpus data information 
   * direct access to Manatee corpus configuration
   * indices location and modification datetime
   * basic registry configuration
   * KonText corpora database access
3. corpus data synchronization between two locations
4. corpus querying (experimental)
   * getting a frequency distribution of a searched expression
   
For more information, see the [API.md](./API.md).
   
## API

see [API.md](./API.md)

## How to build the project

1. Get the sources (`git clone --depth 1 https://github.com/czcorpus/cnc-masm.git`)
2. `go mod tidy`
3. `./configure`
4. `make`
