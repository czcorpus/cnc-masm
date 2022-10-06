# cnc-masm

*M*anatee *A*ssets, *S*ervices and *M*etadata is a set of REST services for 
managing miscellaneous corpora data, mainly related to running a [KonText](https://github.com/czcorpus/kontext)
instance. But it can be also run as a standalone service for generating
n-grams and searching corpora structural metadata.

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
   
For more information, see the [API.md](./API.md).
   
## API

see [API.md](./API.md)
