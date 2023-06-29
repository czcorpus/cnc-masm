# CNC-MASM

*M*anatee *A*ssets, *S*ervices and *M*etadata is a set of REST services for 
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

To build MASM, your system must contain:
  * Python3 (to run the installer script)
  * [Manatee-open](https://nlp.fi.muni.cz/trac/noske) (at least the core shared libraries)
  * [Go language](https://go.dev/) (to compile MASM)

To start the building process, just run:
```
./build3 [manatee version]
```
The concrete supported versions of Manatee-open are: `2.167.8`,  `2.167.10`,  `2.208`, `2.223.6`. In most cases, the latest version is the best option to use.
Once build, a standalone binary `masm3` should be created in the working directory.
